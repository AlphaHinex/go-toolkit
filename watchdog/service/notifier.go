package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-toolkit/watchdog/utils"
	"io"
	"log"
	"net/http"
	"strings"
)

func Notify(configs *Config, msg string) {
	msg = strings.TrimSpace(msg)
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
