package utils

import "strings"

func ParseCommaSeparated(in string) []string {
	spl := strings.Split(in, ",")
	out := make([]string, 0)
	for _, v := range spl {
		vc := strings.ReplaceAll(v, " ", "")
		out = append(out, vc)
	}
	return out
}
