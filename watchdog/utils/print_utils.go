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
// 添加涨跌符号
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
