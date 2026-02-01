package analysis

import (
	"fmt"
	"time"
)

// Streak `yaml:"streak"` 连续上涨或下跌信息
type Streak struct {
	Info       string    `yaml:"info"`        // 连续上涨或下跌信息
	UpdateDate time.Time `yaml:"update-date"` // streak 信息的最后更新日期
	Trend      string    `yaml:"trend"`       // 趋势，用📈📉表示的最近 12 个交易日的涨跌状态
}

type HistoryValue struct {
	Date  time.Time // 日期
	Value float64   // 净值
}

type HistoryValueRange struct {
	Title string       // 历史区间范围
	Min   HistoryValue // 历史区间最小净值
	Max   HistoryValue // 历史区间最大净值
}

// MarkValueInHistory 生成历史值区间行
// 包括不同阶段的历史值区间
// 并根据传入的 markValue 值，在历史区间中标记出所在位置
func MarkValueInHistory(markValue float64, histories []HistoryValueRange, isRise bool) string {
	idx, exceeded := positionInHistory(markValue, histories, isRise)

	historyRow := ""
	for i, history := range histories {
		mark := ""
		if idx == i {
			if exceeded {
				if isRise {
					mark = "⏭️"
				} else {
					mark = "⏮️"
				}
			} else {
				if isRise {
					mark = "▶️"
				} else {
					mark = "◀️"
				}
			}
		}
		historyRow += fmt.Sprintf("%s：[%.4f, %.4f] %s\n", history.Title, history.Min.Value, history.Max.Value, mark)
	}
	return historyRow
}

/**
 * 查找某值在给定净值历史区间中所处的位置。
 * 返回值：历史区间数组位置索引，是否超过边界值（下跌看左边界，上涨看右边界）
 */
func positionInHistory(value float64, histories []HistoryValueRange, isRise bool) (int, bool) {
	idx, exceeded := -1, false
	for i, h := range histories {
		if value >= h.Min.Value && value <= h.Max.Value {
			idx = i
			break
		}
	}
	// 位于某区间内时，根据是否上涨，将对应侧的边界值进行向下穿透（下个历史数据区间对应侧边界值与当前区间一致时，idx 向下移动）
	if idx > -1 {
		for i := idx; i < len(histories)-1; i++ {
			if isRise {
				if histories[i].Max == histories[i+1].Max {
					idx++
				} else {
					break
				}
			} else {
				if histories[i].Min == histories[i+1].Min {
					idx++
				} else {
					break
				}
			}
		}
	}
	// 超过所有历史之区间
	if idx == -1 && len(histories) > 0 {
		idx = len(histories) - 1
		exceeded = true
	}
	return idx, exceeded
}
