package product

import (
	"go-toolkit/watchdog/utils"
	"strings"
)

// Factory defines an interface for retrieving and building financial products.
type Factory interface {
	GetAllCodes() []string              // fetching financial products' code.
	Build(code string) FinancialProduct // build a financial product by code.
}

type FinancialProduct interface {
	// IsTradingDay 当天是否是交易日
	IsTradingDay() bool
	// IsTradable 当前是否可交易
	IsTradable() bool
	// QueryHistoryMinMaxValues 获取历史净值/价格最小值和最大值
	QueryHistoryMinMaxValues(rangeStr string) (float64, float64)
}

type HistoryValueRange struct {
	title string  // 历史区间范围
	min   float64 // 历史区间最小净值
	max   float64 // 历史区间最大净值
}

// ShouldShowAll
// 根据当前时间判断是否需要显示全部金融产品信息
// 满足一下任一条件时，显示全部信息：
// 1. 如果是交易日的开盘时间，且当前分钟为 48 分钟
// 2. 交易日收盘后的 21:48
func ShouldShowAll(product FinancialProduct) bool {
	now, _ := utils.GetNow()
	hour := now.Hour()
	minute := now.Minute()
	return product.IsTradingDay() &&
		((product.IsTradable() && minute == 48) || (hour == 21 && minute == 48))
}

func GetHistoryValueRanges(product FinancialProduct) []HistoryValueRange {
	var ranges []HistoryValueRange
	for _, s := range []string{"m|月度", "3m|季度", "6m|半年", "y|一年", "3y|三年", "5y|五年", "all|成立"} {
		min, max := product.QueryHistoryMinMaxValues(strings.Split(s, "|")[0])
		ranges = append(ranges, HistoryValueRange{
			title: strings.Split(s, "|")[1],
			min:   min,
			max:   max,
		})
	}
	return ranges
}
