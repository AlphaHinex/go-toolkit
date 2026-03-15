package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-toolkit/watchdog/product"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

type StockSelection struct {
	Type            string  `json:"type"`
	Code            string  `json:"code"`
	Name            string  `json:"name,omitempty"`
	Price           float64 `json:"price,omitempty"`
	MarketValueYi   float64 `json:"market_value_yi,omitempty"`
	InvestmentValue float64 `json:"investment_value"`
	Reason          string  `json:"reason"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type llmSelectionResponse struct {
	Selections []StockSelection `json:"selections"`
}

func BuildStockSiftNotification(configs *Config, candidates []product.StockSiftCandidate, csvFilePath string) string {
	if len(candidates) == 0 {
		return "股票初筛完成，但无符合条件的标的。"
	}

	selections, err := SelectTopStocksByOpenAI(configs, candidates)
	if err != nil {
		log.Printf("OpenAI 二次筛选失败，使用本地回退策略: %v", err)
		selections = fallbackSelections(candidates)
	}
	if len(selections) == 0 {
		return fmt.Sprintf("股票初筛完成（%d 只），但二次筛选未得到结果。CSV：%s", len(candidates), csvFilePath)
	}

	order := map[string]int{
		"突破历史高点":  1,
		"连续上涨":    2,
		"连续下跌后反弹": 3,
	}
	sort.Slice(selections, func(i, j int) bool {
		li, lj := order[selections[i].Type], order[selections[j].Type]
		if li != lj {
			return li < lj
		}
		return selections[i].InvestmentValue > selections[j].InvestmentValue
	})

	var b strings.Builder
	b.WriteString("股票二次筛选结果（LLM）\n")
	b.WriteString(fmt.Sprintf("初筛总数：%d\n", len(candidates)))
	b.WriteString(fmt.Sprintf("CSV：%s\n", csvFilePath))

	currentType := ""
	for _, item := range selections {
		if item.Type != currentType {
			currentType = item.Type
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("【%s】\n", currentType))
		}
		b.WriteString(fmt.Sprintf("%s %s | 投资价值 %.1f | 现价 %.2f | 市值 %.2f亿\n",
			item.Code, item.Name, item.InvestmentValue, item.Price, item.MarketValueYi))
		if strings.TrimSpace(item.Reason) != "" {
			b.WriteString(fmt.Sprintf("理由：%s\n", strings.TrimSpace(item.Reason)))
		}
	}
	return strings.TrimSpace(b.String())
}

func SelectTopStocksByOpenAI(configs *Config, candidates []product.StockSiftCandidate) ([]StockSelection, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	if strings.TrimSpace(configs.OpenAI.Endpoint) == "" ||
		strings.TrimSpace(configs.OpenAI.ApiKey) == "" ||
		strings.TrimSpace(configs.OpenAI.Model) == "" {
		return nil, fmt.Errorf("缺少 openai.endpoint / openai.api-key / openai.model 配置")
	}

	temp := configs.OpenAI.Temp
	if temp == 0 {
		temp = 0.2
	}

	type candidatePayload struct {
		Code         string   `json:"code"`
		Name         string   `json:"name"`
		Price        float64  `json:"price"`
		MarketValue  float64  `json:"market_value_yi"`
		TypeLabels   []string `json:"type_labels"`
		Trend        string   `json:"trend"`
		StreakInfo   string   `json:"streak_info"`
		HistoryRange string   `json:"history_row"`
	}

	payloadItems := make([]candidatePayload, 0, len(candidates))
	for _, c := range candidates {
		payloadItems = append(payloadItems, candidatePayload{
			Code:         c.Code,
			Name:         c.Name,
			Price:        c.Price,
			MarketValue:  c.MarketValue,
			TypeLabels:   c.TypeLabels(),
			Trend:        c.Trend,
			StreakInfo:   c.StreakInfo,
			HistoryRange: c.HistoryRow,
		})
	}
	candidatesJSON, _ := json.Marshal(payloadItems)

	systemPrompt := "你是A股选股分析助手。请在每个类型中挑选最具投资潜力的三只股票。"
	userPrompt := fmt.Sprintf("以下是股票初筛结果 JSON。请严格遵守：\n1) 只允许从给定股票中选择。\n2) 每个类型最多 3 只。\n3) 输出必须是 JSON 对象，格式为 {\\\"selections\\\":[...]}。\n4) selections 每项字段：type, code, investment_value(0~100数字), reason(不超过50字)。\n5) type 仅允许：突破历史高点、连续上涨、连续下跌后反弹。\n\n初筛数据：\n%s", string(candidatesJSON))

	requestBody := map[string]interface{}{
		"model": configs.OpenAI.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": temp,
	}
	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("序列化 OpenAI 请求失败: %w", err)
	}

	endpoint := strings.TrimRight(configs.OpenAI.Endpoint, "/") + "/v1/chat/completions"
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, fmt.Errorf("创建 OpenAI 请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+configs.OpenAI.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 OpenAI 失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI 返回状态码 %d: %s", resp.StatusCode, string(body))
	}

	var completion chatCompletionResponse
	if err := json.Unmarshal(body, &completion); err != nil {
		return nil, fmt.Errorf("解析 OpenAI 响应失败: %w", err)
	}
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI 响应无 choices")
	}

	rawContent := strings.TrimSpace(completion.Choices[0].Message.Content)
	cleaned := extractJSONObject(rawContent)
	if cleaned == "" {
		return nil, fmt.Errorf("未在 OpenAI 返回内容中找到 JSON")
	}

	var selections []StockSelection
	var wrapped llmSelectionResponse
	if err := json.Unmarshal([]byte(cleaned), &wrapped); err == nil && len(wrapped.Selections) > 0 {
		selections = wrapped.Selections
	} else {
		if err := json.Unmarshal([]byte(cleaned), &selections); err != nil {
			return nil, fmt.Errorf("解析筛选 JSON 失败: %w", err)
		}
	}

	return normalizeSelections(selections, candidates), nil
}

func normalizeSelections(raw []StockSelection, candidates []product.StockSiftCandidate) []StockSelection {
	typeOrder := map[string]int{
		"突破历史高点":  1,
		"连续上涨":    2,
		"连续下跌后反弹": 3,
	}

	candidateByCode := make(map[string]product.StockSiftCandidate, len(candidates))
	candidateTypes := make(map[string]map[string]bool, len(candidates))
	for _, c := range candidates {
		candidateByCode[c.Code] = c
		types := make(map[string]bool)
		for _, t := range c.TypeLabels() {
			types[t] = true
		}
		candidateTypes[c.Code] = types
	}

	grouped := map[string][]StockSelection{
		"突破历史高点":  {},
		"连续上涨":    {},
		"连续下跌后反弹": {},
	}
	seen := make(map[string]bool)
	for _, s := range raw {
		if _, exists := typeOrder[s.Type]; !exists {
			continue
		}
		candidate, exists := candidateByCode[s.Code]
		if !exists {
			continue
		}
		if !candidateTypes[s.Code][s.Type] {
			continue
		}
		key := s.Type + ":" + s.Code
		if seen[key] {
			continue
		}
		seen[key] = true

		s.Name = candidate.Name
		s.Price = candidate.Price
		s.MarketValueYi = candidate.MarketValue
		grouped[s.Type] = append(grouped[s.Type], s)
	}

	for typ := range grouped {
		sort.Slice(grouped[typ], func(i, j int) bool {
			return grouped[typ][i].InvestmentValue > grouped[typ][j].InvestmentValue
		})
		if len(grouped[typ]) > 3 {
			grouped[typ] = grouped[typ][:3]
		}
	}

	result := make([]StockSelection, 0, 9)
	orderedTypes := []string{"突破历史高点", "连续上涨", "连续下跌后反弹"}
	for _, typ := range orderedTypes {
		result = append(result, grouped[typ]...)
	}
	return result
}

func fallbackSelections(candidates []product.StockSiftCandidate) []StockSelection {
	grouped := map[string][]StockSelection{
		"突破历史高点":  {},
		"连续上涨":    {},
		"连续下跌后反弹": {},
	}

	for _, c := range candidates {
		for _, typ := range c.TypeLabels() {
			grouped[typ] = append(grouped[typ], StockSelection{
				Type:            typ,
				Code:            c.Code,
				Name:            c.Name,
				Price:           c.Price,
				MarketValueYi:   c.MarketValue,
				InvestmentValue: c.MarketValue,
				Reason:          "OpenAI 不可用，按市值回退排序",
			})
		}
	}

	for typ := range grouped {
		sort.Slice(grouped[typ], func(i, j int) bool {
			return grouped[typ][i].InvestmentValue > grouped[typ][j].InvestmentValue
		})
		if len(grouped[typ]) > 3 {
			grouped[typ] = grouped[typ][:3]
		}
	}

	result := make([]StockSelection, 0, 9)
	for _, typ := range []string{"突破历史高点", "连续上涨", "连续下跌后反弹"} {
		result = append(result, grouped[typ]...)
	}
	return result
}

func extractJSONObject(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}

	firstBrace := strings.Index(trimmed, "{")
	lastBrace := strings.LastIndex(trimmed, "}")
	if firstBrace >= 0 && lastBrace > firstBrace {
		return trimmed[firstBrace : lastBrace+1]
	}

	firstBracket := strings.Index(trimmed, "[")
	lastBracket := strings.LastIndex(trimmed, "]")
	if firstBracket >= 0 && lastBracket > firstBracket {
		return trimmed[firstBracket : lastBracket+1]
	}
	return ""
}
