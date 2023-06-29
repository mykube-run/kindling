package utils

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"
)

// MapKeys extracts all keys from given map
func MapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k, _ := range m {
		kc := k
		keys = append(keys, kc)
	}
	return keys
}

func FilePath2Index(in string) (int64, error) {
	fn := strings.ReplaceAll(path.Base(in), path.Ext(in), "")
	out, err := strconv.ParseInt(fn, 10, 0)
	return out, err
}

func FilePath2Suffix(in string) string {
	return strings.TrimLeft(path.Ext(in), ".")
}

func KVStringsToMap(in ...string) map[string]string {
	out := make(map[string]string)
	if len(in) == 0 {
		return out
	}

	if len(in)%2 == 0 {
		for i := 0; i < len(in)/2; i++ {
			k := in[i*2]
			v := in[i*2+1]
			out[k] = v
		}
	}

	if (len(in)-1)%2 == 0 {
		for i := 0; i < (len(in)-1)/2; i++ {
			k := in[i*2]
			v := in[i*2+1]
			out[k] = v
		}
		out[in[len(in)-1]] = ""
	}
	return out
}

// Seconds2ClockDuration converts seconds to clock like duration, e.g. 4003(s) -> 01:06:43
func Seconds2ClockDuration(v int64) string {
	d := time.Second * time.Duration(v)
	h := int64(d.Seconds() / (60 * 60))
	m := (int64(d.Seconds()) - h*60*60) / 60
	s := int64(d.Seconds()) - h*60*60 - m*60
	if h <= 99 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d:%02d", h, m, s)
}
