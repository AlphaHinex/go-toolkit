package product

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestStockFactory_GetAllCodes(t *testing.T) {
	codes := StockFactory{}.GetAllCodes()
	println("total stocks: %d", len(codes))
	for _, c := range codes {
		if strings.HasSuffix(c, ".SZ") || strings.HasSuffix(c, ".SH") {
			continue
		} else {
			t.Errorf("invalid stock code: %s", c)
		}
	}
}

func TestStockFactory_Build(t *testing.T) {
	s := StockFactory{}.Build("601019.SH").(*Stock)
	content, _ := json.Marshal(s)
	fmt.Println(string(content))
	if s.MarketValue == 0 || s.Price == 0 {
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

func TestStock_QueryHistoryMinMaxValues(t *testing.T) {
	s := StockFactory{}.Build("000603.SZ").(*Stock)
	ranges := GetHistoryValueRanges(s)
	fmt.Printf("%s|%s:\n市值：%.2f 亿\n最新成交价：%.2f\n", s.Code, s.Name, s.MarketValue, s.Price)
	for _, r := range ranges {
		fmt.Printf("%s: [%.2f,%.2f]\n", r.Title, r.Min, r.Max)
	}
}

func TestSift(t *testing.T) {
	t.Skip("Skipping TestSift for now")
	result := Sift(StockFactory{}, true)
	if len(result) == 0 {
		t.Error("Expected sift result to be non-empty")
	}
	fmt.Println(result)
}
