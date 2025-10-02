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

type Stock struct {
	Code     string    `yaml:"-"`        // 股票代码
	Market   string    `yaml:"market"`   // 0：其他；1：上证；2：未知；116：港股；105：美股；155：英股
	Name     string    `yaml:"name"`     // 股票名称
	Low      float64   `yaml:"low"`      // 监控阈值低点
	High     float64   `yaml:"high"`     // 监控阈值高点
	Datetime time.Time `yaml:"datetime"` // 股票最新更新时间
	Price    float64   `yaml:"price"`    // 股票最新价格
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

func (s *Stock) IsTradingDay() bool {
	now, _ := utils.GetNow()
	return utils.IsSameDay(s.Datetime, now)
}

func (s *Stock) IsTradable() bool {
	return s.IsTradingDay() && utils.InOpeningHours()
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
