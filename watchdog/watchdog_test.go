package main

import (
	"fmt"
	"github.com/go-yaml/yaml"
	"os"
	"testing"
	"time"
)

func TestGetFundNetValue(t *testing.T) {
	code := "002401"
	name, netValue := getFundNetValue(code)
	if name == "" {
		t.Error("Expected fund name to be non-empty")
	}
	if netValue.Date == "" {
		t.Error("Expected net value date to be non-empty")
	}
}

func TestNetValueUpdated(t *testing.T) {
	yamlText := `
funds:
  501203:
    cost: 1.0000
    net:
      date: "2025-08-14"
      updated: false
    estimate:
      datetime: 2025-08-14 15:00
      changed: false
    ended: true`
	var config Config
	_ = yaml.Unmarshal([]byte(yamlText), &config)
	_, _, latestNetValueDate := getDateTimes(*config.Funds["501203"])
	_, loc := getNow()
	now, _ := time.ParseInLocation("2006-01-02 15:04", "2025-08-14 18:00", loc)
	if !isSameDay(now, latestNetValueDate) {
		t.Error("Expected net value date to be today")
	}
}

func TestAddIndexRow(t *testing.T) {
	index := addIndexRow()
	if len(index) == 0 {
		t.Error("Expected index to be non-nil")
	}
}

func TestSendToDingTalk(t *testing.T) {
	token := "xxx"
	message := `hinex
2025-08-22 15:03`
	sendToDingTalk(token, message)
}

func TestQueryStreakInfo(t *testing.T) {
	f := Fund{
		Code: "008099",
	}
	f.queryStreakInfo()
	fmt.Print(f.Streak)
	if f.Streak.Info == "" {
		t.Error("Expected streak info to be non-empty")
	}
}

func TestRetrieveLatestPrice(t *testing.T) {
	s := Stock{
		Code:   "510210",
		Market: "1",
		Low:    0.7,
		High:   1.0,
	}
	s.retrieveLatestPrice()
	fmt.Println(s.prettyPrint())
	if s.Price == 0 {
		t.Error("Expected latest price to be non-zero")
	}
}

func TestUseEmojiNumber(t *testing.T) {
	if useEmojiNumber(1234567890) != "1️⃣2️⃣3️⃣4️⃣5️⃣6️⃣7️⃣8️⃣9️⃣0️⃣" {
		t.Error("Expected emoji number is wrong")
	}
}

func TestGetAllFundCodes(t *testing.T) {
	codes := getAllFundCodes()
	println(len(codes))
	if len(codes) == 0 {
		t.Error("Expected fund codes to be non-empty")
	}

	// 打开文件用于写入
	file, err := os.OpenFile("/Users/alphahinex/Desktop/funds.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("无法打开文件: %v\n", err)
		return
	}
	defer file.Close()

	for _, code := range codes {
		println("Choose code: " + code)
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("捕获到异常，跳过当前基金代码 %s: %v\n", code, r)
				}
			}()

			// 可能引发异常的代码
			name, netValue := getFundNetValue(code)
			fund := Fund{
				Code:     code,
				Name:     name,
				NetValue: *netValue,
			}
			historyRow := fund.composeHistoryRow(netValue.Value)
			// 写入文件
			_, err = fmt.Fprintf(file, "%s|%s\n最新净值：%.4f\n%s\n", code, name, netValue.Value, historyRow)
			if err != nil {
				fmt.Printf("写入文件失败: %v\n", err)
			}
		}()
	}
}
