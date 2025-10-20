package analysis

import (
	"fmt"
	"time"
)

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
func MarkValueInHistory(markValue float64, histories []HistoryValueRange) string {
	idx, leftOrRight, exceeded := positionInHistory(markValue, histories)

	historyRow := ""
	for i, history := range histories {
		mark := ""
		if idx == i {
			if exceeded {
				if leftOrRight < 0 {
					mark = "⏮️"
				} else {
					mark = "⏭️"
				}
			} else {
				if leftOrRight < 0 {
					mark = "◀️"
				} else {
					mark = "▶️"
				}
			}
		}
		historyRow += fmt.Sprintf("%s：[%.4f, %.4f] %s\n", history.Title, history.Min.Value, history.Max.Value, mark)
	}
	return historyRow
}

/**
 * 查找某值在给定净值历史区间中所处的位置。
 * 返回值：历史区间数组位置索引，在所属区间偏左还是偏右（小于 0 偏左，大于 0 偏右），是否超过边界值
 */
func positionInHistory(value float64, histories []HistoryValueRange) (int, int, bool) {
	idx, leftOrRight, exceeded := -1, 0, false
	for i, h := range histories {
		if value >= h.Min.Value && value <= h.Max.Value {
			idx = i
			break
		}
	}
	// 位于某区间内时，判断偏左还是偏右，并对对应侧的边界值进行向下穿透（下个历史数据区间对应侧边界值与当前区间一致时，idx 向下移动）
	if idx > -1 {
		if value < (histories[idx].Min.Value+histories[idx].Max.Value)/2 {
			leftOrRight = -1
		} else {
			leftOrRight = 1
		}
		for i := idx; i < len(histories)-1; i++ {
			if leftOrRight > 0 {
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
		if value < (histories[idx].Min.Value+histories[idx].Max.Value)/2 {
			leftOrRight = -1
		} else {
			leftOrRight = 1
		}
	}
	// 超过所有历史之区间
	if idx == -1 && len(histories) > 0 {
		idx = len(histories) - 1
		exceeded = true
		if value < histories[idx].Min.Value {
			leftOrRight = -1
		}
		if value > histories[idx].Max.Value {
			leftOrRight = 1
		}
	}
	return idx, leftOrRight, exceeded
}
