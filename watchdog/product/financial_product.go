package product

import (
	"fmt"
	"go-toolkit/watchdog/product/analysis"
	"go-toolkit/watchdog/utils"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
)

// Factory defines an interface for retrieving and building financial products.
type Factory interface {
	GetAllCodes() []string                        // fetching financial products' code.
	Build(code string) FinancialProduct           // build a financial product by code.
	SiftIn(item interface{}, verbose bool) string // sift in a financial product item and return its representation.
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
	// QueryHistoryValues 获取历史净值/价格数据
	QueryHistoryValues() []analysis.HistoryValue
	// QueryHistoryMinMaxValues 获取历史净值/价格最小值和最大值
	//QueryHistoryMinMaxValues(rangeStr string) (analysis.HistoryValue, analysis.HistoryValue)
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
	for _, s := range strings.Split(fmt.Sprintf("%d|月度,%d|季度,%d|半年,%d|一年,%d|三年,%d|五年,%d|成立",
		30, 30*3, 30*6, 365, 365*3, 365*5, math.MaxInt16), ",") {
		rangeNum, _ := strconv.ParseFloat(strings.Split(s, "|")[0], 64)
		min, max := queryHistoryMinMaxValues(values, rangeNum)
		if min.Value == 0 && max.Value == 0 {
			continue
		}
		ranges = append(ranges, analysis.HistoryValueRange{
			Title: strings.Split(s, "|")[1],
			Min:   min,
			Max:   max,
		})
	}
	return ranges
}

func queryHistoryMinMaxValues(values []analysis.HistoryValue, rangeNum float64) (analysis.HistoryValue, analysis.HistoryValue) {
	var min, max analysis.HistoryValue
	if rangeNum != math.MaxInt16 && rangeNum > float64(len(values)) {
		return min, max
	}
	for i := 0; i < int(math.Min(rangeNum, float64(len(values)))); i++ {
		value := values[i]
		if min.Value == 0 || value.Value < min.Value {
			min = value
		}
		if max.Value == 0 || value.Value > max.Value {
			max = value
		}
	}
	return min, max
}
