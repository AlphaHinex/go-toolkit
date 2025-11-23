package main

import (
	"fmt"
	"github.com/go-yaml/yaml"
	"go-toolkit/watchdog/product"
	"go-toolkit/watchdog/service"
	"go-toolkit/watchdog/utils"
	"testing"
	"time"
)

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
	var config service.Config
	_ = yaml.Unmarshal([]byte(yamlText), &config)
	latestNetValueDate, _ := config.Funds["501203"].GetNetValueDate()
	now, _ := time.ParseInLocation("2006-01-02 15:04", "2025-08-14 18:00", utils.GetNow().Location())
	if !utils.IsSameDay(now, latestNetValueDate) {
		t.Error("Expected net value date to be today")
	}
}

func TestAddIndexRow(t *testing.T) {
	index := addIndexRow()
	if len(index) == 0 {
		t.Error("Expected index to be non-nil")
	}
}

func TestQueryStreakInfo(t *testing.T) {
	f := product.Fund{
		Code: "008099",
	}
	f.QueryHistoryValues()
	fmt.Print(f.Streak)
	if f.Streak.Info == "" {
		t.Error("Expected streak info to be non-empty")
	}
}

func TestUseEmojiNumber(t *testing.T) {
	if utils.TurnToEmojiNumber(1234567890) != "1️⃣2️⃣3️⃣4️⃣5️⃣6️⃣7️⃣8️⃣9️⃣0️⃣" {
		t.Error("Expected emoji number is wrong")
	}
}

func TestSift(t *testing.T) {
	t.Skip("Skipping TestSift for now")
	result := product.Sift(product.StockFactory{}, true)
	if len(result) == 0 {
		t.Error("Expected sift result to be non-empty")
	}
	fmt.Println(result)
}
