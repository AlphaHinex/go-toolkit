package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-yaml/yaml"
	"testing"
	"time"
)

func TestGetFundNetValue(t *testing.T) {
	code := "002401"
	fund := buildFund(code)
	if fund.Name == "" {
		t.Error("Expected fund name to be non-empty")
	}
	if fund.NetValue.Date == "" {
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

func TestCompose(t *testing.T) {
	fund := buildFund("011130")
	fmt.Printf("%s|%s\n最新净值：%.4f\n%s\n", fund.Code, fund.Name, fund.NetValue.Value, fund.composeHistoryRow(fund.NetValue.Value))
	content, _ := json.Marshal(fund)
	fmt.Println(string(content))
}

func TestSift(t *testing.T) {
	result := sift()
	if len(result) == 0 {
		t.Error("Expected sift result to be non-empty")
	}
	fmt.Println(result)
}
