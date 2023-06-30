package utils

import (
	"fmt"
	"time"
)

const (
	dateFormat  = "2006-01-02"
	monthFormat = "2006-01"
)

// StartEndOfTheDay returns start and end time of t's day
// NOTE: timezone is set to UTC
func StartEndOfTheDay(t time.Time) (time.Time, time.Time) {
	t1 := t.UTC().Format(dateFormat) + "T00:00:00Z"
	t2 := t.UTC().Format(dateFormat) + "T23:59:59Z"
	s, _ := time.Parse(time.RFC3339, t1)
	e, _ := time.Parse(time.RFC3339, t2)
	return s, e
}

// StartEndOfTheMonth returns start and end time of t's month
// NOTE: timezone is set to UTC
func StartEndOfTheMonth(t time.Time) (time.Time, time.Time) {
	t1 := t.UTC().Format(monthFormat) + "-01T00:00:00Z"
	s, _ := time.Parse(time.RFC3339, t1)
	t2 := s.AddDate(0, 1, -1).UTC().Format(dateFormat) + "T23:59:59Z"
	e, _ := time.Parse(time.RFC3339, t2)
	return s, e
}

// StartEndOfTheYear returns start and end time of t's year
// NOTE: timezone is set to UTC
func StartEndOfTheYear(t time.Time) (time.Time, time.Time) {
	t1 := fmt.Sprintf("%v-01-01T00:00:00Z", t.UTC().Year())
	s, _ := time.Parse(time.RFC3339, t1)
	t2 := s.UTC().AddDate(1, 0, -1)
	t3 := fmt.Sprintf("%v-12-31T23:59:59Z", t2.UTC().Year())
	e, _ := time.Parse(time.RFC3339, t3)
	return s, e
}
