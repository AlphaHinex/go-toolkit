package utils

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func GetFundHttpsResponse(getUrl string, params url.Values) (map[string]interface{}, string) {
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

	// 创建请求对象
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		log.Panic(err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1 Edg/94.0.4606.71")

	// 发送请求
	resp, err := DoRequestWithRetry(req)
	if err != nil {
		log.Println("Error making GET request:", err)
		return nil, ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, string(body)
	}
	return result, ""
}

func HttpsGet(url string) []byte {
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := DoRequestWithRetry(req)
	if err != nil {
		log.Println("Error making GET request:", err)
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return body
}

// DoRequestWithRetry 执行 HTTP 请求并支持重试机制
func DoRequestWithRetry(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	var maxRetries, retryDelay = 3, 2 * time.Second

	// 1. 创建自定义Transport（支持HTTPS）
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // 生产环境应设为false并配置CA证书
		},
	}
	// 2. 创建HTTP客户端
	client := &http.Client{Transport: tr}

	for i := 0; i <= maxRetries; i++ {
		resp, err = client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			if i > 0 {
				log.Printf("%s 请求第 %d 次成功.\n", req.URL, i+1)
			}
			return resp, nil
		}

		// 如果不是最后一次重试，等待一段时间后重试
		if i < maxRetries {
			log.Printf("%s 请求失败，稍后第 %d 次重试...\n错误信息：\n%v\n", req.URL, i+1, err)
			time.Sleep(time.Duration(i) * retryDelay)
		}
	}

	// 返回最后一次的响应或错误
	return resp, err
}

// GetLastNDataFromThs 请求同花顺接口，处理掉 jsonp 函数，返回 json 数据对象
// 目前支持查询日线和月线，默认日线，isMonth 为 true 时查询月线
func GetLastNDataFromThs(stockCode string, lastN int, isMonth bool) (map[string]interface{}, error) {
	innerMarketMap := map[string]string{
		"0": "33", // 深证及其他
		"1": "17", // 上证
	}
	marketCode, codeNumber := GetMarketAndCodeNumber(stockCode)

	kLineType := "01"
	if isMonth {
		kLineType = "21"
	}
	// 获取股票k线数据
	reqUrl := fmt.Sprintf("https://d.10jqka.com.cn/v6/line/%s_%s/%s/last%d.js",
		innerMarketMap[marketCode], codeNumber, kLineType, lastN)
	bodyStr := string(HttpsGet(reqUrl))
	re := regexp.MustCompile(`(?s)quotebridge_v6_line_\d+_\d+_\d+_last\d+\((.*?)\)$`)
	matches := re.FindStringSubmatch(bodyStr)
	if len(matches) < 2 {
		return nil, fmt.Errorf("Get unusual response format from: %s\n%s", reqUrl, bodyStr)
	}

	var jsonObj map[string]interface{}
	_ = json.Unmarshal([]byte(matches[1]), &jsonObj)
	return jsonObj, nil
}

func GetMarketAndCodeNumber(stockCode string) (string, string) {
	// TODO 编码跟具体 API 绑定
	marketMap := map[string]string{
		"SZ":  "0",   // 深证及其他
		"SH":  "1",   // 上证
		"UNK": "2",   // 未知
		"HK":  "116", // 港股
		"US":  "105", // 美股
		"UK":  "155", // 英股
	}

	parts := strings.Split(stockCode, ".")
	if len(parts) != 2 {
		log.Fatalf("Invalid stock code format: %s", stockCode)
		return "", ""
	}
	marketAbbr := strings.ToUpper(parts[1])
	marketCode, exists := marketMap[marketAbbr]
	if !exists {
		log.Fatalf("Unknown market abbreviation: %s", marketAbbr)
		return "", ""
	}
	return marketCode, parts[0]
}
