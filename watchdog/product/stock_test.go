package product

import (
	"strings"
	"testing"
)

func TestRetrieveLatestPrice(t *testing.T) {
	s := Stock{
		Code:   "510210",
		Market: "1",
		Low:    0.7,
		High:   1.0,
	}
	s.RetrieveLatestPrice()
	println(s.PrettyPrint())
	if s.Price == 0 {
		t.Error("Expected latest price to be non-zero")
	}
}

func TestGetAllCodes(t *testing.T) {
	codes := StockCodeProvider{}.GetAllCodes()
	println("total stocks: %d", len(codes))
	for _, c := range codes {
		if strings.HasSuffix(c, ".SZ") || strings.HasSuffix(c, ".SH") {
			continue
		} else {
			t.Errorf("invalid stock code: %s", c)
		}
	}
}
