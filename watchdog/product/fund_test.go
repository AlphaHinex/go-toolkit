package product

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestFundFactory_Build(t *testing.T) {
	fund := FundFactory{}.Build("002401")
	println(fund.Name)
	println(fund.NetValue.Date)
	if fund.Name == "" {
		t.Error("Expected fund name to be non-empty")
	}
	if fund.NetValue.Date == "" {
		t.Error("Expected net value date to be non-empty")
	}
	content, _ := json.Marshal(fund)
	fmt.Println(string(content))
}

func TestFund_ComposeHistoryRow(t *testing.T) {
	fund := FundFactory{}.Build("011130")
	row := fund.ComposeHistoryRow(fund.NetValue.Value)
	fmt.Printf("%s|%s\n最新净值：%.4f\n%s\n", fund.Code, fund.Name, fund.NetValue.Value, row)
	if row == "" {
		t.Error("Expected history row to be non-empty")
	}
}
