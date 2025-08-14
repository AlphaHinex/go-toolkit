package main

import (
	"github.com/go-yaml/yaml"
	"testing"
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
  "501203":
    name: 易方达创新未来混合(LOF)
    cost: 0.9294
    net:
      date: "2025-08-14"
      updated: false
    estimate:
      datetime: 2025-08-14 15:00
      changed: false
    ended: true`
	var config Config
	_ = yaml.Unmarshal([]byte(yamlText), &config)
	now, _, latestNetValueDate := getDateTimes(*config.Funds["501203"])
	if !isSameDay(now, latestNetValueDate) {
		t.Error("Expected net value date to be today")
	}
}
