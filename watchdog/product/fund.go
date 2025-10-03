package product

import (
	"encoding/json"
	"fmt"
	"go-toolkit/watchdog/utils"
	"log"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
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
	Scale float64 `yaml:"-"` // 基金规模（亿元）
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

type HistoryNetValueRange struct {
	title string
	min   NetValue
	max   NetValue
}

func (f *Fund) IsTradingDay() bool {
	now, _ := utils.GetNow()
	estimateTime, _ := f.getEstimateTime()
	return utils.IsSameDay(now, estimateTime)
}

func (f *Fund) IsTradable() bool {
	return f.IsTradingDay() && utils.InOpeningHours()
}

// FundCodeProvider implements CodeProvider for funds.
type FundCodeProvider struct{}

func (f FundCodeProvider) GetAllCodes() []string {
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

// QueryStreakInfo
// 查询最近一个月的连续上涨或下跌信息
// 连续 3️⃣ 天 🔺2.05% 1.4818 ↗️ 1.5752
// 连续 1️⃣2️⃣ 天 ▼ 2.05% 1.5752 ↘️ 1.4818
func (f *Fund) QueryStreakInfo() {
	now, _ := utils.GetNow()
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
	f.QueryStreakInfo()
	historyRow := fmt.Sprintf("%s\n历史净值：\n", f.Streak.Info)

	ranges := f.getHistoryNetValueRanges()
	idx, leftOrRight, exceeded := positionInHistory(markValue, ranges)

	for i, history := range ranges {
		mark := ""
		if idx == i {
			if exceeded {
				if leftOrRight < 0 {
					mark = "⏮️"
				} else {
					mark = "⏭️"
				}
			} else {
				if leftOrRight < 0 {
					mark = "◀️"
				} else {
					mark = "▶️"
				}
			}
		}
		historyRow += fmt.Sprintf("%s：[%.4f, %.4f] %s\n", history.title, history.min.Value, history.max.Value, mark)
	}
	return historyRow
}

func (f *Fund) getHistoryNetValueRanges() []HistoryNetValueRange {
	var ranges []HistoryNetValueRange
	for _, s := range []string{"y|月度", "3y|季度", "6y|半年", "n|一年", "3n|三年", "5n|五年", "ln|成立"} {
		min, max := findFundHistoryMinMaxNetValues(f.Code, strings.Split(s, "|")[0])
		ranges = append(ranges, HistoryNetValueRange{
			title: strings.Split(s, "|")[1],
			min:   min,
			max:   max,
		})
	}
	return ranges
}

func Sift(verbose bool) string {
	codes := FundCodeProvider{}.GetAllCodes() // 获取所有基金代码

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
				log.Printf("Processing fund code: %s\n", code)
			}
			fund := BuildFund(code)

			if !fund.Status.Valid && verbose {
				log.Printf("跳过无法购买的基金: %s\n", code)
				return
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
				// Write to the string builder with mutex protection
				mu.Lock()
				// 写入文件
				resultBuilder.WriteString(fmt.Sprintf("%s|%s\n最新净值：%.4f\n%s\n", code, fund.Name, fund.NetValue.Value, historyRow))
				mu.Unlock()
			}
			if verbose {
				log.Printf("Finished processing fund code: %s\n", code)
			}
		}(code)
	}

	wg.Wait() // 等待所有任务完成
	return resultBuilder.String()
}

/**
 * 查找某值在给定净值历史区间中所处的位置。
 * 返回值：历史区间数组位置索引，在所属区间偏左还是偏右（小于 0 偏左，大于 0 偏右），是否超过边界值
 */
func positionInHistory(value float64, histories []HistoryNetValueRange) (int, int, bool) {
	idx, leftOrRight, exceeded := -1, 0, false
	for i, h := range histories {
		if value >= h.min.Value && value <= h.max.Value {
			idx = i
			break
		}
	}
	// 位于某区间内时，判断偏左还是偏右，并对对应侧的边界值进行向下穿透（下个历史数据区间对应侧边界值与当前区间一致时，idx 向下移动）
	if idx > -1 {
		if value < (histories[idx].min.Value+histories[idx].max.Value)/2 {
			leftOrRight = -1
		} else {
			leftOrRight = 1
		}
		for i := idx; i < len(histories)-1; i++ {
			if leftOrRight > 0 {
				if histories[i].max.Value == histories[i+1].max.Value {
					idx++
				} else {
					break
				}
			} else {
				if histories[i].min.Value == histories[i+1].min.Value {
					idx++
				} else {
					break
				}
			}
		}
		if value < (histories[idx].min.Value+histories[idx].max.Value)/2 {
			leftOrRight = -1
		} else {
			leftOrRight = 1
		}
	}
	// 超过所有历史之区间
	if idx == -1 {
		idx = len(histories) - 1
		exceeded = true
		if value < histories[idx].min.Value {
			leftOrRight = -1
		}
		if value > histories[idx].max.Value {
			leftOrRight = 1
		}
	}
	return idx, leftOrRight, exceeded
}

func (f *Fund) GetNetValueDate() (time.Time, error) {
	_, loc := utils.GetNow()
	// 获取净值日期
	netValueDate, err := time.ParseInLocation("2006-01-02", f.NetValue.Date, loc)
	return netValueDate, err
}

// 返回当前东八区时间，基金最近的估值时间，以及净值日期
func (f *Fund) getEstimateTime() (time.Time, error) {
	_, loc := utils.GetNow()
	// 获取估值时间
	estimateTime, err := time.ParseInLocation("2006-01-02 15:04", f.Estimate.Datetime, loc)
	return estimateTime, err
}

func findFundHistoryMinMaxNetValues(fundCode string, rangeCode string) (NetValue, NetValue) {
	var min, max NetValue
	res, _ := utils.GetFundHttpsResponse("https://fundcomapi.tiantianfunds.com/mm/newCore/FundVPageDiagram",
		url.Values{"FCODE": {fundCode}, "RANGE": {rangeCode}})
	if res["data"] == nil || len(res["data"].([]interface{})) == 0 {
		log.Printf("未获取到基金 %s 的历史净值数据，可能是基金代码错误或该基金已被清盘", fundCode)
		return min, max
	}
	for _, data := range res["data"].([]interface{}) {
		d := data.(map[string]interface{})
		if d["DWJZ"] == nil {
			log.Printf("基金 %s 历史净值数据缺失 DWJZ 字段，数据内容: %v", fundCode, d)
			continue
		}
		value, err := strconv.ParseFloat(d["DWJZ"].(string), 64)
		if err != nil {
			log.Printf("解析基金 %s 历史净值数据失败: %v", fundCode, err)
			continue
		}
		if min.Value == 0 || value < min.Value {
			min.Value = value
			min.Date = d["FSRQ"].(string)
		}
		if max.Value == 0 || value > max.Value {
			max.Value = value
			max.Date = d["FSRQ"].(string)
		}
	}
	return min, max
}

// BuildFund 获得基金名称以及净值信息
func BuildFund(fundCode string) *Fund {
	res, _ := utils.GetFundHttpsResponse("https://fundmobapi.eastmoney.com/FundMApi/FundBaseTypeInformation.ashx", url.Values{"FCODE": {fundCode}})
	if res["Datas"] == nil {
		log.Printf("未获取到基金 %s 的净值数据，可能是基金代码错误或该基金已被清盘", fundCode)
		return &Fund{
			Code: fundCode,
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
	establishRes, _ := utils.GetFundHttpsResponse("https://fundmobapi.eastmoney.com/FundMNewApi/FundMNDetailInformation", url.Values{"FCODE": {fundCode}})
	if establishRes["Datas"] != nil {
		establishRes = establishRes["Datas"].(map[string]interface{})
		createdDate = establishRes["ESTABDATE"].(string)
	}
	return &Fund{
		Code:        fundCode,
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
	now, loc := utils.GetNow()
	netValueDate, _ := time.ParseInLocation("2006-01-02", f.NetValue.Date, loc)
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
			if !strings.HasPrefix(f.Profit.Estimate, "-") {
				mark = "💹"
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

func (f *Fund) NeedToShowNetValue() bool {
	now, _ := utils.GetNow()
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
