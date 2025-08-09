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
				message.WriteString(fmt.Sprint(fund))
			}
			if configs.Token.Lark != "" {
				sendToFeishu(configs.Token.Lark, message.String())
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
	netValueRes, _ := getFundHttpsResponse("https://fundmobapi.eastmoney.com/FundMNewApi/FundMNFInfo", url.Values{"Fcodes": {fund.Code}})
	netValueRes = netValueRes["Datas"].([]interface{})[0].(map[string]interface{})
	netValue := netValueRes["NAV"].(string)
	pDate := netValueRes["PDATE"].(string)
	navChgRt := netValueRes["NAVCHGRT"].(string)

	// 获取实时估算净值
	estimate := getFundRealtimeEstimate(fund.Code)
	if estimate == nil {
		return
	}

	var profitStr string
	netVal, _ := strconv.ParseFloat(netValue, 64)
	profit := (netVal - fund.Cost) / fund.Cost * 100
	profitStr = fmt.Sprintf("%.2f", profit)
	profitStr = upOrDown(profitStr)

	fund.Name = netValueRes["SHORTNAME"].(string)
	fund.Estimate.Gszzl = estimate.Gszzl
	fund.Estimate.Gztime = estimate.Gztime
	fund.Estimate.Gsz = estimate.Gsz
	fund.NavChgRt = navChgRt
	fund.NetValue = netVal
	fund.NetValueDate = pDate
	fund.Profit = profitStr

	//res := fmt.Sprintf(
	//	"%s|%s\n%s 成本价：%s\n%s 估算涨跌幅：%s 估算净值：%s\n%s 涨跌幅：%s 净值：%s（收益率：%s）\n------------------------------------------------------\n",
	//	fundCode,
	//	netValueRes["SHORTNAME"].(string),
	//	time.Now().Format("2006-01-02"),
	//	costStr,
	//	estimate.Gztime,
	//	strings.ReplaceAll(upOrDown(estimate.Gszzl), "▲", "🔺"),
	//	estimate.Gsz,
	//	pDate,
	//	upOrDown(navChgRt),
	//	netValue,
	//	profitStr,
	//)
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
		return nil
	}
	return &e
}

// 涨跌幅格式
func upOrDown(value string) string {
	v, _ := strconv.ParseFloat(value, 64)
	if v > 0 {
		return fmt.Sprintf("%.2f%% ▲", v)
	}
	return fmt.Sprintf("%.2f%% ▼", v)
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

// 发送消息到飞书
func sendToFeishu(larkWebhookToken, msg string) {
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
	Code         string   // 基金代码
	Name         string   // 基金名称
	Cost         float64  // 基金成本价
	NetValue     float64  // 基金净值
	NetValueDate string   // 净值日期
	NavChgRt     string   // 净值涨跌幅
	Profit       string   // 基金收益率
	Estimate     Estimate // 实时估算净值
	Hint         string   // 提示信息
}

// Estimate 实时估值结构体
type Estimate struct {
	Gsz    string `json:"gsz"`    // 实时估算净值
	Gszzl  string `json:"gszzl"`  // 实时估算涨跌幅
	Gztime string `json:"gztime"` // 实时估算时间
}
