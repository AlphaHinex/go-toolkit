package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-toolkit/watchdog/product"
	"go-toolkit/watchdog/utils"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
)

var stockSiftTypes = []string{"破高", "连涨", "止跌反弹"}

type RankedStock struct {
	Code   string  `json:"code"`
	Name   string  `json:"name"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

type RankedStockGroup struct {
	Type   string        `json:"type"`
	Stocks []RankedStock `json:"stocks"`
}

type StockSiftRankResult struct {
	Groups []RankedStockGroup `json:"groups"`
}

type rankChatRequest struct {
	Model       string              `json:"model"`
	Temperature float64             `json:"temperature"`
	Messages    []rankChatMessage   `json:"messages"`
	ResponseFmt *rankResponseFormat `json:"response_format,omitempty"`
}

type rankChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type rankResponseFormat struct {
	Type string `json:"type"`
}

type rankChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func RankStocksByAI(config *Config, apiKey string, candidates []product.StockSiftCandidate) (*StockSiftRankResult, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY 不能为空")
	}
	if len(candidates) == 0 {
		return defaultRankStocks(candidates), nil
	}
	payload, err := json.Marshal(candidates)
	if err != nil {
		return nil, fmt.Errorf("序列化初筛股票失败: %w", err)
	}
	body := rankChatRequest{
		Model:       config.SiftAI.Model,
		Temperature: 0.2,
		Messages: []rankChatMessage{
			{Role: "system", Content: buildStockRankingSystemPrompt()},
			{Role: "user", Content: buildStockRankingUserPrompt(string(payload))},
		},
		ResponseFmt: &rankResponseFormat{Type: "json_object"},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化 OpenAI 请求体失败: %w", err)
	}
	url := strings.TrimRight(config.SiftAI.Endpoint, "/") + "/v1/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建 OpenAI 请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := utils.DoRequestWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("调用 OpenAI 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI 返回异常: %s %s", resp.Status, string(bodyBytes))
	}
	var chatResp rankChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("解析 OpenAI 响应失败: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI 响应无可用候选")
	}
	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	var result StockSiftRankResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("解析模型复筛 JSON 失败: %w; content=%s", err, content)
	}
	if err := normalizeRankResult(&result, candidates); err != nil {
		return nil, err
	}
	return &result, nil
}

func RankStocks(config *Config, apiKey string, candidates []product.StockSiftCandidate) (*StockSiftRankResult, bool, error) {
	result, err := RankStocksByAI(config, apiKey, candidates)
	if err == nil {
		return result, false, nil
	}
	return defaultRankStocks(candidates), true, err
}

func normalizeRankResult(result *StockSiftRankResult, candidates []product.StockSiftCandidate) error {
	candidateMap := make(map[string]product.StockSiftCandidate, len(candidates))
	for _, c := range candidates {
		candidateMap[c.Code] = c
	}
	typeSet := map[string]struct{}{"破高": {}, "连涨": {}, "止跌反弹": {}}
	groupMap := map[string]*RankedStockGroup{}
	for i := range result.Groups {
		g := &result.Groups[i]
		if _, ok := typeSet[g.Type]; !ok {
			continue
		}
		filtered := make([]RankedStock, 0, 3)
		for _, s := range g.Stocks {
			candidate, ok := candidateMap[s.Code]
			if !ok {
				continue
			}
			if !containsType(candidate.Types, g.Type) {
				continue
			}
			if strings.TrimSpace(s.Name) == "" {
				s.Name = candidate.Name
			}
			if math.IsNaN(s.Score) || math.IsInf(s.Score, 0) {
				s.Score = 0
			}
			filtered = append(filtered, s)
			if len(filtered) == 3 {
				break
			}
		}
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Score > filtered[j].Score
		})
		g.Stocks = filtered
		groupMap[g.Type] = g
	}
	groups := make([]RankedStockGroup, 0, len(stockSiftTypes))
	for _, t := range stockSiftTypes {
		if g, ok := groupMap[t]; ok {
			groups = append(groups, *g)
			continue
		}
		groups = append(groups, pickTopByType(candidates, t))
	}
	result.Groups = groups
	return nil
}

func defaultRankStocks(candidates []product.StockSiftCandidate) *StockSiftRankResult {
	groups := make([]RankedStockGroup, 0, len(stockSiftTypes))
	for _, t := range stockSiftTypes {
		groups = append(groups, pickTopByType(candidates, t))
	}
	return &StockSiftRankResult{Groups: groups}
}

func pickTopByType(candidates []product.StockSiftCandidate, targetType string) RankedStockGroup {
	items := make([]RankedStock, 0)
	for _, c := range candidates {
		if !containsType(c.Types, targetType) {
			continue
		}
		items = append(items, RankedStock{
			Code:   c.Code,
			Name:   c.Name,
			Score:  heuristicScore(c, targetType),
			Reason: "本地规则兜底：按信号强度、趋势与市值综合排序",
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})
	if len(items) > 3 {
		items = items[:3]
	}
	return RankedStockGroup{Type: targetType, Stocks: items}
}

func heuristicScore(c product.StockSiftCandidate, targetType string) float64 {
	score := 50.0
	if c.BreakHistoryHigh {
		score += 12
	}
	if c.ContinuousRise {
		score += 10
	}
	if c.FallThenRise {
		score += 10
	}
	score += float64(strings.Count(c.Trend, "📈")) * 1.2
	score -= float64(strings.Count(c.Trend, "📉")) * 0.8
	if c.MarketValue > 0 {
		score += math.Min(8, math.Log10(c.MarketValue+1)*3)
	}
	switch targetType {
	case "破高":
		if c.BreakHistoryHigh {
			score += 8
		}
	case "连涨":
		if c.ContinuousRise {
			score += 8
		}
	case "止跌反弹":
		if c.FallThenRise {
			score += 8
		}
	}
	if score > 100 {
		return 100
	}
	if score < 0 {
		return 0
	}
	return math.Round(score*100) / 100
}

func containsType(types []string, target string) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}

func buildStockRankingSystemPrompt() string {
	return "你是资深股票研究员。请根据给定初筛数据，从三个类型中分别选择最具投资潜力的3只股票，按投资价值从高到低排序。只输出严格 JSON，不要任何额外文本。"
}

func buildStockRankingUserPrompt(payload string) string {
	return fmt.Sprintf(`股票初筛候选如下（JSON 数组，每项包含 code/name/price/marketValue/signals/trend 等）：
%s

请完成复筛，要求：
1) 按类型输出三个分组：破高、连涨、止跌反弹；
2) 每个分组最多3只股票；
3) 组内按 score 从高到低；
4) 每项包含 code,name,score,reason。
5) score 范围 0~100。

输出 JSON 格式必须是：
{
  "groups": [
    {
      "type": "破高",
      "stocks": [
        {"code":"000001.SZ","name":"示例","score":88.5,"reason":"示例原因"}
      ]
    },
    {
      "type": "连涨",
      "stocks": []
    },
    {
      "type": "止跌反弹",
      "stocks": []
    }
  ]
}`, payload)
}
