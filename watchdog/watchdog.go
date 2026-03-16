package main

import (
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"go-toolkit/watchdog/product"
	"go-toolkit/watchdog/product/analysis"
	"go-toolkit/watchdog/service"
	"go-toolkit/watchdog/utils"
	"log"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var verbose bool
var watchNow bool
var showAll bool

func main() {
	app := &cli.App{
		Name:    "watchdog",
		Usage:   "Watchdog of funds & stocks.\nexport BYAPI_LICENCE=xxx && watchdog [flags]",
		Version: "v2.6.2",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config-file",
				Aliases:  []string{"c"},
				Usage:    "Path to the config YAML file containing fund costs, tokens, etc.",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "watch-now",
				Usage:    "Watch funds now, bypassing the predefined watch time points.",
				Value:    false,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "show-all",
				Usage:    "Show all funds regardless of conditions.",
				Value:    false,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "verbose",
				Usage:    "Enable verbose output",
				Value:    false,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "template",
				Aliases:  []string{"t"},
				Usage:    "Generate template file template.yaml in current path.",
				Value:    false,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "sift",
				Aliases:  []string{"s"},
				Usage:    "Sift through all funds&stocks and notify result.",
				Value:    false,
				Required: false,
			},
		},
		Action: func(cCtx *cli.Context) error {
			needTemplate := cCtx.Bool("template")
			configFilePath := cCtx.String("config-file")
			if needTemplate || configFilePath == "" {
				if configFilePath == "" {
					log.Println("需指定配置文件，可基于自动生成的 template.yaml 调整。")
				}
				if runtime.GOOS == "windows" {
					service.ConfigTemplate = strings.ReplaceAll(service.ConfigTemplate, "\n", "\r\n")
				}
				err := os.WriteFile("template.yaml", []byte(strings.TrimSpace(service.ConfigTemplate)), 0644)
				if err != nil {
					log.Fatalf("生成配置文件模板失败: %v", err)
				} else {
					log.Println("生成配置文件模板成功！")
				}
				return nil
			}

			verbose = cCtx.Bool("verbose")
			watchNow = cCtx.Bool("watch-now")
			showAll = cCtx.Bool("show-all")
			configs := service.ReadConfigs(configFilePath)

			needToSift := cCtx.Bool("sift")
			if needToSift {
				notifySiftWithLLM(configs, &product.StockFactory{})
				notifySiftWithLLM(configs, &product.FundFactory{})
				return nil
			}

			fundsMap := configs.Funds
			var funds []*product.Fund
			for key, fund := range fundsMap {
				fund.Code = key
				watchFund(fund)
				funds = append(funds, fund)
			}
			funds = filterFunds(funds)
			sortFunds(funds)

			stocksMap := configs.Stocks
			var stocks []*product.Stock
			for key, persistedStock := range stocksMap {
				if persistedStock.Low == 0 || persistedStock.High == 0 {
					log.Printf("股票 %s 未设置低点和高点，跳过监控\n", key)
					continue
				}
				lastPrice := persistedStock.Price
				stock := product.StockFactory{}.Build(key).(*product.Stock)
				stock.Low = persistedStock.Low
				stock.High = persistedStock.High
				// 股票价格监视不关心监视时间点，只要开盘中超过阈值及上分钟值，每分钟都可发消息
				if product.ShouldShowAll(stock) ||
					(stock.IsTradable() &&
						((stock.Price < stock.Low && stock.Price < lastPrice) ||
							(stock.Price > stock.High && stock.Price > lastPrice))) {
					if persistedStock.Streak.Info == "" && !utils.IsSameDay(persistedStock.Streak.UpdateDate, utils.GetNow()) {
						stock.QueryHistoryValues()
					}
					stocks = append(stocks, stock)
				}
				// 持久化股票最新信息
				persistedStock.Price = stock.Price
				persistedStock.LastDayPrice = stock.LastDayPrice
				persistedStock.Datetime = stock.Datetime
				persistedStock.Streak = stock.Streak
			}

			var message strings.Builder
			for _, fund := range funds {
				message.WriteString(fund.PrettyPrint(showAll))
				if fund.NetValue.Updated {
					fund.Ended = true
				}
			}
			for _, stock := range stocks {
				message.WriteString(stock.PrettyPrint())
			}

			if len(strings.TrimSpace(message.String())) > 0 {
				msg := strings.TrimSpace(addIndexRow() + message.String())
				service.Notify(configs, msg)
			}

			service.WriteConfigs(configFilePath, configs)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func notifySiftWithLLM(configs *service.Config, factory product.Factory) {
	raw := product.Sift(factory, verbose)
	msg, err := analysis.AnalyzeWithLLM(configs.LLM.BaseURL, configs.LLM.APIKey, configs.LLM.Model, factory.LLMPrompt(), raw)
	if err != nil {
		log.Printf("LLM 分析失败，回退原始筛选结果: %v", err)
	}
	msg = strings.TrimSpace(msg)
	if msg == "" || msg == "No data available." {
		return
	}
	service.Notify(configs, msg)
}

func watchFund(fund *product.Fund) {
	// 获取基金最新净值
	retrievedFund := product.FundFactory{}.Build(fund.Code).(*product.Fund)
	fund.Name = retrievedFund.Name
	fund.NetValue = retrievedFund.NetValue
	now := utils.GetNow()
	latestNetValueDate, _ := fund.GetNetValueDate()

	if !utils.IsSameDay(now, latestNetValueDate) {
		fund.Ended = false
	} else {
		fund.NetValue.Updated = true
	}
	if fund.Cost > 0 {
		fund.Profit.Net = fmt.Sprintf("%.2f", (fund.NetValue.Value-fund.Cost)/fund.Cost*100)
	}

	if watchNow || isWatchTime(now) {
		// 获取实时估算净值
		estimate := getFundRealtimeEstimate(fund.Code)
		if estimate != nil {
			changed := !(estimate.Datetime == fund.Estimate.Datetime)
			fund.Estimate = *estimate
			fund.Estimate.Changed = changed
			estimateValue, _ := strconv.ParseFloat(estimate.Value, 64)
			if fund.Cost > 0 {
				fund.Profit.Estimate = fmt.Sprintf("%.2f", (estimateValue-fund.Cost)/fund.Cost*100)
			}
		}
	}
}

// 获取基金实时估算净值
func getFundRealtimeEstimate(fundCode string) *product.Estimate {
	reUrl := fmt.Sprintf("https://fundgz.1234567.com.cn/js/%s.js", fundCode)
	bodyStr := string(utils.HttpsGet(reUrl))
	re := regexp.MustCompile(`jsonpgz\((.*?)\);`)
	matches := re.FindStringSubmatch(bodyStr)
	if len(matches) < 2 {
		return nil
	}

	var e product.Estimate
	if err := json.Unmarshal([]byte(matches[1]), &e); err != nil {
		// 部分基金没有实时估值信息，返回内容为 `jsonpgz();`
		log.Println(fundCode, "未获取到实时估值数据", bodyStr)
	}
	return &e
}

func filterFunds(funds []*product.Fund) []*product.Fund {
	var result []*product.Fund
	for _, f := range funds {
		if product.ShouldShowAll(f) || conditionChain(f) {
			result = append(result, f)
		}
	}
	if verbose {
		log.Printf("Filter funds from %d to %d\n", len(funds), len(result))
	}
	return result
}

func conditionChain(fund *product.Fund) bool {
	now := utils.GetNow()
	estimateMargin, _ := strconv.ParseFloat(fund.Estimate.Margin, 64)
	return isWatchTime(now) && ((fund.IsTradable() && (estimateMargin > 0 || fund.NeedToShowHistory())) || fund.NeedToShowNetValue())
}

// 判断当前时间是否为监测时间点
func isWatchTime(now time.Time) bool {
	hour := now.Hour()
	minute := now.Minute()
	// [09:03~22:03)，每 15 分钟一次，错开整点避免通知限流
	if hour >= 9 && hour <= 21 && minute%15 == 3 {
		return true
	}
	// [14:45~15:00)，每 2 分钟一次
	if hour == 14 && minute >= 45 && minute%2 == 0 {
		return true
	}
	return false
}

func sortFunds(funds []*product.Fund) {
	for i := 0; i < len(funds)-1; i++ {
		for j := i + 1; j < len(funds); j++ {
			if funds[i].NetValue.Updated || funds[j].NetValue.Updated {
				// 按照净值涨幅降序排序
				if funds[i].NetValue.Margin < funds[j].NetValue.Margin {
					funds[i], funds[j] = funds[j], funds[i]
				}
			} else {
				// 按照估值涨幅降序排序
				if funds[i].Estimate.Margin < funds[j].Estimate.Margin {
					funds[i], funds[j] = funds[j], funds[i]
				}
			}
		}
	}
}

func addIndexRow() string {
	indexUrl := "https://push2.eastmoney.com/api/qt/ulist.np/get?fltt=2&fields=f2,f3,f4,f14&secids=1.000001,1.000300,0.399001,0.399006&_=1754373624121"
	indexRes, _ := utils.GetFundHttpsResponse(indexUrl, nil)
	indices := indexRes["data"].(map[string]interface{})["diff"].([]interface{})
	now := utils.GetNow()
	indexRow := fmt.Sprintf("%s\n", now.Format("2006-01-02 15:04:05"))
	for _, index := range indices {
		entry := index.(map[string]interface{})
		indexRow += fmt.Sprintf("%s：%.2f %.2f %s\n", entry["f14"], entry["f2"], entry["f4"], utils.PrefixUpOrDown(fmt.Sprint(entry["f3"])))
	}
	return indexRow + "\n"
}
