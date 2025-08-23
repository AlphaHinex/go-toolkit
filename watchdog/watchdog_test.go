package main

import (
	"github.com/go-yaml/yaml"
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
