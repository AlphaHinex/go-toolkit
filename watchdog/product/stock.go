package product

import (
	"encoding/json"
	"fmt"
	"go-toolkit/watchdog/utils"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var marketMap = map[string]string{
	"SZ":  "0",   // 深证及其他
	"SH":  "1",   // 上证
	"UNK": "2",   // 未知
	"HK":  "116", // 港股
	"US":  "105", // 美股
	"UK":  "155", // 英股
}

type Stock struct {
	Code     string    `yaml:"-"`        // 股票代码
	Market   string    `yaml:"market"`   // 0：其他；1：上证；2：未知；116：港股；105：美股；155：英股
	Name     string    `yaml:"name"`     // 股票名称
	Low      float64   `yaml:"low"`      // 监控阈值低点
	High     float64   `yaml:"high"`     // 监控阈值高点
	Datetime time.Time `yaml:"datetime"` // 股票最新更新时间
	Price    float64   `yaml:"price"`    // 股票最新价格
}

func (s *Stock) IsTradingDay() bool {
	now, _ := utils.GetNow()
	return utils.IsSameDay(s.Datetime, now)
}

func (s *Stock) IsTradable() bool {
	return s.IsTradingDay() && utils.InOpeningHours()
}

// StockFactory implements Factory for stocks.
type StockFactory struct{}

func (s StockFactory) GetAllCodes() []string {
	// Example implementation for stock codes.
	bodyStr := string(utils.HttpsGet("https://api.mairui.club/hslt/list/b997d4403688d5e66a"))
	var jsonArray []map[string]string
	_ = json.Unmarshal([]byte(bodyStr), &jsonArray)
	var codes []string
	for _, item := range jsonArray {
		codes = append(codes, item["dm"])
	}
	return codes
}

// Build constructs a Stock instance based on the provided stock code.
// The stock code format should be like "000001.SZ" or "600000.SH".
func (s StockFactory) Build(stockCode string) *Stock {
	parts := strings.Split(stockCode, ".")
	if len(parts) != 2 {
		log.Fatalf("Invalid stock code format: %s", stockCode)
		return &Stock{}
	}
	code := parts[0]
	marketAbbr := strings.ToUpper(parts[1])
	market, exists := marketMap[marketAbbr]
	if !exists {
		log.Fatalf("Unknown market abbreviation: %s", marketAbbr)
		return &Stock{}
	}
	return &Stock{
		Code:   code,
		Market: market,
	}
}

func (s *Stock) RetrieveLatestPrice() {
	// 获取股票最新价格
	reqUrl := fmt.Sprintf("https://push2.eastmoney.com/api/qt/stock/trends2/get?"+
		"fields1=f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13&fields2=f51,f53,f56,f58&iscr=0&iscca=0&secid=%s.%s",
		s.Market, s.Code)
	req, _ := http.NewRequest("GET", reqUrl, nil)
	resp, err := utils.DoRequestWithRetry(req)
	if err != nil {
		log.Println("Error making GET request:", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err = json.Unmarshal(body, &result); err != nil {
		log.Println("Error unmarshalling JSON response:", err)
	}
	data := result["data"].(map[string]interface{})
	s.Name = data["name"].(string)
	trends := data["trends"].([]interface{})
	lastRow := strings.Split(trends[len(trends)-1].(string), ",")
	s.Price, _ = strconv.ParseFloat(lastRow[1], 64)
	_, loc := utils.GetNow()
	s.Datetime, _ = time.ParseInLocation("2006-01-02 15:04", lastRow[0], loc)
}

// PrettyPrint
// 美化输出，示例如下：
// 510210|上证指数ETF
// 1.20 🔺1.00
// or
// 0.69 ▼ 0.70
func (s *Stock) PrettyPrint() string {
	row := fmt.Sprintf("%s|%s\n", s.Code, s.Name)
	if s.Price > s.High {
		row += fmt.Sprintf("%.4f 🔺%.4f\n", s.Price, s.High)
	} else if s.Price < s.Low {
		row += fmt.Sprintf("%.4f ▼ %.4f\n", s.Price, s.Low)
	} else {
		row += fmt.Sprintf("%.4f (%.4f ~ %.4f)\n", s.Price, s.Low, s.High)
	}
	return row + "\n"
}
