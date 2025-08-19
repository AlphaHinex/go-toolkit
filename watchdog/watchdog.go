package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/go-yaml/yaml"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var verbose bool

func main() {
	app := &cli.App{
		Name:    "watchdog",
		Usage:   "Watchdog of fund",
		Version: "v2.5.1",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config-file",
				Aliases:  []string{"c"},
				Usage:    "Path to the config YAML file containing fund costs, tokens, etc.",
				Required: true,
			},
			&cli.BoolFlag{
				Name:     "verbose",
				Usage:    "Enable verbose output",
				Value:    false,
				Required: false,
			},
		},
		Action: func(cCtx *cli.Context) error {
			verbose = cCtx.Bool("verbose")
			configFilePath := cCtx.String("config-file")
			configs := readConfigs(configFilePath)

			fundsMap := configs.Funds
			var funds []*Fund
			for key, fund := range fundsMap {
				fund.Code = key
				watchFund(fund)
				funds = append(funds, fund)
			}
			funds = filterFunds(funds)
			sortFunds(funds)

			var message strings.Builder
			for _, fund := range funds {
				message.WriteString(prettyPrint(*fund))
				if fund.NetValue.Updated {
					fund.Ended = true
				}
			}
			if len(strings.TrimSpace(message.String())) > 0 {
				if configs.Token.Lark != "" {
					sendToLark(configs.Token.Lark, strings.TrimSpace(addIndexRow()+message.String()))
				} else {
					log.Println(strings.TrimSpace(addIndexRow() + message.String()))
				}
			}

			writeConfigs(configFilePath, configs)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
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
	name, netValue := getFundNetValue(fund.Code)
	fund.Name = name
	fund.NetValue = *netValue
	now, _, latestNetValueDate := getDateTimes(*fund)

	if !isSameDay(now, latestNetValueDate) {
		fund.Ended = false
	} else {
		fund.NetValue.Updated = true
	}
	if fund.Cost > 0 {
		fund.Profit.Net = fmt.Sprintf("%.2f", (netValue.Value-fund.Cost)/fund.Cost*100)
	}

	if isWatchTime(now) {
		// 获取实时估算净值
		estimate := getFundRealtimeEstimate(fund.Code)
		if estimate != nil {
			changed := !(estimate.Datetime == fund.Estimate.Datetime)
			fund.Estimate = *estimate
			fund.Estimate.Changed = changed
			estimateValue, _ := strconv.ParseFloat(estimate.Value, 64)
			fund.Profit.Estimate = fmt.Sprintf("%.2f", (estimateValue-fund.Cost)/fund.Cost*100)
		}
	}
}

// 获得基金名称以及净值信息
func getFundNetValue(fundCode string) (string, *NetValue) {
	var netValue NetValue
	netValueRes, _ := getFundHttpsResponse("https://fundmobapi.eastmoney.com/FundMNewApi/FundMNFInfo", url.Values{"Fcodes": {fundCode}})
	netValueRes = netValueRes["Datas"].([]interface{})[0].(map[string]interface{})
	netValue.Value, _ = strconv.ParseFloat(netValueRes["NAV"].(string), 64)
	netValue.Date = netValueRes["PDATE"].(string)
	netValue.Margin, _ = strconv.ParseFloat(netValueRes["NAVCHGRT"].(string), 64)
	return netValueRes["SHORTNAME"].(string), &netValue
}

// 获取基金实时估算净值
func getFundRealtimeEstimate(fundCode string) *Estimate {
	reUrl := fmt.Sprintf("https://fundgz.1234567.com.cn/js/%s.js", fundCode)
	client := &http.Client{}
	req, _ := http.NewRequest("GET", reUrl, nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	bodyStr := string(body)
	re := regexp.MustCompile(`jsonpgz\((.*?)\);`)
	matches := re.FindStringSubmatch(bodyStr)
	if len(matches) < 2 {
		return nil
	}

	var e Estimate
	_ = json.Unmarshal([]byte(matches[1]), &e)
	if err := json.Unmarshal([]byte(matches[1]), &e); err != nil {
		// 部分基金没有实时估值信息，返回内容为 `jsonpgz();`
		log.Println(fundCode, "未获取到实时估值数据", bodyStr)
	}
	return &e
}

func findFundHistoryMinMaxNetValues(fundCode string, rangeCode string) (NetValue, NetValue) {
	var min, max NetValue
	res, _ := getFundHttpsResponse("https://fundcomapi.tiantianfunds.com/mm/newCore/FundVPageDiagram",
		url.Values{"FCODE": {fundCode}, "RANGE": {rangeCode}})
	for _, data := range res["data"].([]interface{}) {
		value, _ := strconv.ParseFloat(data.(map[string]interface{})["DWJZ"].(string), 64)
		if min.Value == 0 || value < min.Value {
			min.Value = value
			min.Date = data.(map[string]interface{})["FSRQ"].(string)
		}
		if max.Value == 0 || value > max.Value {
			max.Value = value
			max.Date = data.(map[string]interface{})["FSRQ"].(string)
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
	return fmt.Sprintf("▼ %.2f%%", v)
}

func getFundHttpsResponse(getUrl string, params url.Values) (map[string]interface{}, string) {
	var (
		DeviceID = "874C427C-7C24-4980-A835-66FD40B67605"
		Version  = "6.5.5"
	)

	// GET 请求通用参数
	var commonParams = url.Values{
		"product":       {"EFund"},
		"deviceid":      {DeviceID},
		"MobileKey":     {DeviceID},
		"plat":          {"Iphone"},
		"PhoneType":     {"IOS15.1.0"},
		"OSVersion":     {"15.5"},
		"version":       {Version},
		"ServerVersion": {Version},
		"Version":       {Version},
		"appVersion":    {Version},
	}

	fullURL := getUrl + "?" + commonParams.Encode() + "&" + params.Encode()

	// 1. 创建自定义Transport（支持HTTPS）
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // 生产环境应设为false并配置CA证书
		},
	}

	// 2. 创建HTTP客户端
	client := &http.Client{Transport: tr}

	// 3. 创建请求对象
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		panic(err)
	}

	// 4. 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1 Edg/94.0.4606.71")

	// 5. 发送请求
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error making GET request:", err)
		return nil, ""
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, string(body)
	}
	return result, ""
}

func getNow() (time.Time, *time.Location) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	// 获取当前时间并转换为东八区时间
	now := time.Now().In(loc)
	return now, loc
}

func filterFunds(funds []*Fund) []*Fund {
	var result []*Fund
	now, _ := getNow()
	for _, f := range funds {
		if showAll(now, f) || conditionChain(f) {
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
	return isWatchTime(now) && ((isOpening(*fund) && (estimateMargin > 0 || needToShowHistory(*fund))) || needToShowNetValue(*fund))
}

func showAll(now time.Time, fund *Fund) bool {
	hour := now.Hour()
	minute := now.Minute()
	return isTradingDay(*fund) &&
		((isOpening(*fund) && !inOpeningBreakTime(now) && minute == 48) || (hour == 21 && minute == 48))
}

func isTradingDay(fund Fund) bool {
	now, estimateTime, _ := getDateTimes(fund)
	return isSameDay(now, estimateTime)
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

// 判断是否开盘中
func isOpening(fund Fund) bool {
	now, _ := getNow()
	if isTradingDay(fund) && inOpeningHours(now) {
		if verbose {
			log.Printf("开盘中 %s\n", fund.Name)
		}
		return true
	} else {
		if verbose {
			log.Printf("非开盘时间 %s\n", fund.Name)
		}
		return false
	}
}

func needToShowNetValue(fund Fund) bool {
	now, _, netValueDate := getDateTimes(fund)
	if isTradingDay(fund) && inOpeningBreakTime(now) && fund.Estimate.Changed {
		log.Printf("%s 已更新上午最新估值\n", fund.Name)
		return true
	} else if isTradingDay(fund) && !isOpening(fund) && fund.Estimate.Changed {
		log.Printf("%s 已更新下午最新估值\n", fund.Name)
		return true
	} else if isTradingDay(fund) && isSameDay(now, netValueDate) &&
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

func inOpeningHours(t time.Time) bool {
	hour := t.Hour()
	minute := t.Minute()

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

func inOpeningBreakTime(t time.Time) bool {
	hour := t.Hour()
	minute := t.Minute()
	return (hour == 11 && minute >= 30) || (hour == 12)
}

// 美化输出，示例如下：
// 008099|广发价值领先混合A
// 成本：1.5258
// 估值：1.4914 ▼ -0.32% -2.25% 15:00
// 净值：1.4969 🔺0.05% -1.89% 2025-08-08
// 月度：1.4818 → 1.5752
// 季度：1.4325 → 1.5752
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
	costRow := fmt.Sprintf("成本：%.4f\n", fund.Cost)
	netRow := fmt.Sprintf("净值：%.4f %s %s%% %s\n",
		fund.NetValue.Value,
		upOrDown(fmt.Sprint(fund.NetValue.Margin)),
		fund.Profit.Net,
		fund.NetValue.Date)
	estimateRow := fmt.Sprintf("估值：%s %s %s%% %s\n",
		fund.Estimate.Value,
		upOrDown(fund.Estimate.Margin),
		fund.Profit.Estimate,
		strings.Split(fund.Estimate.Datetime, " ")[1])

	result := title + costRow

	now, _, _ := getDateTimes(fund)
	if inOpeningBreakTime(now) {
		// 如果是交易日的午休时间，先显示上一日估值，再显示当日净值
		result += netRow + estimateRow
	} else if !fund.NetValue.Updated && needToShowHistory(fund) {
		// 交易日当日净值未更新且需要显示历史净值时，先显示上一日估值，再显示当日净值
		historyRow := ""
		for _, s := range []string{"y|月度", "3y|季度", "6y|半年", "n|一年", "3n|三年", "5n|五年", "ln|成立"} {
			min, max := findFundHistoryMinMaxNetValues(fund.Code, strings.Split(s, "|")[0])
			historyRow += fmt.Sprintf("%s：%.4f → %.4f\n", strings.Split(s, "|")[1], min.Value, max.Value)
		}
		result += netRow + estimateRow + historyRow
	} else {
		if isOpening(fund) {
			// 开盘中显示实时估值
			result += estimateRow
		} else if needToShowNetValue(fund) {
			if fund.NetValue.Updated {
				// 交易日净值更新后，先显示最后的估值，再显示当日最终净值
				result += estimateRow + netRow
			} else {
				result += netRow + estimateRow
			}
		} else if showAll(now, &fund) {
			result += estimateRow + netRow
		}
	}
	return result + "\n"
}

func needToShowHistory(fund Fund) bool {
	estimateMargin, _ := strconv.ParseFloat(fund.Estimate.Margin, 64)
	estimateProfit, _ := strconv.ParseFloat(fund.Profit.Estimate, 64)
	if !(fund.NetValue.Margin+estimateMargin > 0 || (fund.NetValue.Margin+estimateMargin < -1 && fund.NetValue.Margin*estimateMargin > 0)) {
		// 不满足下面任一条件时，不显示历史
		// 1. 前日净值+当日估值涨幅为正
		// 2. 前日净值及当日估值均下跌，且总跌幅大于1%
		return false
	}
	if isOpening(fund) && estimateMargin > 0 && estimateProfit > 0 {
		log.Printf("%s 开盘中，且估值涨幅大于0(%f)；估值收益率大于0(%f)\n", fund.Name, estimateMargin, estimateProfit)
		return true
	}
	if isOpening(fund) && estimateMargin < -1 && estimateProfit < 0 {
		log.Printf("%s 开盘中，且估值跌幅超1(%f)；估值收益率小于0(%f)\n", fund.Name, estimateMargin, estimateProfit)
		return true
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
	indexRes, _ := getFundHttpsResponse(indexUrl, nil)
	indices := indexRes["data"].(map[string]interface{})["diff"].([]interface{})
	now, _ := getNow()
	indexRow := fmt.Sprintf("%s\n", now.Format("2006-01-02 15:04"))
	for _, index := range indices {
		entry := index.(map[string]interface{})
		indexRow += fmt.Sprintf("%s：%.2f %.2f %s\n", entry["f14"], entry["f2"], entry["f4"], upOrDown(fmt.Sprint(entry["f3"])))
	}
	return indexRow + "\n"
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

	client := &http.Client{}
	resp, _ := client.Do(req)
	log.Println("飞书返回状态: ", resp.Status)
	if resp.StatusCode != 200 {
		log.Println(resp.Body)
	}
	defer resp.Body.Close()
}

type Config struct {
	Funds map[string]*Fund `yaml:"funds"`
	Token struct {
		Lark string `yaml:"lark"`
	} `yaml:"token"`
}

type Fund struct {
	Code     string   `yaml:"-"`        // 基金代码
	Name     string   `yaml:"name"`     // 基金名称
	Cost     float64  `yaml:"cost"`     // 基金成本价
	NetValue NetValue `yaml:"net"`      // 基金净值
	Estimate Estimate `yaml:"estimate"` // 实时估算净值
	Profit   struct {
		Estimate string `yaml:"-"` // 实时估算净值收益率
		Net      string `yaml:"-"` // 基金净值收益率
	} `yaml:"-"` // 基金净值收益率
	Ended bool `yaml:"ended"` // 当日监测是否已结束
}

// Estimate 实时估值结构体
type Estimate struct {
	Value    string `json:"gsz" yaml:"-"`           // 实时估算净值
	Margin   string `json:"gszzl" yaml:"-"`         // 实时估算涨跌幅
	Datetime string `json:"gztime" yaml:"datetime"` // 实时估算时间
	Changed  bool   `json:"-" yaml:"changed"`       // 实时估算是否变动
}

type NetValue struct {
	Value   float64 `yaml:"-"`       // 净值
	Margin  float64 `yaml:"-"`       // 净值涨跌幅百分比
	Date    string  `yaml:"date"`    // 净值日期
	Updated bool    `yaml:"updated"` // 是否已更新净值
}
