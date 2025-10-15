package product

import (
	"go-toolkit/watchdog/product/analysis"
	"go-toolkit/watchdog/utils"
	"log"
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
	// QueryHistoryMinMaxValues 获取历史净值/价格最小值和最大值
	QueryHistoryMinMaxValues(rangeStr string) (float64, float64)
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
	for _, s := range []string{"m|月度", "3m|季度", "6m|半年", "y|一年", "3y|三年", "5y|五年", "all|成立"} {
		min, max := product.QueryHistoryMinMaxValues(strings.Split(s, "|")[0])
		ranges = append(ranges, analysis.HistoryValueRange{
			Title: strings.Split(s, "|")[1],
			Min:   min,
			Max:   max,
		})
	}
	return ranges
}
