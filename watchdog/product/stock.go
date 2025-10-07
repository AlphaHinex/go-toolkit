package product

import (
	"encoding/json"
	"fmt"
	"go-toolkit/watchdog/product/analysis"
	"go-toolkit/watchdog/utils"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Stock struct {
	Code        string    `yaml:"-"`        // 股票代码，如 300750.SZ
	Name        string    `yaml:"name"`     // 股票名称
	MarketValue float64   `yaml:"value"`    // 股票市值，单位：亿元
	Low         float64   `yaml:"low"`      // 监控阈值低点
	High        float64   `yaml:"high"`     // 监控阈值高点
	Datetime    time.Time `yaml:"datetime"` // 股票最新更新时间
	Price       float64   `yaml:"price"`    // 股票最新价格
}

func (s *Stock) IsTradingDay() bool {
	now, _ := utils.GetNow()
	return utils.IsSameDay(s.Datetime, now)
}

func (s *Stock) IsTradable() bool {
	return s.IsTradingDay() && utils.InOpeningHours()
}

func (s *Stock) QueryHistoryMinMaxValues(rangeStr string) (float64, float64) {
	rangeMapping := map[string]string{
		"m":   "01|30",   // 日k|30天
		"3m":  "01|90",   // 日k|90天
		"6m":  "01|180",  // 日k|180天
		"y":   "01|365",  // 日k|365天
		"3y":  "21|36",   // 月k|36个月
		"5y":  "21|60",   // 月k|60个月
		"all": "21|1200", // 月k|1200个月
	}

	innerMarketMap := map[string]string{
		"0": "33", // 深证及其他
		"1": "17", // 上证
	}

	params, exists := rangeMapping[rangeStr]
	if !exists {
		log.Fatalf("Invalid range string: %s", rangeStr)
		return 0, 0
	}
	ktlAndLmt := strings.Split(params, "|")
	marketCode, codeNumber := s.getMarketAndCodeNumber()
	// 获取股票k线数据
	reqUrl := fmt.Sprintf("https://d.10jqka.com.cn/v6/line/%s_%s/%s/last%s.js",
		innerMarketMap[marketCode], codeNumber, ktlAndLmt[0], ktlAndLmt[1])
	bodyStr := string(utils.HttpsGet(reqUrl))

	re := regexp.MustCompile(`(?s)quotebridge_v6_line_\d+_\d+_\d+_last\d+\((.*?)\)$`)
	matches := re.FindStringSubmatch(bodyStr)
	if len(matches) < 2 {
		log.Printf("No k-line data found for stock %s in range %s\n", s.Code, rangeStr)
		return 0, 0
	}

	var jsonObj map[string]interface{}
	_ = json.Unmarshal([]byte(matches[1]), &jsonObj)
	if jsonObj["data"] == nil {
		log.Printf("No data found for stock %s in range %s\n", s.Code, rangeStr)
		return 0, 0
	}
	data := jsonObj["data"].(string)
	klines := strings.Split(data, ";")
	var min, max float64
	for _, kline := range klines {
		// 时间,开盘,最高,最低,收盘
		// 20250930,11.34,11.42,11.18,11.22,41626229,469459340.00,3.500,,,0
		parts := strings.Split(kline, ",")
		high, _ := strconv.ParseFloat(parts[2], 64)
		low, _ := strconv.ParseFloat(parts[3], 64)
		if min == 0 || low < min {
			min = low
		}
		if max == 0 || high > max {
			max = high
		}
	}
	return min, max
}

// StockFactory implements Factory for stocks.
type StockFactory struct{}

func (s StockFactory) GetAllCodes() []string {
	bodyStr := utils.HttpsGet("https://api.mairui.club/hslt/list/b997d4403688d5e66a")
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
	marketCode, codeNumber := (&Stock{Code: stockCode}).getMarketAndCodeNumber()
	reqUrl := fmt.Sprintf("https://push2.eastmoney.com/api/qt/stock/get?invt=2"+
		"&fields=f19,f20,f23,f24,f25,f26,f27,f28,f29,f30,f43,f44,f45,f46,f47,f48,f49,f50,f57,f58,f59,f60,f113,f114,f115,f116,f117,f127,f130,f131,f132,f133,f135,f136,f137,f138,f139,f140,f141,f142,f143,f144,f145,f146,f147,f148,f149,f152,f161,f162,f164,f165,f167,f168,f169,f170,f171,f174,f175,f177,f178,f198,f199,f294,f530,f531"+
		"&secid=%s.%s", marketCode, codeNumber)
	bodyStr := utils.HttpsGet(reqUrl)
	var jsonObject map[string]interface{}
	_ = json.Unmarshal(bodyStr, &jsonObject)
	if jsonObject["data"] == nil {
		log.Printf("No data found for stock code: %s", stockCode)
		return &Stock{
			Code: stockCode,
		}
	}
	data := jsonObject["data"].(map[string]interface{})
	now, _ := utils.GetNow()
	value, err := strconv.ParseFloat(fmt.Sprint(data["f116"]), 64)
	if err != nil {
		log.Printf("Error parsing market value for stock %s: %v", stockCode, err)
		value = 0
	}
	price, err := strconv.ParseFloat(fmt.Sprint(data["f43"]), 64)
	if err != nil {
		log.Printf("Error parsing price for stock %s: %v", stockCode, err)
		price = 0
	}
	return &Stock{
		Code:        stockCode,
		Name:        data["f58"].(string),
		MarketValue: value / 100_000_000, // 单位：亿元
		Price:       price / 100,
		Datetime:    now,
	}
}

func (s StockFactory) SiftIn(item interface{}, verbose bool) string {
	stock := item.(*Stock)
	if stock.MarketValue < 10 { // 市值小于10亿的股票不监控
		return ""
	}
	histories := GetHistoryValueRanges(stock)
	historyRow := analysis.MarkValueInHistory(stock.Price, histories)
	matched, _ := regexp.MatchString(`(?s).*[^度]：[^\n]+◀️\n`, historyRow)
	if matched && histories[0].Max-stock.Price > 10 {
		result := fmt.Sprintf("%s|%s\n%.2f|%.2f亿\n%s\n",
			stock.Code, stock.Name, stock.Price, stock.MarketValue, historyRow)
		if verbose {
			log.Printf("Matched stock: %s", result)
		}
		return result
	}
	return ""
}

var marketMap = map[string]string{
	"SZ":  "0",   // 深证及其他
	"SH":  "1",   // 上证
	"UNK": "2",   // 未知
	"HK":  "116", // 港股
	"US":  "105", // 美股
	"UK":  "155", // 英股
}

func (s *Stock) getMarketAndCodeNumber() (string, string) {
	parts := strings.Split(s.Code, ".")
	if len(parts) != 2 {
		log.Fatalf("Invalid stock code format: %s", s.Code)
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

func (s *Stock) RetrieveLatestPrice() {
	marketCode, codeNumber := s.getMarketAndCodeNumber()
	// 获取股票最新价格
	reqUrl := fmt.Sprintf("https://push2.eastmoney.com/api/qt/stock/trends2/get?"+
		"fields1=f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13&fields2=f51,f53,f56,f58&iscr=0&iscca=0&secid=%s.%s",
		marketCode, codeNumber)
	body := utils.HttpsGet(reqUrl)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
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
