package util

import "time"

const TimestampLayout = time.RFC3339

func TimePtr(t time.Time) *time.Time {
	return &t
}

func HumanizeAge(t *time.Time) string {
	if t == nil {
		return "never"
	}
	d := time.Since(*t)
	if d <= 0 {
		return "just now"
	}
	d = d.Truncate(time.Second)
	if d < time.Second {
		return "just now"
	}
	return d.String() + " ago"
}
