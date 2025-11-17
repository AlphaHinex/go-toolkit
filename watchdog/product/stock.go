package product

import (
	"encoding/json"
	"fmt"
	"go-toolkit/watchdog/product/analysis"
	"go-toolkit/watchdog/utils"
	"log"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Stock struct {
	Code          string                  `yaml:"-"`        // 股票代码，如 300750.SZ
	Name          string                  `yaml:"name"`     // 股票名称
	CreatedAt     time.Time               `yaml:"-"`        // 成立日期
	CreatedDays   int                     `yaml:"-"`        // 成立天数
	MarketValue   float64                 `yaml:"value"`    // 股票市值，单位：亿元
	Low           float64                 `yaml:"low"`      // 监控阈值低点
	High          float64                 `yaml:"high"`     // 监控阈值高点
	Datetime      time.Time               `yaml:"datetime"` // 股票最新更新时间
	Price         float64                 `yaml:"price"`    // 股票最新价格
	HistoryValues []analysis.HistoryValue `yaml:"-"`        // 历史价格数据
}

func (s *Stock) IsTradingDay() bool {
	now := utils.GetNow()
	return utils.IsSameDay(s.Datetime, now)
}

func (s *Stock) IsTradable() bool {
	return s.IsTradingDay() && utils.InOpeningHours()
}

func (s *Stock) QueryHistoryValues() []analysis.HistoryValue {
	jsonObj, err := getLastNDataFromThs(s.Code, 365*100, false)
	if err != nil {
		log.Printf(err.Error())
		return s.HistoryValues
	}
	if jsonObj["data"] == nil {
		log.Printf("No history value data found for stock %s\n", s.Code)
		return s.HistoryValues
	}
	data := jsonObj["data"].(string)
	klines := strings.Split(data, ";")
	for _, kline := range klines {
		// 时间,开盘,最高,最低,收盘
		// 20250930,11.34,11.42,11.18,11.22,41626229,469459340.00,3.500,,,0
		parts := strings.Split(kline, ",")
		if len(parts) < 4 {
			log.Printf("Invalid kline data: %s", kline)
			continue
		}
		date, _ := time.ParseInLocation("20060102", parts[0], utils.GetNow().Location())
		closed, _ := strconv.ParseFloat(parts[4], 64)
		s.HistoryValues = append(s.HistoryValues, analysis.HistoryValue{
			Date:  date,
			Value: closed,
		})
	}
	return s.HistoryValues
}

// StockFactory implements Factory for stocks.
type StockFactory struct{}

func (s StockFactory) GetAllCodes() []string {
	bodyStr := utils.HttpsGet("https://api.biyingapi.com/hslt/list/biyinglicence")
	var jsonArray []map[string]string
	_ = json.Unmarshal(bodyStr, &jsonArray)
	var codes []string
	for _, item := range jsonArray {
		codes = append(codes, item["dm"])
	}
	return codes
}

// Build constructs a Stock instance based on the provided stock code.
// The stock code format should be like "000001.SZ" or "600000.SH".
func (s StockFactory) Build(stockCode string) FinancialProduct {
	now := utils.GetNow()
	jsonObj, _ := getLastNDataFromThs(stockCode, 1, false)
	var createdAt time.Time
	start, ok := jsonObj["start"].(string)
	if ok {
		createdAt, _ = time.ParseInLocation("20060102", start, now.Location())
	}
	days := 0
	totalValue, ok := jsonObj["total"].(string)
	if !ok {
		if fmt.Sprintf("%T", jsonObj["total"]) == "float64" {
			days, _ = jsonObj["total"].(int)
		} else {
			log.Printf("Invalid type for 'total': expected string, got %T %v", jsonObj["total"], jsonObj["total"])
		}
	} else {
		days, _ = strconv.Atoi(totalValue)
	}
	data := strings.Split(jsonObj["data"].(string), ",")
	price := 0.0
	if len(data) < 4 {
		log.Printf("Invalid kline data: %s", jsonObj["data"])
	} else {
		price, _ = strconv.ParseFloat(strings.Split(jsonObj["data"].(string), ",")[4], 64)
	}

	stock := &Stock{
		Code:        stockCode,
		Name:        jsonObj["name"].(string),
		CreatedAt:   createdAt,
		CreatedDays: days,
		//MarketValue: value, // 单位：亿元
		Price:    price,
		Datetime: now,
	}
	stock.RetrieveLatestPrice()
	return stock
}

func (s StockFactory) SiftIn(item interface{}, verbose bool) string {
	stock := item.(*Stock)
	histories := GetHistoryValueRanges(stock)
	historyRow := analysis.MarkValueInHistory(stock.Price, histories)
	matched, _ := regexp.MatchString(`(?s).*[^度]：[^\n]+◀️\n`, historyRow)
	if matched && histories[0].Max.Value-stock.Price > 10 {
		stock.RetrieveMarketValue()
		result := fmt.Sprintf("%s | %s\n%.2f | %.2f亿\n%s\n",
			stock.Code, stock.Name, stock.Price, stock.MarketValue, historyRow)
		if verbose {
			log.Printf("Matched stock: %s", result)
		}
		return result
	}
	return ""
}

func (s *Stock) RetrieveLatestPrice() {
	marketCode, codeNumber := getMarketAndCodeNumber(s.Code)
	// 获取股票最新价格
	reqUrl := fmt.Sprintf("https://push2delay.eastmoney.com/api/qt/stock/trends2/get?"+
		"fields1=f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13&fields2=f51,f53,f56,f58&iscr=0&iscca=0&secid=%s.%s",
		marketCode, codeNumber)
	body := utils.HttpsGet(reqUrl)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("Error unmarshalling JSON response %v:%v", body, err)
	} else if result["data"] != nil {
		data := result["data"].(map[string]interface{})
		s.Name = data["name"].(string)
		trends := data["trends"].([]interface{})
		lastRow := strings.Split(trends[len(trends)-1].(string), ",")
		s.Price, _ = strconv.ParseFloat(lastRow[1], 64)
		now := utils.GetNow()
		s.Datetime, _ = time.ParseInLocation("2006-01-02 15:04", lastRow[0], now.Location())
	}
}

func (s *Stock) RetrieveMarketValue() {
	_, codeNumber := getMarketAndCodeNumber(s.Code)
	licence := os.Getenv("BYAPI_LICENCE")
	if licence != "" {
		bodyStr := utils.HttpsGet(fmt.Sprintf("https://api.biyingapi.com/hscp/gsjj/%s/%s", codeNumber, licence))
		var byRes map[string]interface{}
		_ = json.Unmarshal(bodyStr, &byRes)
		if byRes["rprice"] != nil {
			re := regexp.MustCompile(`\d+`)
			rprice, _ := strconv.ParseFloat(re.FindString(byRes["rprice"].(string)), 64)
			s.MarketValue = math.Round(rprice*s.Price*10_000/100_000_000*100) / 100 // 单位：亿元
		}
	}
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

// getLastNDataFromThs 请求同花顺接口，处理掉 jsonp 函数，返回 json 数据对象
// 目前支持查询日线和月线，默认日线，isMonth 为 true 时查询月线
func getLastNDataFromThs(stockCode string, lastN int, isMonth bool) (map[string]interface{}, error) {
	innerMarketMap := map[string]string{
		"0": "33", // 深证及其他
		"1": "17", // 上证
	}
	marketCode, codeNumber := getMarketAndCodeNumber(stockCode)

	kLineType := "01"
	if isMonth {
		kLineType = "21"
	}
	// 获取股票k线数据
	reqUrl := fmt.Sprintf("https://d.10jqka.com.cn/v6/line/%s_%s/%s/last%d.js",
		innerMarketMap[marketCode], codeNumber, kLineType, lastN)
	bodyStr := string(utils.HttpsGet(reqUrl))
	re := regexp.MustCompile(`(?s)quotebridge_v6_line_\d+_\d+_\d+_last\d+\((.*?)\)$`)
	matches := re.FindStringSubmatch(bodyStr)
	if len(matches) < 2 {
		return nil, fmt.Errorf("Get unusual response format from: %s\n%s", reqUrl, bodyStr)
	}

	var jsonObj map[string]interface{}
	_ = json.Unmarshal([]byte(matches[1]), &jsonObj)
	return jsonObj, nil
}

func getMarketAndCodeNumber(stockCode string) (string, string) {
	// TODO 编码跟具体 API 绑定
	marketMap := map[string]string{
		"SZ":  "0",   // 深证及其他
		"SH":  "1",   // 上证
		"UNK": "2",   // 未知
		"HK":  "116", // 港股
		"US":  "105", // 美股
		"UK":  "155", // 英股
	}

	parts := strings.Split(stockCode, ".")
	if len(parts) != 2 {
		log.Fatalf("Invalid stock code format: %s", stockCode)
		return "", ""
	}
	marketAbbr := strings.ToUpper(parts[1])
	marketCode, exists := marketMap[marketAbbr]
	if !exists {
		log.Fatalf("Unknown market abbreviation: %s", marketAbbr)
		return "", ""
	}
	return marketCode, parts[0]
}
