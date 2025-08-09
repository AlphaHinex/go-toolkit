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
			&cli.StringFlag{
				Name:     "lark-webhook-token",
				Usage:    "Lark webhook token for notifications",
				Required: false,
			},
		},
		Action: func(cCtx *cli.Context) error {
			verbose = cCtx.Bool("verbose")
			configFilePath := cCtx.String("config-file")
			configs := readConfigs(configFilePath)

			costs := configs.Funds
			var funds []Fund
			for code, cost := range costs {
				f := Fund{
					Code: code,
					Cost: cost,
				}
				watchFund(&f)
				funds = append(funds, f)
			}
			// TODO: Implement filtering and sorting of funds
			//filterFunds(funds)
			//sortFunds(funds)

			var message strings.Builder
			for _, fund := range funds {
				message.WriteString(prettyPrint(fund))
			}
			if configs.Token.Lark != "" && len(strings.TrimSpace(message.String())) > 0 {
				sendToLark(configs.Token.Lark, strings.TrimSpace(message.String()))
			}

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

func watchFund(fund *Fund) {
	// 获取基金净值
	name, netValue := getFundNetValue(fund.Code)
	fund.Name = name
	fund.NetValue = *netValue
	// 获取实时估算净值
	estimate := getFundRealtimeEstimate(fund.Code)
	fund.Estimate = *estimate
	// 计算收益率
	fund.NetProfit = fmt.Sprintf("%.2f", (netValue.Value-fund.Cost)/fund.Cost*100)
	estimateValue, _ := strconv.ParseFloat(estimate.Gsz, 64)
	fund.EstimateProfit = fmt.Sprintf("%.2f", (estimateValue-fund.Cost)/fund.Cost*100)
}

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
	if err := json.Unmarshal([]byte(matches[1]), &e); err != nil {
		panic(err.Error())
		return nil
	}
	return &e
}

func findFundHistoryMinMaxNetValues(fundCode string, rangeCode string) (NetValue, NetValue) {
	var min, max NetValue
	res, _ := getFundHttpsResponse("https://fundcomapi.tiantianfunds.com/mm/newCore/FundVPageDiagram",
		url.Values{"FCODE": {fundCode}, "RANGE": {rangeCode}})
	for _, data := range res["data"].([]interface{}) {
		value, _ := strconv.ParseFloat(data.(map[string]interface {})["DWJZ"].(string), 64)
		if min.Value == 0 || value < min.Value {
			min.Value = value
			min.Date = data.(map[string]interface {})["FSRQ"].(string)
		}
		if max.Value == 0 || value > max.Value {
			max.Value = value
			max.Date = data.(map[string]interface {})["FSRQ"].(string)
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
		if verbose {
			fmt.Println("Error making GET request:", err)
		}
		return nil, ""
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	if verbose {
		fmt.Println(string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, string(body)
	}
	return result, ""
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
func prettyPrint(fund Fund) string {
	today := time.Now().Format("2006-01-02")
	if today > fund.NetValue.Date {
		return ""
	}
	title := fmt.Sprintf("%s|%s\n", fund.Code, fund.Name)
	costRow := fmt.Sprintf("成本：%.4f\n", fund.Cost)
	netRow := fmt.Sprintf("净值：%.4f %s %s%% %s\n",
		fund.NetValue.Value,
		upOrDown(fmt.Sprint(fund.NetValue.Margin)),
		fund.NetProfit,
		fund.NetValue.Date)
	estimateRow := fmt.Sprintf("估值：%s %s %s%% %s\n",
		fund.Estimate.Gsz,
		upOrDown(fund.Estimate.Gszzl),
		fund.EstimateProfit,
		strings.Split(fund.Estimate.Gztime, " ")[1])

	if fund.NetValue.Date == strings.Split(fund.Estimate.Gztime, " ")[0] {
		historyRow := ""
		for _, s := range []string{"y|月度", "3y|季度", "6y|半年", "n|一年", "3n|三年", "5n|五年", "ln|成立"} {
			min, max := findFundHistoryMinMaxNetValues(fund.Code, strings.Split(s, "|")[0])
			historyRow += fmt.Sprintf("%s：%.4f → %.4f\n", strings.Split(s, "|")[1], min.Value, max.Value)
		}
		return title + costRow + netRow + historyRow + "\n"
	} else {
		return title + costRow + estimateRow + "\n"
	}
}

// 发送消息到飞书
func sendToLark(larkWebhookToken, msg string) {
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
	defer resp.Body.Close()
}

type Config struct {
	Funds map[string]float64 `yaml:"funds"`
	Token struct {
		Lark string `yaml:"lark"`
	} `yaml:"token"`
}

type Fund struct {
	Code           string   // 基金代码
	Name           string   // 基金名称
	Cost           float64  // 基金成本价
	NetValue       NetValue // 基金净值
	NetProfit      string   // 基金净值收益率
	Estimate       Estimate // 实时估算净值
	EstimateProfit string   // 实时估算净值收益率
	Hint           string   // 提示信息
}

// Estimate 实时估值结构体
type Estimate struct {
	Gsz    string `json:"gsz"`    // 实时估算净值
	Gszzl  string `json:"gszzl"`  // 实时估算涨跌幅
	Gztime string `json:"gztime"` // 实时估算时间
}

type NetValue struct {
	Value  float64 // 净值
	Margin float64 // 净值涨跌幅百分比
	Date   string  // 净值日期
}
