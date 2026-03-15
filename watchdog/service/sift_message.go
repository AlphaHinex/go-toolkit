package service

import (
	"fmt"
	"strings"
)

func BuildStockSiftMessage(rankResult *StockSiftRankResult, csvPath string, fallback bool) string {
	if rankResult == nil || len(rankResult.Groups) == 0 {
		return "股票复筛结果为空"
	}
	var builder strings.Builder
	builder.WriteString("股票复筛结果（每类 Top3）\n")
	if fallback {
		builder.WriteString("说明：OpenAI 复筛失败，本次使用本地规则兜底。\n")
	}
	builder.WriteString(fmt.Sprintf("初筛 CSV：%s\n\n", csvPath))
	for _, group := range rankResult.Groups {
		builder.WriteString(fmt.Sprintf("【%s】\n", group.Type))
		if len(group.Stocks) == 0 {
			builder.WriteString("- 无候选\n\n")
			continue
		}
		for idx, stock := range group.Stocks {
			reason := strings.TrimSpace(stock.Reason)
			if reason == "" {
				reason = "无"
			}
			builder.WriteString(fmt.Sprintf("%d. %s|%s 评分 %.2f\n原因：%s\n", idx+1, stock.Code, stock.Name, stock.Score, reason))
		}
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}
