package product

import (
	"fmt"
	"go-toolkit/watchdog/product/analysis"
	"go-toolkit/watchdog/utils"
	"log"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Factory defines an interface for retrieving and building financial products.
type Factory interface {
	GetAllCodes() []string                        // fetching financial products' code.
	Build(code string) FinancialProduct           // build a financial product by code.
	SiftIn(item interface{}, verbose bool) string // sift in a financial product item and return its representation.
	LLMPrompt() string                            // prompt used for LLM analysis.
}

func Sift(factory Factory, verbose bool) string {
	codes := factory.GetAllCodes() // 获取所有代码

	var resultBuilder strings.Builder            // String builder to accumulate results
	var mu sync.Mutex                            // 用于保护文件写入的互斥锁
	wg := &sync.WaitGroup{}                      // 创建 WaitGroup
	concurrencyLimit := 8                        // 设置并发限制数量
	sem := make(chan struct{}, concurrencyLimit) // 创建带缓冲的通道

	for _, code := range codes {
		wg.Add(1) // 增加一个任务
		go func(code string) {
			defer wg.Done() // 任务完成时减少计数

			sem <- struct{}{}        // 占用一个并发槽
			defer func() { <-sem }() // 释放并发槽

			if verbose {
				log.Printf("Processing code: %s\n", code)
			}
			item := factory.Build(code)
			if result := factory.SiftIn(item, verbose); result != "" {
				// Write to the string builder with mutex protection
				mu.Lock()
				resultBuilder.WriteString(result)
				mu.Unlock()
			}
			if verbose {
				log.Printf("Finished processing code: %s\n", code)
			}
		}(code)
	}

	wg.Wait() // 等待所有任务完成
	if resultBuilder.Len() == 0 {
		return "No data available."
	} else {
		return resultBuilder.String()
	}
}

type FinancialProduct interface {
	// IsTradingDay 当天是否是交易日
	IsTradingDay() bool
	// IsTradable 当前是否可交易
	IsTradable() bool
	// QueryHistoryValues 获取历史净值/价格数据，按日期正序排列（由远及近），同时分析近期 Streak 走势
	QueryHistoryValues() []analysis.HistoryValue
}

// ShouldShowAll
// 根据当前时间判断是否需要显示全部金融产品信息
// 满足一下任一条件时，显示全部信息：
// 1. 如果是交易日的开盘时间，且当前分钟为 48 分钟
// 2. 交易日收盘后的 21:48
func ShouldShowAll(product FinancialProduct) bool {
	now := utils.GetNow()
	hour := now.Hour()
	minute := now.Minute()
	return product.IsTradingDay() &&
		((product.IsTradable() && minute == 48) || (hour == 21 && minute == 48))
}

func GetHistoryValueRanges(product FinancialProduct) []analysis.HistoryValueRange {
	var ranges []analysis.HistoryValueRange
	values := product.QueryHistoryValues()
	// 将 values 倒序，按日期从近到远排列
	for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
		values[i], values[j] = values[j], values[i]
	}

	rangeMap := map[string]time.Time{
		"月度": utils.LastMonth(),
		"季度": utils.LastQuarter(),
		"半年": utils.LastSixMonths(),
		"一年": utils.LastYear(),
		"三年": utils.LastThreeYears(),
		"五年": utils.LastFiveYears(),
		"成立": utils.EarliestTime(),
	}

	for _, s := range []string{"月度", "季度", "半年", "一年", "三年", "五年", "成立"} {
		startDate := rangeMap[s]
		min, max := queryHistoryMinMaxValues(values, startDate)
		if min.Value == math.MaxInt16 && max.Value == math.MinInt16 {
			continue
		}
		ranges = append(ranges, analysis.HistoryValueRange{
			Title: s,
			Min:   min,
			Max:   max,
		})
	}
	return ranges
}

// queryHistoryMinMaxValues 获取 startDate 至今的历史净值/价格最小值和最大值
func queryHistoryMinMaxValues(values []analysis.HistoryValue, startDate time.Time) (analysis.HistoryValue, analysis.HistoryValue) {
	var min = analysis.HistoryValue{
		Value: math.MaxInt16,
		Date:  utils.GetNow(),
	}
	var max = analysis.HistoryValue{
		Value: math.MinInt16,
		Date:  utils.GetNow(),
	}
	if len(values) == 0 || !startDate.Equal(utils.EarliestTime()) && startDate.Before(values[len(values)-1].Date) {
		return min, max
	}
	for _, value := range values {
		if value.Date.Before(startDate) {
			break
		}
		if value.Value < min.Value {
			min = value
		}
		if value.Value > max.Value {
			max = value
		}
	}
	return min, max
}

// TrendDuring Trend 最大日期范围天数
var TrendDuring = 12

// QueryStreakInfo 查询最近的连续上涨或下跌信息，将查询得到信息填入 Streak 对应属性中，例如：
// Streak.Trend:
// 📈📈📉📉📉📉－📉📈📈📉📉📉
// Streak.Info:
// 连续 3️⃣ 天 🔺2.05% 1.4818 ↗️ 1.5752
// 或
// 连续 1️⃣2️⃣ 天 ▼ 2.05% 1.5752 ↘️ 1.4818
// 等
// Streak.UpdateDate: 更新为当前时间
// 入参：
// valuesFromOldToNew 历史净值/价格数据，按日期正序排列（由远及近）
func QueryStreakInfo(valuesFromOldToNew []analysis.HistoryValue, streak *analysis.Streak) {
	now := utils.GetNow()
	if streak.Info != "" && utils.IsSameDay(streak.UpdateDate, now) {
		return // 当天已经查询过了
	}

	valuesLen := len(valuesFromOldToNew)
	if valuesLen < 2 {
		return // 数据不足，无法计算
	}
	// 从多少钱涨跌到多少钱
	from, to := 0.0, 0.0
	streak.Trend = ""
	for i := valuesLen - 2; i >= max(valuesLen-TrendDuring-1, 0); i-- {
		from = valuesFromOldToNew[i].Value
		to = valuesFromOldToNew[i+1].Value
		if from == to {
			streak.Trend = "－" + streak.Trend
		} else if from > to {
			streak.Trend = "📉" + streak.Trend
		} else {
			streak.Trend = "📈" + streak.Trend
		}
	}

	raisePattern := regexp.MustCompile(`[📈－]+$`)
	fallPattern := regexp.MustCompile(`[📉－]+$`)

	matches := raisePattern.FindString(streak.Trend)
	if matches == "" {
		matches = fallPattern.FindString(streak.Trend)
	}
	lastIdx := len([]rune(matches))
	if lastIdx == 0 {
		return // 最近没有连续涨跌
	}

	from = valuesFromOldToNew[valuesLen-lastIdx-1].Value
	to = valuesFromOldToNew[valuesLen-1].Value
	// 涨跌幅百分比是多少
	totalMargin := (to - from) / from * 100
	marks := []string{"🔺", "↗️"}
	if totalMargin < 0 {
		marks = []string{"▼ ", "↘️"}
	}

	streak.Info = fmt.Sprintf("连续 %s 天 %s%.2f%% %.4f %s %.4f",
		utils.TurnToEmojiNumber(lastIdx), marks[0], totalMargin, from, marks[1], to)
	streak.UpdateDate = now
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
