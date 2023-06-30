package utils

import (
	"fmt"
	"testing"
	"time"
)

func TestStartEndOfTime(t *testing.T) {
	var (
		t1, _ = time.Parse(time.RFC3339, "2022-02-01T19:34:56Z")
		t2, _ = time.Parse(time.RFC3339, "2022-02-01T19:34:56+08:00")
		t3, _ = time.Parse(time.RFC3339, "2022-02-28T19:34:56+08:00")

		t4, _ = time.Parse(time.RFC3339, "2020-02-29T19:34:56Z")
		t5, _ = time.Parse(time.RFC3339, "2020-02-29T19:34:56+08:00")
		t6, _ = time.Parse(time.RFC3339, "2020-02-20T19:34:56+08:00")

		t7, _ = time.Parse(time.RFC3339, "2022-12-31T19:34:56Z")
		t8, _ = time.Parse(time.RFC3339, "2022-12-31T01:34:56+08:00")
	)
	times := []time.Time{t1, t2, t3, t4, t5, t6, t7, t8}
	for _, t := range times {
		fmt.Println(t, t.Location().String())
		fmt.Println(StartEndOfTheDay(t))
		fmt.Println(StartEndOfTheMonth(t))
		fmt.Println(StartEndOfTheYear(t))
		fmt.Println("------------")
	}
}
