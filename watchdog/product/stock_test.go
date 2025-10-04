package product

import (
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
	s := StockFactory{}.Build("688256.SH")
	if s.Code != "688256" || s.Market != "1" {
		t.Errorf("unexpected stock: %+v", s)
	}
}

func TestStock_RetrieveLatestPrice(t *testing.T) {
	s := StockFactory{}.Build("510210.SH")
	s.Low = 0.7
	s.High = 1.0
	s.RetrieveLatestPrice()
	println(s.PrettyPrint())
	if s.Price == 0 {
		t.Error("Expected latest price to be non-zero")
	}
}

func TestStock_QueryHistoryMinMaxValues(t *testing.T) {
	s := StockFactory{}.Build("002352.SZ")
	ranges := GetHistoryValueRanges(s)
	for _, r := range ranges {
		fmt.Printf("%s: [%.2f,%.2f]\n", r.title, r.min, r.max)
	}
}
