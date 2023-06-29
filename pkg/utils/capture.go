package utils

import "fmt"

func GetRate(interval float32) float32 {
	if interval == 0 {
		return 1
	}
	return 1.0 / interval
}

func GetSize(w, h int) string {
	if w == 0 || h == 0 {
		return ""
	}
	min := w
	if h < w {
		min = h
	}
	if min >= 1024 {
		ratio := 1024.0 / float64(min)
		w = int(float64(w) * ratio)
		h = int(float64(h) * ratio)
	}
	// return "1024x576"
	return fmt.Sprintf("%vx%v", w, h)
}

func GetBatchSize(interval float32, fragment int) int {
	return int(float32(fragment) / interval)
}

func GetImageSecond(idx int64, rate float32) float64 {
	if rate == 0 {
		return 0.0
	}
	// idx 始终从 1 开始, 我们希望截图时间从 0 秒开始
	return float64(idx-1) / float64(rate)
}

func GetImagePosition(idx int64, rate float32) int64 {
	if rate == 0 {
		return 0
	}
	// Position从 1 开始
	return int64(float64(idx) / float64(rate))
}

func GetSegmentStart(idx int64, seg int) float64 {
	if seg == 0 {
		return 0.0
	}
	return float64(idx) * float64(seg)
}

func GetMaxFrameLimit(max int, rate float32) int {
	if rate >= 1 {
		return int(float32(max) / rate)
	}
	return max * int(1/rate)
}
