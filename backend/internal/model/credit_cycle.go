package model

import "time"

// ValidMonthDay 信用卡日期字段：1–28（避开月末歧义）
func ValidMonthDay(day int) bool {
	return day >= 1 && day <= 28
}

func clampMonthDay(year int, month time.Month, day int, loc *time.Location) time.Time {
	if day < 1 {
		day = 1
	}
	if day > 28 {
		day = 28
	}
	return time.Date(year, month, day, 0, 0, 0, 0, loc)
}

// PreviousStatementDate 不晚于 from 的最近账单日
func PreviousStatementDate(statementDay int, from time.Time) time.Time {
	loc := from.Location()
	y, m, d := from.Date()
	thisMonth := clampMonthDay(y, m, statementDay, loc)
	if d >= statementDay {
		return thisMonth
	}
	prev := time.Date(y, m, 1, 0, 0, 0, 0, loc).AddDate(0, -1, 0)
	return clampMonthDay(prev.Year(), prev.Month(), statementDay, loc)
}

// BillingCycleRange 使用周期：账单日后一天 00:00 ～ 下一账单日 23:59:59。
// offset=0 本账期，-1 上账期，以此类推。
func BillingCycleRange(statementDay int, from time.Time, offset int) (start, end time.Time) {
	loc := from.Location()
	last := PreviousStatementDate(statementDay, from).AddDate(0, offset, 0)
	// AddDate 可能落到 29/30/31，再钳到账单日
	last = clampMonthDay(last.Year(), last.Month(), statementDay, loc)
	startDay := last.AddDate(0, 0, 1)
	start = time.Date(startDay.Year(), startDay.Month(), startDay.Day(), 0, 0, 0, 0, loc)
	next := last.AddDate(0, 1, 0)
	next = clampMonthDay(next.Year(), next.Month(), statementDay, loc)
	end = time.Date(next.Year(), next.Month(), next.Day(), 23, 59, 59, 0, loc)
	return start, end
}

// NextDueDate 下一个还款日（含今天）
func NextDueDate(dueDay int, from time.Time) time.Time {
	loc := from.Location()
	y, m, d := from.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, loc)
	candidate := clampMonthDay(y, m, dueDay, loc)
	if candidate.Before(today) {
		next := today.AddDate(0, 1, 0)
		candidate = clampMonthDay(next.Year(), next.Month(), dueDay, loc)
	}
	return candidate
}

// DaysUntilDue 距下次还款日的天数（0=今天）
func DaysUntilDue(dueDay int, from time.Time) int {
	due := NextDueDate(dueDay, from)
	y, m, d := from.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, from.Location())
	return int(due.Sub(today).Hours() / 24)
}
