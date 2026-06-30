package product

import (
	"encoding/json"
	"fmt"
	"go-toolkit/watchdog/product/analysis"
	"strings"
	"testing"
	"time"
)

func TestStockFactory_GetAllCodes(t *testing.T) {
	codes := StockFactory{}.GetAllCodes()
	println("total stocks: ", len(codes))
	if len(codes) != 5186 {
		t.Errorf("expected 5160=>5164=>5168=>5186 stocks, got %d", len(codes))
	}
	for _, c := range codes {
		if strings.HasSuffix(c, ".SZ") || strings.HasSuffix(c, ".SH") {
			continue
		} else {
			t.Errorf("invalid stock code: %s", c)
		}
	}
}

func TestStockFactory_Build(t *testing.T) {
	s := StockFactory{}.Build("002594.SZ").(*Stock)
	content, _ := json.Marshal(s)
	fmt.Println(string(content))
	//if s.MarketValue == 0 || s.Price == 0 {
	if s.Price == 0 {
		t.Errorf("unexpected stock: %+v", s)
	}
}

func TestStock_RetrieveLatestPrice(t *testing.T) {
	s := StockFactory{}.Build("510210.SH").(*Stock)
	s.Low = 0.7
	s.High = 1.0
	s.RetrieveLatestPrice()
	println(s.PrettyPrint())
	if s.Price == 0 {
		t.Error("Expected latest price to be non-zero")
	}
}

func TestStock_GetHistoryValueRanges(t *testing.T) {
	s := StockFactory{}.Build("688765.SH").(*Stock)
	ranges := GetHistoryValueRanges(s)
	fmt.Printf("%s|%s:\n上市日期：%s\n市值：%.2f 亿\n最新成交价：%.2f\n", s.Code, s.Name, s.CreatedAt, s.MarketValue, s.Price)
	fmt.Println(s.Streak.Info)
	fmt.Printf(analysis.MarkValueInHistory(s.Price, ranges, s.Price > s.LastDayPrice))
}

func TestStockFactory_SiftIn(t *testing.T) {
	s := StockFactory{}.Build("600036.SH").(*Stock)
	result := StockFactory{}.SiftIn(s, true)
	if result == "" {
		t.Error("Expected sift in result to be non-empty")
	}
	fmt.Println(result)
}

func TestSift(t *testing.T) {
	t.Skip("Skipping TestSift for now")
	result := Sift(StockFactory{}, true)
	if len(result) == 0 {
		t.Error("Expected sift result to be non-empty")
	}
	fmt.Println(result)
}

func TestStock_GetPeriodStats(t *testing.T) {
	now := time.Now()
	s := &Stock{Price: 11}
	for i := 0; i < 10; i++ {
		s.HistoryValues = append(s.HistoryValues, analysis.HistoryValue{
			Date:  now.AddDate(0, 0, -10+i),
			Value: float64(i + 1),
		})
	}

	stats := s.GetPeriodStats([]int{5, 10, 20})
	if !stats[5].Available || stats[5].Min != 6 || stats[5].Max != 10 || stats[5].MA != 8 {
		t.Fatalf("unexpected 5-day stats: %+v", stats[5])
	}
	if !stats[10].Available || stats[10].Min != 1 || stats[10].Max != 10 || stats[10].MA != 5.5 {
		t.Fatalf("unexpected 10-day stats: %+v", stats[10])
	}
	if stats[20].Available {
		t.Fatalf("expected 20-day stats unavailable: %+v", stats[20])
	}
}

func TestStock_ComposePeriodStatsRows(t *testing.T) {
	now := time.Now()
	s := &Stock{Price: 10}
	for i := 0; i < 30; i++ {
		s.HistoryValues = append(s.HistoryValues, analysis.HistoryValue{
			Date:  now.AddDate(0, 0, -30+i),
			Value: 9,
		})
	}

	row := s.ComposePeriodStatsRows()
	if !strings.Contains(row, "5日：") || !strings.Contains(row, "10日：") || !strings.Contains(row, "20日：") {
		t.Fatalf("expected period labels in row, got:\n%s", row)
	}
	if !strings.Contains(row, "[9.0000, 9.0000]") || !strings.Contains(row, "MA=9.0000") {
		t.Fatalf("expected min/max/MA output in row, got:\n%s", row)
	}
	if !strings.Contains(row, "高于MA") {
		t.Fatalf("expected price relation in row, got:\n%s", row)
	}
	if !strings.Contains(row, "120日：N/A") || !strings.Contains(row, "250日：N/A") {
		t.Fatalf("expected unavailable period markers in row, got:\n%s", row)
	}
}

func TestStock_ComposePeriodStatsRows_Order(t *testing.T) {
	now := time.Now()
	s := &Stock{Price: 10}
	for i := 0; i < 300; i++ {
		s.HistoryValues = append(s.HistoryValues, analysis.HistoryValue{
			Date:  now.AddDate(0, 0, -300+i),
			Value: float64(i%10 + 1),
		})
	}
	row := s.ComposePeriodStatsRows()
	lines := strings.Split(strings.TrimSpace(row), "\n")
	expected := []string{"5日", "10日", "20日", "60日", "120日", "250日"}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d: %v", len(expected), len(lines), lines)
	}
	for i, prefix := range expected {
		if !strings.HasPrefix(lines[i], prefix) {
			t.Fatalf("line %d expected prefix %s, got %s", i, prefix, lines[i])
		}
	}
}
