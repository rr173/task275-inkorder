package store

import (
	"fmt"
	"time"
)

// scanTime 把 SQLite 返回的时间文本解析为 time.Time。
// modernc.org/sqlite 以字符串存储/返回时间列，需显式解析。
func scanTime(v interface{}) (time.Time, error) {
	switch s := v.(type) {
	case string:
		return time.Parse(time.RFC3339Nano, s)
	case []byte:
		return time.Parse(time.RFC3339Nano, string(s))
	case time.Time:
		return s, nil
	case nil:
		return time.Time{}, nil
	}
	return time.Time{}, fmt.Errorf("unsupported time type %T", v)
}

// scanTimeText 供 Scan 时间列时取回原始字符串。
type timeText string

func (t *timeText) Scan(v interface{}) error {
	switch s := v.(type) {
	case string:
		*t = timeText(s)
	case []byte:
		*t = timeText(s)
	case nil:
		*t = ""
	default:
		return fmt.Errorf("unsupported time type %T", v)
	}
	return nil
}

func (t timeText) time() time.Time {
	return parseTime(string(t))
}
