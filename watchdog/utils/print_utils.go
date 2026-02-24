package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func TurnToEmojiNumber(num int) string {
	str := strconv.Itoa(num)
	str = strings.ReplaceAll(str, "0", "0️⃣")
	str = strings.ReplaceAll(str, "1", "1️⃣")
	str = strings.ReplaceAll(str, "2", "2️⃣")
	str = strings.ReplaceAll(str, "3", "3️⃣")
	str = strings.ReplaceAll(str, "4", "4️⃣")
	str = strings.ReplaceAll(str, "5", "5️⃣")
	str = strings.ReplaceAll(str, "6", "6️⃣")
	str = strings.ReplaceAll(str, "7", "7️⃣")
	str = strings.ReplaceAll(str, "8", "8️⃣")
	str = strings.ReplaceAll(str, "9", "9️⃣")
	return str
}

// UpOrDown
// 在输入的字符串前面，添加涨跌符号。输入字符串需表示数值，数值为正添加 "🔺"，数值为负添加 "▼"
func UpOrDown(value string) string {
	v, _ := strconv.ParseFloat(value, 64)
	if v > 0 {
		return fmt.Sprintf("🔺%.2f%%", v)
	}
	if v == 0 {
		return fmt.Sprintf(" %.2f%%", v)
	}
	return fmt.Sprintf("▼ %.2f%%", v)
}

// UpOrDownEmoji
// 根据当前值与之前值的比较，返回对应的涨跌表情符号。当前值大于之前值返回 "📈"，当前值小于之前值返回 "📉"，否则返回空字符串。
func UpOrDownEmoji(before, current float64) string {
	upOrDownMark := ""
	if current > before {
		upOrDownMark = "📈"
	} else if current < before {
		upOrDownMark = "📉"
	}
	return upOrDownMark
}
