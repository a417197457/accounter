package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// parseDateQuery 解析 query 中的日期/时间。
// 支持 YYYY-MM-DD、YYYY-MM-DD HH:MM:SS、RFC3339。
// endKey 为 true 时，纯日期会补到当天 23:59:59。
func parseDateQuery(s string, asEnd bool) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	formats := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}
	var parsed time.Time
	var err error
	for _, f := range formats {
		parsed, err = time.ParseInLocation(f, s, time.Local)
		if err == nil {
			if asEnd && len(s) == 10 {
				parsed = parsed.Add(24*time.Hour - time.Second)
			}
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("时间格式错误: %s", s)
}

func queryDate(c *gin.Context, key string, asEnd bool) (*time.Time, error) {
	return parseDateQuery(c.Query(key), asEnd)
}
