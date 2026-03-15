package service

import (
	"encoding/csv"
	"fmt"
	"go-toolkit/watchdog/product"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func SaveStockSiftCandidatesAsCSV(candidates []product.StockSiftCandidate) (string, error) {
	if err := os.MkdirAll("sift-results", 0755); err != nil {
		return "", fmt.Errorf("创建 sift-results 目录失败: %w", err)
	}

	fileName := fmt.Sprintf("stock_sift_%s.csv", time.Now().Format("20060102_150405"))
	filePath := filepath.Join("sift-results", fileName)
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("创建 CSV 文件失败: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	headers := []string{
		"code",
		"name",
		"price",
		"market_value_yi",
		"type_labels",
		"break_history_high",
		"continuous_rise",
		"continuous_fall_then_rise",
		"trend",
		"streak_info",
		"history_row",
	}
	if err := writer.Write(headers); err != nil {
		return "", fmt.Errorf("写入 CSV 表头失败: %w", err)
	}

	for _, candidate := range candidates {
		record := []string{
			candidate.Code,
			candidate.Name,
			fmt.Sprintf("%.4f", candidate.Price),
			fmt.Sprintf("%.2f", candidate.MarketValue),
			strings.Join(candidate.TypeLabels(), ";"),
			fmt.Sprintf("%t", candidate.BreakHistoryHigh),
			fmt.Sprintf("%t", candidate.ContinuousRise),
			fmt.Sprintf("%t", candidate.ContinuousFallThenRise),
			candidate.Trend,
			candidate.StreakInfo,
			candidate.HistoryRow,
		}
		if err := writer.Write(record); err != nil {
			return "", fmt.Errorf("写入 CSV 行失败: %w", err)
		}
	}

	return filePath, nil
}
