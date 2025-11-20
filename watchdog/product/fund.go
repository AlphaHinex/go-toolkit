package product

import (
	"encoding/json"
	"fmt"
	"go-toolkit/watchdog/product/analysis"
	"go-toolkit/watchdog/utils"
	"log"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Fund struct {
	Code        string `yaml:"-"`    // 基金代码
	Name        string `yaml:"name"` // 基金名称
	CreatedDate string `yaml:"-"`    // 基金成立日期
	Manager     struct {
		Id   string `yaml:"-"` // 基金经理 ID
		Name string `yaml:"-"` // 基金经理名称
	} `yaml:"-"` // 基金经理信息
	Cost   float64 `yaml:"cost"` // 基金成本价
	Status struct {
		Valid bool   `yaml:"-"` // 是否可买卖
		Buy   string `yaml:"-"` // 买入状态
		Sell  string `yaml:"-"` // 卖出状态
	} `yaml:"-"` // 基金买卖状态
	NetValue NetValue `yaml:"net"`      // 基金净值
	Estimate Estimate `yaml:"estimate"` // 实时估算净值
	Profit   struct {
		Estimate string `yaml:"-"` // 实时估算净值收益率
		Net      string `yaml:"-"` // 基金净值收益率
	} `yaml:"-"` // 基金净值收益率
	Ended  bool `yaml:"ended"` // 当日监测是否已结束
	Streak struct {
		Info       string    `yaml:"info"`        // 连续上涨或下跌信息
		UpdateDate time.Time `yaml:"update-date"` // streak 信息的最后更新日期
	} `yaml:"streak"` // 连续上涨或下跌信息
	Scale         float64                 `yaml:"-"` // 基金规模（亿元）
	HistoryValues []analysis.HistoryValue `yaml:"-"` // 历史净值数据
}

// Estimate 实时估值结构体
type Estimate struct {
	Value    string `json:"gsz" yaml:"-"`           // 实时估算净值
	Margin   string `json:"gszzl" yaml:"-"`         // 实时估算涨跌幅
	Datetime string `json:"gztime" yaml:"datetime"` // 实时估算时间
	Changed  bool   `json:"-" yaml:"changed"`       // 实时估算是否变动
}

type NetValue struct {
	Value       float64 `yaml:"-"`       // 单位净值
	Margin      float64 `yaml:"-"`       // 单位净值涨跌幅百分比
	Date        string  `yaml:"date"`    // 单位净值日期
	Updated     bool    `yaml:"updated"` // 是否已更新单位净值
	Accumulated float64 `yaml:"-"`       // 累计净值
}

func (f *Fund) IsTradingDay() bool {
	now := utils.GetNow()
	estimateTime, _ := f.getEstimateTime()
	return utils.IsSameDay(now, estimateTime)
}

func (f *Fund) IsTradable() bool {
	return f.IsTradingDay() && utils.InOpeningHours()
}

func (f *Fund) QueryHistoryValues() []analysis.HistoryValue {
	res, _ := utils.GetFundHttpsResponse("https://fundcomapi.tiantianfunds.com/mm/newCore/FundVPageDiagram",
		url.Values{"FCODE": {f.Code}, "RANGE": {"ln"}})
	if res["data"] == nil || len(res["data"].([]interface{})) == 0 {
		log.Printf("未获取到基金 %s 的历史净值数据，可能是基金代码错误或该基金已被清盘", f.Code)
		return f.HistoryValues
	}
	for _, data := range res["data"].([]interface{}) {
		d := data.(map[string]interface{})
		if d["DWJZ"] == nil {
			log.Printf("基金 %s 历史净值数据缺失 DWJZ 字段，数据内容: %v", f.Code, d)
			continue
		}
		value, err := strconv.ParseFloat(d["DWJZ"].(string), 64)
		if err != nil {
			log.Printf("解析基金 %s 历史净值数据失败: %v", f.Code, err)
			continue
		}
		date, _ := time.ParseInLocation("2006-01-02", d["FSRQ"].(string), utils.GetNow().Location())
		f.HistoryValues = append(f.HistoryValues, analysis.HistoryValue{
			Date:  date,
			Value: value,
		})
	}
	return f.HistoryValues
}

// FundFactory implements Factory for funds.
type FundFactory struct{}

func (f FundFactory) GetAllCodes() []string {
	bodyStr := string(utils.HttpsGet("https://m.1234567.com.cn/data/FundSuggestList.js"))
	re := regexp.MustCompile(`(?s).*FundSuggestList\((.*?)\)\s*$`)
	matches := re.FindStringSubmatch(bodyStr)
	if len(matches) < 2 {
		return nil
	}

	var jsonObj map[string]interface{}
	_ = json.Unmarshal([]byte(matches[1]), &jsonObj)
	var codes []string
	for _, item := range jsonObj["Datas"].([]interface{}) {
		codes = append(codes, strings.Split(item.(string), "|")[0])
	}
	return codes
}

// Build 获得基金名称以及净值信息
func (f FundFactory) Build(code string) FinancialProduct {
	res, _ := utils.GetFundHttpsResponse("https://fundmobapi.eastmoney.com/FundMApi/FundBaseTypeInformation.ashx", url.Values{"FCODE": {code}})
	if res["Datas"] == nil {
		log.Printf("未获取到基金 %s 的净值数据，可能是基金代码错误或该基金已被清盘", code)
		return &Fund{
			Code: code,
			Name: "未知基金",
		}
	} else {
		res = res["Datas"].(map[string]interface{})
	}
	var netValue NetValue
	netValue.Value, _ = strconv.ParseFloat(res["DWJZ"].(string), 64)
	netValue.Date = res["FSRQ"].(string)
	netValue.Margin, _ = strconv.ParseFloat(res["RZDF"].(string), 64)
	netValue.Accumulated, _ = strconv.ParseFloat(res["LJJZ"].(string), 64)
	scale, _ := strconv.ParseFloat(res["ENDNAV"].(string), 64)
	scale = scale / 100000000 // 转换为亿元
	scale = math.Round(scale*100) / 100

	createdDate := "未知成立日期"
	establishRes, _ := utils.GetFundHttpsResponse("https://fundmobapi.eastmoney.com/FundMNewApi/FundMNDetailInformation", url.Values{"FCODE": {code}})
	if establishRes["Datas"] != nil {
		establishRes = establishRes["Datas"].(map[string]interface{})
		createdDate = establishRes["ESTABDATE"].(string)
	}
	return &Fund{
		Code:        code,
		Name:        res["SHORTNAME"].(string),
		CreatedDate: createdDate,
		Manager: struct {
			Id   string `yaml:"-"`
			Name string `yaml:"-"`
		}{
			Id:   res["JJJLID"].(string),
			Name: res["JJJL"].(string),
		},
		Status: struct {
			Valid bool   `yaml:"-"`
			Buy   string `yaml:"-"`
			Sell  string `yaml:"-"`
		}{
			Valid: res["BUY"].(bool),
			Buy:   res["SGZT"].(string),
			Sell:  res["SHZT"].(string),
		},
		NetValue: netValue,
		Scale:    scale,
	}
}

func (f FundFactory) SiftIn(item interface{}, verbose bool) string {
	fund := item.(*Fund)
	if !fund.Status.Valid && verbose {
		log.Printf("跳过无法购买的基金: %s\n", fund.Code)
		return ""
	}
	historyRow := fund.ComposeHistoryRow(fund.NetValue.Value)
	matched, _ := regexp.MatchString(`(?s).*[^度]：[^\n]+◀️\n`, historyRow)
	// 筛选要显示的基金
	if !strings.Contains(fund.Name, "债") &&
		strings.Contains(historyRow, "连续") &&
		fund.Scale > 10 &&
		matched {
		if verbose {
			log.Printf("Matched fund: %s\n%s\n", fund.Name, historyRow)
		}
		return fmt.Sprintf("%s|%s\n最新净值：%.4f\n%s\n", fund.Code, fund.Name, fund.NetValue.Value, historyRow)
	}
	return ""
}

// QueryStreakInfo
// 查询最近一个月的连续上涨或下跌信息
// 连续 3️⃣ 天 🔺2.05% 1.4818 ↗️ 1.5752
// 连续 1️⃣2️⃣ 天 ▼ 2.05% 1.5752 ↘️ 1.4818
func (f *Fund) QueryStreakInfo() {
	now := utils.GetNow()
	if f.Streak.Info != "" && utils.IsSameDay(f.Streak.UpdateDate, now) {
		return // 已经查询过了
	}

	res, _ := utils.GetFundHttpsResponse("https://fundcomapi.tiantianfunds.com/mm/newCore/FundVPageDiagram",
		url.Values{"FCODE": {f.Code}, "RANGE": {"y"}})
	if res["data"] == nil || len(res["data"].([]interface{})) == 0 {
		log.Printf("未获取到基金 %s 的历史净值数据，可能是基金代码错误或该基金已被清盘", f.Code)
		return
	}

	riseStreak, fallStreak := 0, 0
	netValueFrom, netValueTo, netValueMargin := 0.0, 0.0, 0.0
	for i := len(res["data"].([]interface{})) - 1; i >= 0; i-- {
		data := res["data"].([]interface{})[i]
		margin, _ := strconv.ParseFloat(data.(map[string]interface{})["JZZZL"].(string), 64)
		value, _ := strconv.ParseFloat(data.(map[string]interface{})["DWJZ"].(string), 64)
		if riseStreak == 0 && fallStreak == 0 {
			netValueMargin = margin
			netValueFrom, netValueTo = value, value
			// 最近一天如果涨跌幅为 0，直接跳过，看前一日涨跌状态
			if margin > 0 {
				riseStreak++
			} else if margin < 0 {
				fallStreak++
			}
		} else {
			if margin > 0 {
				if riseStreak > 0 {
					riseStreak++
					netValueMargin += margin
				} else {
					netValueFrom = value
					break
				}
			} else if margin < 0 {
				if fallStreak > 0 {
					fallStreak++
					netValueMargin += margin
				} else {
					netValueFrom = value
					break
				}
			} else if margin == 0 {
				// 中间如果有一天涨跌幅为 0，继续计算连续上涨或下跌
				if riseStreak > 0 {
					riseStreak++
				} else if fallStreak > 0 {
					fallStreak++
				}
			}
		}
	}
	if riseStreak > 0 {
		f.Streak.Info = fmt.Sprintf("连续 %s 天 🔺%.2f%% %.4f ↗️ %.4f", utils.TurnToEmojiNumber(riseStreak), netValueMargin, netValueFrom, netValueTo)
	} else if fallStreak > 0 {
		f.Streak.Info = fmt.Sprintf("连续 %s 天 ▼ %.2f%% %.4f ↘️ %.4f", utils.TurnToEmojiNumber(fallStreak), netValueMargin, netValueFrom, netValueTo)
	}
	f.Streak.UpdateDate = now
}

// ComposeHistoryRow
/**
 * 生成历史净值区间行
 * 包括历史净值连续涨跌信息
 * 以及不同阶段的历史净值区间
 * 并根据传入的 markValue 值，在历史区间中标记出所在位置
 */
func (f *Fund) ComposeHistoryRow(markValue float64) string {
	ranges := GetHistoryValueRanges(f)
	estimateValue, _ := strconv.ParseFloat(f.Estimate.Value, 64)
	if markValue == estimateValue {
		// 标记估值位置时，如果当日估值下跌但未低于月度最低值、或当日估值上涨但未高于月度最高值，不显示历史数据
		if strings.HasPrefix(f.Estimate.Margin, "-") && estimateValue > ranges[0].Min.Value {
			return ""
		}
		if !strings.HasPrefix(f.Estimate.Margin, "-") && estimateValue < ranges[0].Max.Value {
			return ""
		}
	}

	f.QueryStreakInfo()
	historyRow := fmt.Sprintf("%s\n历史净值：\n", f.Streak.Info)

	isRise := estimateValue > f.NetValue.Value
	if estimateValue == 0 {
		isRise = strings.Contains(f.Streak.Info, "🔺")
	}
	historyRow += analysis.MarkValueInHistory(markValue, ranges, isRise)
	return historyRow
}

func (f *Fund) GetNetValueDate() (time.Time, error) {
	now := utils.GetNow()
	// 获取净值日期
	netValueDate, err := time.ParseInLocation("2006-01-02", f.NetValue.Date, now.Location())
	return netValueDate, err
}

// 返回当前东八区时间，基金最近的估值时间，以及净值日期
func (f *Fund) getEstimateTime() (time.Time, error) {
	now := utils.GetNow()
	// 获取估值时间
	estimateTime, err := time.ParseInLocation("2006-01-02 15:04", f.Estimate.Datetime, now.Location())
	return estimateTime, err
}

// PrettyPrint
// 美化输出，示例如下：
// 008099|广发价值领先混合A
// 成本：1.5258
// 净值：1.4969 🔺0.05% -1.89% 前日
// 估值：1.4914 ▼ -0.32% -2.25% 15:00
// 连续 3️⃣ 天 🔺2.05% 1.4818 ↗️ 1.5752
// 历史净值：
// 月度：[1.4818, 1.5752]
// 季度：[1.4325, 1.5752]
// 半年：...
// 一年：...
// 三年：...
// 五年：...
// 成立：...
// 开盘时，需要显示历史记录的基金显示估值和净值，否则只显示估值
// 交易日中午休盘时间，同时显示估值和净值
// 交易日收盘后，待所有基金净值更新后，显示估值及净值
func (f *Fund) PrettyPrint(showAll bool) string {
	// 标题行
	title := fmt.Sprintf("%s|%s\n", f.Code, f.Name)

	// 成本行
	costRow := ""
	if f.Cost > 0 {
		costRow = fmt.Sprintf("成本：%.4f\n", f.Cost)
	}

	// 净值行
	now := utils.GetNow()
	netValueDate, _ := time.ParseInLocation("2006-01-02", f.NetValue.Date, now.Location())
	netValueDateStr := "前日"
	if utils.IsSameDay(now, netValueDate) {
		netValueDateStr = "今日"
	}
	netRow := ""
	netProfit := ""
	if f.Cost > 0 {
		netProfit = fmt.Sprintf("%s%% ", f.Profit.Net)
	}
	// 净值 净值涨跌幅 累计收益 净值日期/时间
	netRow = fmt.Sprintf("净值：%.4f %s %s%s\n",
		f.NetValue.Value,
		utils.UpOrDown(fmt.Sprint(f.NetValue.Margin)),
		netProfit,
		netValueDateStr)

	// 估值行
	estimateRow := ""
	if len(f.Estimate.Value) > 0 {
		estimateProfit := ""
		if f.Cost > 0 {
			mark := ""
			if !strings.HasPrefix(f.Profit.Estimate, "-") && !strings.HasPrefix(f.Profit.Net, "-") {
				// 💹 => 📈 📉
				mark = "📈"
			}
			// 估值收益为正标记 估值%
			estimateProfit = fmt.Sprintf("%s%s%% ", mark, f.Profit.Estimate)
		}
		// 估值 估值涨跌幅 估值收益率 估值时间
		estimateRow = fmt.Sprintf("估值：%s %s %s%s\n",
			f.Estimate.Value,
			utils.UpOrDown(f.Estimate.Margin),
			estimateProfit,
			strings.Split(f.Estimate.Datetime, " ")[1])
	}

	result := title + costRow
	if f.IsTradingDay() && utils.InBreakingTime() {
		// 如果是交易日的午休时间，先显示上一日估值，再显示当日净值
		result += netRow + estimateRow
	} else if showAll || (!f.NetValue.Updated && f.NeedToShowHistory()) {
		estimateValue, _ := strconv.ParseFloat(f.Estimate.Value, 64)
		historyRow := ""
		if estimateValue > 0 {
			historyRow = f.ComposeHistoryRow(estimateValue)
		} else {
			historyRow = f.ComposeHistoryRow(f.NetValue.Value)
		}
		// 交易日当日净值未更新且需要显示历史净值时，先显示上一日估值，再显示当日净值
		result += netRow + estimateRow + historyRow
	} else {
		if f.IsTradable() {
			// 开盘中显示实时估值
			result += estimateRow
		} else if f.NeedToShowNetValue() {
			if f.NetValue.Updated {
				// 交易日净值更新后，先显示最后的估值，再显示当日最终净值
				result += estimateRow + netRow
			} else {
				result += netRow + estimateRow
			}
		} else if showAll || ShouldShowAll(f) {
			result += estimateRow + netRow
		}
	}
	return result + "\n"
}

// NeedToShowHistory 是否需要显示历史数据
func (f *Fund) NeedToShowHistory() bool {
	if f.IsTradingDay() && (utils.InOpeningHours() || utils.InBreakingTime()) {
		estimateMargin, _ := strconv.ParseFloat(f.Estimate.Margin, 64)
		estimateProfit, _ := strconv.ParseFloat(f.Profit.Estimate, 64)
		if estimateMargin > 0 && estimateProfit > 0 && f.NetValue.Margin+estimateMargin > 0 {
			// 前日净值+当日估值涨幅为正时，可考虑卖出
			log.Printf("%s 开盘中，且估值涨幅大于0(%f)；估值收益率大于0(%f)\n", f.Name, estimateMargin, estimateProfit)
			return true
		}
		if estimateMargin < 0 && estimateProfit < 0 && f.NetValue.Margin+estimateMargin < -1 && f.NetValue.Margin < 0 {
			// 前日净值及当日估值均下跌，且总跌幅大于1%，可考虑买入
			log.Printf("%s 开盘中，且估值跌幅超1(%f)；估值收益率小于0(%f)\n", f.Name, estimateMargin, estimateProfit)
			return true
		}
	}
	return false
}

// NeedToShowNetValue 是否需要显示净值
func (f *Fund) NeedToShowNetValue() bool {
	now := utils.GetNow()
	netValueDate, _ := f.GetNetValueDate()
	if f.IsTradingDay() && utils.InBreakingTime() && f.Estimate.Changed {
		log.Printf("%s 已更新上午最新估值\n", f.Name)
		return true
	} else if f.IsTradingDay() && !f.IsTradable() && f.Estimate.Changed {
		log.Printf("%s 已更新下午最新估值\n", f.Name)
		return true
	} else if f.IsTradingDay() && utils.IsSameDay(now, netValueDate) &&
		!f.Ended && f.NetValue.Updated {
		log.Printf("%s 今日净值已更新\n", f.Name)
		return true
	} else {
		return false
	}
}
