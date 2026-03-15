package service

import (
	"encoding/csv"
	"fmt"
	"go-toolkit/watchdog/product"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func SaveStockInitialSiftCSV(outputDir string, candidates []product.StockSiftCandidate) (string, error) {
	if outputDir == "" {
		outputDir = DefaultSiftAIOutputDir
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败 %s: %w", outputDir, err)
	}
	filePath := filepath.Join(outputDir, fmt.Sprintf("stocks_initial_%s.csv", time.Now().Format("20060102_150405")))
	f, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("创建初筛 CSV 失败: %w", err)
	}
	defer f.Close()
	writer := csv.NewWriter(f)
	defer writer.Flush()
	headers := []string{"code", "name", "price", "market_value", "break_history_high", "continuous_rise", "fall_then_rise", "trend", "streak_info", "history_row", "types"}
	if err := writer.Write(headers); err != nil {
		return "", fmt.Errorf("写入初筛 CSV 表头失败: %w", err)
	}
	for _, c := range candidates {
		row := []string{
			c.Code,
			c.Name,
			strconv.FormatFloat(c.Price, 'f', 4, 64),
			strconv.FormatFloat(c.MarketValue, 'f', 2, 64),
			strconv.FormatBool(c.BreakHistoryHigh),
			strconv.FormatBool(c.ContinuousRise),
			strconv.FormatBool(c.FallThenRise),
			c.Trend,
			c.StreakInfo,
			c.HistoryRow,
			strings.Join(c.Types, "|"),
		}
		if err := writer.Write(row); err != nil {
			return "", fmt.Errorf("写入初筛 CSV 行失败: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("刷新初筛 CSV 失败: %w", err)
	}
	return filePath, nil
}
