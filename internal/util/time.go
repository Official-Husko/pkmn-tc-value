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
	d := time.Since(*t).Round(time.Minute)
	if d < time.Minute {
		return "just now"
	}
	return d.String() + " ago"
}
