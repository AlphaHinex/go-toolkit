package product

import "go-toolkit/watchdog/utils"

type FinancialProduct interface {
	IsTradingDay() bool // 当天是否是交易日
	IsTradable() bool   // 当前是否可交易
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

// CodeProvider defines an interface for fetching financial products' code.
type CodeProvider interface {
	GetAllCodes() []string
}
