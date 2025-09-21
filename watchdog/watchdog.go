package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-yaml/yaml"
	"github.com/urfave/cli/v2"
	"go-toolkit/watchdog/utils"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var verbose bool
var watchNow bool
var showAll bool

var configTemplate = fmt.Sprintf(`
funds:
  008099: # 基金代码
    cost: 1.6078 # 基金成本价
  000083: 
    cost: 5.1727

stocks:
  510210: # 股票代码
    market: 1 # 0：其他；1：上证；2：未知；116：港股；105：美股；155：英股
    low: 0.7 # 监控阈值低点 
    high: 1.0 # 监控阈值高点

token:
  lark: xxxxxx # 飞书机器人 Webhook token，可选
  dingtalk: xxxxxx # 钉钉机器人 Webhook token，可选`)

func main() {
	app := &cli.App{
		Name:    "watchdog",
		Usage:   "Watchdog of fund",
		Version: "v2.6.2",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config-file",
				Aliases:  []string{"c"},
				Usage:    "Path to the config YAML file containing fund costs, tokens, etc.",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "watch-now",
				Usage:    "Watch funds now, bypassing the predefined watch time points.",
				Value:    false,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "show-all",
				Usage:    "Show all funds regardless of conditions.",
				Value:    false,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "verbose",
				Usage:    "Enable verbose output",
				Value:    false,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "template",
				Aliases:  []string{"t"},
				Usage:    "Generate template file template.yaml in current path.",
				Value:    false,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "sift",
				Aliases:  []string{"s"},
				Usage:    "Sift through all funds and notify result.",
				Value:    false,
				Required: false,
			},
		},
		Action: func(cCtx *cli.Context) error {
			needTemplate := cCtx.Bool("template")
			configFilePath := cCtx.String("config-file")
			if needTemplate || configFilePath == "" {
				if configFilePath == "" {
					log.Println("需指定配置文件，可基于自动生成的 template.yaml 调整。")
				}
				if runtime.GOOS == "windows" {
					configTemplate = strings.ReplaceAll(configTemplate, "\n", "\r\n")
				}
				err := os.WriteFile("template.yaml", []byte(strings.TrimSpace(configTemplate)), 0644)
				if err != nil {
					log.Fatalf("生成配置文件模板失败: %v", err)
				} else {
					log.Println("生成配置文件模板成功！")
				}
				return nil
			}

			verbose = cCtx.Bool("verbose")
			watchNow = cCtx.Bool("watch-now")
			showAll = cCtx.Bool("show-all")
			configs := readConfigs(configFilePath)

			needToSift := cCtx.Bool("sift")
			if needToSift {
				notify(configs, sift())
				return nil
			}

			fundsMap := configs.Funds
			var funds []*Fund
			for key, fund := range fundsMap {
				fund.Code = key
				watchFund(fund)
				funds = append(funds, fund)
			}
			funds = filterFunds(funds)
			sortFunds(funds)

			stocksMap := configs.Stocks
			var stocks []*Stock
			for key, stock := range stocksMap {
				stock.Code = key
				if stock.Market == "" {
					stock.Market = "1" // 默认上证
				}
				if stock.Low == 0 || stock.High == 0 {
					log.Printf("股票 %s 未设置低点和高点，跳过监控\n", stock.Code)
					continue
				}
				lastPrice := stock.Price
				stock.retrieveLatestPrice()
				// 股票价格监视不关心监视时间点，只要开盘中超过阈值及上分钟值，每分钟都可发消息
				if shouldShowAll(stock) ||
					(stock.isTradable() &&
						((stock.Price < stock.Low && stock.Price < lastPrice) ||
							(stock.Price > stock.High && stock.Price > lastPrice))) {
					stocks = append(stocks, stock)
				}
			}

			var message strings.Builder
			for _, fund := range funds {
				message.WriteString(prettyPrint(*fund))
				if fund.NetValue.Updated {
					fund.Ended = true
				}
			}
			for _, stock := range stocks {
				message.WriteString(stock.prettyPrint())
			}

			if len(strings.TrimSpace(message.String())) > 0 {
				msg := strings.TrimSpace(addIndexRow() + message.String())
				notify(configs, msg)
			}

			writeConfigs(configFilePath, configs)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func notify(configs *Config, msg string) {
	if configs.Token.Lark == "" && configs.Token.DingTalk == "" {
		log.Println(msg)
	}
	if configs.Token.Lark != "" {
		sendToLark(configs.Token.Lark, msg)
	}
	if configs.Token.DingTalk != "" {
		sendToDingTalk(configs.Token.DingTalk, msg)
	}
}

func readConfigs(configsFilePath string) *Config {
	content, err := os.ReadFile(configsFilePath)
	if err != nil {
		log.Panicf("读取配置 %s 失败: %v", configsFilePath, err)
	}
	var config Config
	err = yaml.Unmarshal(content, &config)
	if err != nil {
		log.Panicf("解析配置失败: %v", err)
	}
	return &config
}

func writeConfigs(configFilePath string, configs *Config) {
	// 将 Configs 内容序列化至文件
	content, err := yaml.Marshal(configs)
	if err != nil {
		log.Panicf("序列化配置失败: %v", err)
	}
	if err := os.WriteFile(configFilePath, content, 0644); err != nil {
		log.Panicf("写入配置文件 %s 失败: %v", configFilePath, err)
	}
}

func watchFund(fund *Fund) {
	// 获取基金最新净值
	retrievedFund := buildFund(fund.Code)
	fund.Name = retrievedFund.Name
	fund.NetValue = retrievedFund.NetValue
	now, _, latestNetValueDate := getDateTimes(*fund)

	if !isSameDay(now, latestNetValueDate) {
		fund.Ended = false
	} else {
		fund.NetValue.Updated = true
	}
	if fund.Cost > 0 {
		fund.Profit.Net = fmt.Sprintf("%.2f", (fund.NetValue.Value-fund.Cost)/fund.Cost*100)
	}

	if watchNow || isWatchTime(now) {
		// 获取实时估算净值
		estimate := getFundRealtimeEstimate(fund.Code)
		if estimate != nil {
			changed := !(estimate.Datetime == fund.Estimate.Datetime)
			fund.Estimate = *estimate
			fund.Estimate.Changed = changed
			estimateValue, _ := strconv.ParseFloat(estimate.Value, 64)
			if fund.Cost > 0 {
				fund.Profit.Estimate = fmt.Sprintf("%.2f", (estimateValue-fund.Cost)/fund.Cost*100)
			}
		}
	}
}

func getAllFundCodes() []string {
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

// 获得基金名称以及净值信息
func buildFund(fundCode string) *Fund {
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

// 获取基金实时估算净值
func getFundRealtimeEstimate(fundCode string) *Estimate {
	reUrl := fmt.Sprintf("https://fundgz.1234567.com.cn/js/%s.js", fundCode)
	bodyStr := string(utils.HttpsGet(reUrl))
	re := regexp.MustCompile(`jsonpgz\((.*?)\);`)
	matches := re.FindStringSubmatch(bodyStr)
	if len(matches) < 2 {
		return nil
	}

	var e Estimate
	if err := json.Unmarshal([]byte(matches[1]), &e); err != nil {
		// 部分基金没有实时估值信息，返回内容为 `jsonpgz();`
		log.Println(fundCode, "未获取到实时估值数据", bodyStr)
	}
	return &e
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

// 添加涨跌符号
func upOrDown(value string) string {
	v, _ := strconv.ParseFloat(value, 64)
	if v > 0 {
		return fmt.Sprintf("🔺%.2f%%", v)
	}
	if v == 0 {
		return fmt.Sprintf(" %.2f%%", v)
	}
	return fmt.Sprintf("▼ %.2f%%", v)
}

func getNow() (time.Time, *time.Location) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		// Windows 环境使用 time.LoadLocation 报 panic: time: missing Location in call to Time.In
		loc = time.FixedZone("CST", 8*3600)
	}
	// 获取当前时间并转换为东八区时间
	now := time.Now().In(loc)
	return now, loc
}

func filterFunds(funds []*Fund) []*Fund {
	var result []*Fund
	for _, f := range funds {
		if shouldShowAll(f) || conditionChain(f) {
			result = append(result, f)
		}
	}
	if verbose {
		log.Printf("Filter funds from %d to %d\n", len(funds), len(result))
	}
	return result
}

func conditionChain(fund *Fund) bool {
	now, _ := getNow()
	estimateMargin, _ := strconv.ParseFloat(fund.Estimate.Margin, 64)
	return isWatchTime(now) && ((fund.isTradable() && (estimateMargin > 0 || needToShowHistory(*fund))) || needToShowNetValue(*fund))
}

// 根据当前时间判断是否需要显示全部金融产品信息
// 满足一下任一条件时，显示全部信息：
// 1. 如果是交易日的开盘时间，且当前分钟为 48 分钟
// 2. 交易日收盘后的 21:48
func shouldShowAll(product FinancialProduct) bool {
	if showAll {
		return true
	}
	now, _ := getNow()
	hour := now.Hour()
	minute := now.Minute()
	return product.isTradingDay() &&
		((product.isTradable() && minute == 48) || (hour == 21 && minute == 48))
}

// 判断当前时间是否为监测时间点
func isWatchTime(now time.Time) bool {
	hour := now.Hour()
	minute := now.Minute()
	// [09:03~22:03)，每 15 分钟一次，错开整点避免通知限流
	if hour >= 9 && hour <= 21 && minute%15 == 3 {
		return true
	}
	// [14:45~15:00)，每 2 分钟一次
	if hour == 14 && minute >= 45 && minute%2 == 0 {
		return true
	}
	return false
}

// 返回当前东八区时间，基金最近的估值时间，以及净值日期
func getDateTimes(fund Fund) (time.Time, time.Time, time.Time) {
	now, loc := getNow()
	// 获取估值时间
	estimateTime, _ := time.ParseInLocation("2006-01-02 15:04", fund.Estimate.Datetime, loc)
	// 获取净值日期
	netValueDate, _ := time.ParseInLocation("2006-01-02", fund.NetValue.Date, loc)
	return now, estimateTime, netValueDate
}

func needToShowNetValue(fund Fund) bool {
	now, _, netValueDate := getDateTimes(fund)
	if fund.isTradingDay() && inBreakingTime() && fund.Estimate.Changed {
		log.Printf("%s 已更新上午最新估值\n", fund.Name)
		return true
	} else if fund.isTradingDay() && !fund.isTradable() && fund.Estimate.Changed {
		log.Printf("%s 已更新下午最新估值\n", fund.Name)
		return true
	} else if fund.isTradingDay() && isSameDay(now, netValueDate) &&
		!fund.Ended && fund.NetValue.Updated {
		log.Printf("%s 今日净值已更新\n", fund.Name)
		return true
	} else {
		if verbose {
			log.Printf("无需显示净值 %s\n", fund.Name)
		}
		return false
	}
}

func isSameDay(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() && t1.Month() == t2.Month() && t1.Day() == t2.Day()
}

func inOpeningHours() bool {
	now, _ := getNow()
	hour := now.Hour()
	minute := now.Minute()

	// 上午9:30-11:30
	if (hour == 9 && minute >= 30) || (hour > 9 && hour < 11) || (hour == 11 && minute <= 30) {
		return true
	}
	// 下午13:00-15:00
	if (hour == 13) || (hour > 13 && hour < 15) || (hour == 15 && minute == 0) {
		return true
	}
	return false
}

func inBreakingTime() bool {
	now, _ := getNow()
	hour := now.Hour()
	minute := now.Minute()
	return (hour == 11 && minute >= 30) || (hour == 12)
}

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
func prettyPrint(fund Fund) string {
	title := fmt.Sprintf("%s|%s\n", fund.Code, fund.Name)
	costRow := ""
	if fund.Cost > 0 {
		costRow = fmt.Sprintf("成本：%.4f\n", fund.Cost)
	}
	now, loc := getNow()
	netValueDate, _ := time.ParseInLocation("2006-01-02", fund.NetValue.Date, loc)
	netValueDateStr := "前日"
	if isSameDay(now, netValueDate) {
		netValueDateStr = "今日"
	}
	netRow := ""
	if fund.Cost > 0 {
		netRow = fmt.Sprintf("净值：%.4f %s %s 总 %s%%\n",
			fund.NetValue.Value,
			netValueDateStr,
			upOrDown(fmt.Sprint(fund.NetValue.Margin)),
			fund.Profit.Net)
	} else {
		netRow = fmt.Sprintf("净值：%.4f %s %s\n",
			fund.NetValue.Value,
			netValueDateStr,
			upOrDown(fmt.Sprint(fund.NetValue.Margin)))
	}

	estimateRow := ""
	if len(fund.Estimate.Value) > 0 {
		estimateRow = fmt.Sprintf("估值：%s %s %s%% %s\n",
			fund.Estimate.Value,
			upOrDown(fund.Estimate.Margin),
			fund.Profit.Estimate,
			strings.Split(fund.Estimate.Datetime, " ")[1])
	}
	result := title + costRow

	if fund.isTradingDay() && inBreakingTime() {
		// 如果是交易日的午休时间，先显示上一日估值，再显示当日净值
		result += netRow + estimateRow
	} else if showAll || (!fund.NetValue.Updated && needToShowHistory(fund)) {
		estimateValue, _ := strconv.ParseFloat(fund.Estimate.Value, 64)
		historyRow := ""
		if estimateValue > 0 {
			historyRow = fund.composeHistoryRow(estimateValue)
		} else {
			historyRow = fund.composeHistoryRow(fund.NetValue.Value)
		}
		// 交易日当日净值未更新且需要显示历史净值时，先显示上一日估值，再显示当日净值
		result += netRow + estimateRow + historyRow
	} else {
		if fund.isTradable() {
			// 开盘中显示实时估值
			result += estimateRow
		} else if needToShowNetValue(fund) {
			if fund.NetValue.Updated {
				// 交易日净值更新后，先显示最后的估值，再显示当日最终净值
				result += estimateRow + netRow
			} else {
				result += netRow + estimateRow
			}
		} else if shouldShowAll(&fund) {
			result += estimateRow + netRow
		}
	}
	return result + "\n"
}

func needToShowHistory(fund Fund) bool {
	if fund.isTradingDay() && (inOpeningHours() || inBreakingTime()) {
		estimateMargin, _ := strconv.ParseFloat(fund.Estimate.Margin, 64)
		estimateProfit, _ := strconv.ParseFloat(fund.Profit.Estimate, 64)
		if estimateMargin > 0 && estimateProfit > 0 && fund.NetValue.Margin+estimateMargin > 0 {
			// 前日净值+当日估值涨幅为正时，可考虑卖出
			log.Printf("%s 开盘中，且估值涨幅大于0(%f)；估值收益率大于0(%f)\n", fund.Name, estimateMargin, estimateProfit)
			return true
		}
		if estimateMargin < 0 && estimateProfit < 0 && fund.NetValue.Margin+estimateMargin < -1 && fund.NetValue.Margin < 0 {
			// 前日净值及当日估值均下跌，且总跌幅大于1%，可考虑买入
			log.Printf("%s 开盘中，且估值跌幅超1(%f)；估值收益率小于0(%f)\n", fund.Name, estimateMargin, estimateProfit)
			return true
		}
	}
	return false
}

func sortFunds(funds []*Fund) {
	for i := 0; i < len(funds)-1; i++ {
		for j := i + 1; j < len(funds); j++ {
			if funds[i].NetValue.Updated || funds[j].NetValue.Updated {
				// 按照净值涨幅降序排序
				if funds[i].NetValue.Margin < funds[j].NetValue.Margin {
					funds[i], funds[j] = funds[j], funds[i]
				}
			} else {
				// 按照估值涨幅降序排序
				if funds[i].Estimate.Margin < funds[j].Estimate.Margin {
					funds[i], funds[j] = funds[j], funds[i]
				}
			}
		}
	}
}

func addIndexRow() string {
	indexUrl := "https://push2.eastmoney.com/api/qt/ulist.np/get?fltt=2&fields=f2,f3,f4,f14&secids=1.000001,1.000300,0.399001,0.399006&_=1754373624121"
	indexRes, _ := utils.GetFundHttpsResponse(indexUrl, nil)
	indices := indexRes["data"].(map[string]interface{})["diff"].([]interface{})
	now, _ := getNow()
	indexRow := fmt.Sprintf("%s\n", now.Format("2006-01-02 15:04:05"))
	for _, index := range indices {
		entry := index.(map[string]interface{})
		indexRow += fmt.Sprintf("%s：%.2f %.2f %s\n", entry["f14"], entry["f2"], entry["f4"], upOrDown(fmt.Sprint(entry["f3"])))
	}
	return indexRow + "\n"
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

func useEmojiNumber(num int) string {
	str := strconv.Itoa(num)
	str = strings.ReplaceAll(str, "0", "0️⃣")
	str = strings.ReplaceAll(str, "1", "1️⃣")
	str = strings.ReplaceAll(str, "2", "2️⃣")
	str = strings.ReplaceAll(str, "3", "3️⃣")
	str = strings.ReplaceAll(str, "4", "4️⃣")
	str = strings.ReplaceAll(str, "5", "5️⃣")
	str = strings.ReplaceAll(str, "6", "6️⃣")
	str = strings.ReplaceAll(str, "7", "7️⃣")
	str = strings.ReplaceAll(str, "8", "8️⃣")
	str = strings.ReplaceAll(str, "9", "9️⃣")
	return str
}

// 发送消息到飞书
func sendToLark(larkWebhookToken, msg string) {
	log.Println("准备发送消息到飞书: ", msg)
	larkWebhook := "https://open.feishu.cn/open-apis/bot/v2/hook/" + larkWebhookToken

	payload := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": msg,
		},
	}

	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", larkWebhook, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := utils.DoRequestWithRetry(req)
	log.Println("飞书返回状态: ", resp.Status)
	if resp.StatusCode != 200 {
		log.Println(resp.Body)
	}
	defer resp.Body.Close()
}

func sendToDingTalk(dingTalkToken, msg string) {
	payload := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": msg,
		},
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Failed to marshal payload:", err)
		return
	}
	req, err := http.NewRequest("POST",
		"https://oapi.dingtalk.com/robot/send?access_token="+dingTalkToken, bytes.NewBuffer(jsonPayload))

	if err != nil {
		fmt.Println(err)
		return
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := utils.DoRequestWithRetry(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer res.Body.Close()

	_, err = io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
}

type Config struct {
	Funds  map[string]*Fund  `yaml:"funds"`
	Stocks map[string]*Stock `yaml:"stocks"`
	Token  struct {
		Lark     string `yaml:"lark"`
		DingTalk string `yaml:"dingtalk"`
	} `yaml:"token"`
}

type FinancialProduct interface {
	isTradingDay() bool // 当天是否是交易日
	isTradable() bool   // 当前是否可交易
}

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

func (f *Fund) isTradingDay() bool {
	now, estimateTime, _ := getDateTimes(*f)
	return isSameDay(now, estimateTime)
}

func (f *Fund) isTradable() bool {
	return f.isTradingDay() && inOpeningHours()
}

// 查询最近一个月的连续上涨或下跌信息
// 连续 3️⃣ 天 🔺2.05% 1.4818 ↗️ 1.5752
// 连续 1️⃣2️⃣ 天 ▼ 2.05% 1.5752 ↘️ 1.4818
func (f *Fund) queryStreakInfo() {
	now, _ := getNow()
	if f.Streak.Info != "" && isSameDay(f.Streak.UpdateDate, now) {
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
		f.Streak.Info = fmt.Sprintf("连续 %s 天 🔺%.2f%% %.4f ↗️ %.4f", useEmojiNumber(riseStreak), netValueMargin, netValueFrom, netValueTo)
	} else if fallStreak > 0 {
		f.Streak.Info = fmt.Sprintf("连续 %s 天 ▼ %.2f%% %.4f ↘️ %.4f", useEmojiNumber(fallStreak), netValueMargin, netValueFrom, netValueTo)
	}
	f.Streak.UpdateDate = now
}

/**
 * 生成历史净值区间行
 * 包括历史净值连续涨跌信息
 * 以及不同阶段的历史净值区间
 * 并根据传入的 markValue 值，在历史区间中标记出所在位置
 */
func (f *Fund) composeHistoryRow(markValue float64) string {
	f.queryStreakInfo()
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

type Stock struct {
	Code     string    `yaml:"-"`        // 股票代码
	Market   string    `yaml:"market"`   // 0：其他；1：上证；2：未知；116：港股；105：美股；155：英股
	Name     string    `yaml:"name"`     // 股票名称
	Low      float64   `yaml:"low"`      // 监控阈值低点
	High     float64   `yaml:"high"`     // 监控阈值高点
	Datetime time.Time `yaml:"datetime"` // 股票最新更新时间
	Price    float64   `yaml:"price"`    // 股票最新价格
}

func (s *Stock) retrieveLatestPrice() {
	// 获取股票最新价格
	reqUrl := fmt.Sprintf("https://push2.eastmoney.com/api/qt/stock/trends2/get?"+
		"fields1=f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13&fields2=f51,f53,f56,f58&iscr=0&iscca=0&secid=%s.%s",
		s.Market, s.Code)
	req, _ := http.NewRequest("GET", reqUrl, nil)
	resp, err := utils.DoRequestWithRetry(req)
	if err != nil {
		log.Println("Error making GET request:", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err = json.Unmarshal(body, &result); err != nil {
		log.Println("Error unmarshalling JSON response:", err)
	}
	data := result["data"].(map[string]interface{})
	s.Name = data["name"].(string)
	trends := data["trends"].([]interface{})
	lastRow := strings.Split(trends[len(trends)-1].(string), ",")
	s.Price, _ = strconv.ParseFloat(lastRow[1], 64)
	_, loc := getNow()
	s.Datetime, _ = time.ParseInLocation("2006-01-02 15:04", lastRow[0], loc)
}

func (s *Stock) isTradingDay() bool {
	now, _ := getNow()
	return isSameDay(s.Datetime, now)
}

func (s *Stock) isTradable() bool {
	return s.isTradingDay() && inOpeningHours()
}

// 美化输出，示例如下：
// 510210|上证指数ETF
// 1.20 🔺1.00
// or
// 0.69 ▼ 0.70
func (s *Stock) prettyPrint() string {
	row := fmt.Sprintf("%s|%s\n", s.Code, s.Name)
	if s.Price > s.High {
		row += fmt.Sprintf("%.4f 🔺%.4f\n", s.Price, s.High)
	} else if s.Price < s.Low {
		row += fmt.Sprintf("%.4f ▼ %.4f\n", s.Price, s.Low)
	} else {
		row += fmt.Sprintf("%.4f (%.4f ~ %.4f)\n", s.Price, s.Low, s.High)
	}
	return row + "\n"
}

type HistoryNetValueRange struct {
	title string
	min   NetValue
	max   NetValue
}

func sift() string {
	codes := getAllFundCodes()

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
			fund := buildFund(code)

			if !fund.Status.Valid && verbose {
				log.Printf("跳过无法购买的基金: %s\n", code)
				return
			}
			historyRow := fund.composeHistoryRow(fund.NetValue.Value)
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
