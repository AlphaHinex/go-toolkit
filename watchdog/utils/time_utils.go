package utils

import "time"

func GetNow() time.Time {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		// Windows 环境使用 time.LoadLocation 报 panic: time: missing Location in call to Time.In
		loc = time.FixedZone("CST", 8*3600)
	}
	// 获取当前时间并转换为东八区时间
	now := time.Now().In(loc)
	return now
}

func IsSameDay(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() && t1.Month() == t2.Month() && t1.Day() == t2.Day()
}

func InOpeningHours() bool {
	now := GetNow()
	hour := now.Hour()
	minute := now.Minute()

	// 上午9:30-11:30
	if (hour == 9 && minute >= 30) || (hour > 9 && hour < 11) || (hour == 11 && minute <= 30) {
		return true
	}
	// 下午13:00-15:00
	if (hour == 13) || (hour > 13 && hour < 15) || (hour == 15 && minute == 0) {
		return true
	}
	return false
}

func InBreakingTime() bool {
	now := GetNow()
	hour := now.Hour()
	minute := now.Minute()
	return (hour == 11 && minute >= 30) || (hour == 12)
}

func LastMonth() time.Time {
	return GetNow().AddDate(0, -1, 0)
}

func LastQuarter() time.Time {
	return GetNow().AddDate(0, -3, 0)
}

func LastSixMonths() time.Time {
	return GetNow().AddDate(0, -6, 0)
}

func LastYear() time.Time {
	return GetNow().AddDate(-1, 0, 0)
}

func LastThreeYears() time.Time {
	return GetNow().AddDate(-3, 0, 0)
}

func LastFiveYears() time.Time {
	return GetNow().AddDate(-5, 0, 0)
}

func EarliestTime() time.Time {
	return time.Date(1678, 1, 1, 0, 0, 0, 0, time.UTC)
}
