package main

import (
	"fmt"
	"github.com/go-yaml/yaml"
	"go-toolkit/watchdog/product"
	"go-toolkit/watchdog/service"
	"go-toolkit/watchdog/utils"
	"strings"
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

func TestUpdatePersistedStock_NameUpdatedWhenLatestNonEmpty(t *testing.T) {
	persisted := &product.Stock{Name: "旧名称", Price: 1}
	latest := &product.Stock{Name: "新名称", Price: 2}

	updatePersistedStock(persisted, latest)

	if strings.TrimSpace(persisted.Name) != "新名称" {
		t.Fatalf("expected persisted name updated, got: %q", persisted.Name)
	}
	if persisted.Price != 2 {
		t.Fatalf("expected persisted price updated, got: %v", persisted.Price)
	}
}

func TestUpdatePersistedStock_NamePreservedWhenLatestEmpty(t *testing.T) {
	persisted := &product.Stock{Name: "旧名称", Price: 1}
	latest := &product.Stock{Name: " ", Price: 2}

	updatePersistedStock(persisted, latest)

	if persisted.Name != "旧名称" {
		t.Fatalf("expected persisted name preserved, got: %q", persisted.Name)
	}
	if persisted.Price != 2 {
		t.Fatalf("expected persisted price updated, got: %v", persisted.Price)
	}
}

func TestUpdatePersistedStock_NameRules(t *testing.T) {
	tests := []struct {
		name          string
		persistedName string
		latestName    string
		expectedName  string
	}{
		{name: "updated when latest non-empty", persistedName: "旧名称", latestName: "新名称", expectedName: "新名称"},
		{name: "preserved when latest empty", persistedName: "旧名称", latestName: "", expectedName: "旧名称"},
		{name: "preserved when latest whitespace", persistedName: "旧名称", latestName: "  ", expectedName: "旧名称"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			persisted := &product.Stock{Name: tt.persistedName}
			latest := &product.Stock{Name: tt.latestName}
			updatePersistedStock(persisted, latest)
			if persisted.Name != tt.expectedName {
				t.Fatalf("expected name %q, got %q", tt.expectedName, persisted.Name)
			}
		})
	}
}
