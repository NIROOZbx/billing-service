package helpers

import "time"

func UnixToTime(ts int64) time.Time {
	return time.Unix(ts, 0).UTC()
}