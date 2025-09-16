package main

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

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

	// 创建请求对象
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		log.Panic(err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1 Edg/94.0.4606.71")

	// 发送请求
	resp, err := doRequestWithRetry(req)
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

func httpsGet(url string) []byte {
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := doRequestWithRetry(req)
	if err != nil {
		log.Println("Error making GET request:", err)
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return body
}

// doRequestWithRetry 执行 HTTP 请求并支持重试机制
func doRequestWithRetry(req *http.Request) (*http.Response, error) {
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
