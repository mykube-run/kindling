package utils

import "strings"

// ParseCommaSeparated divides the input string seperated by comma (',') into an array
func ParseCommaSeparated(in string) []string {
	spl := strings.Split(in, ",")
	out := make([]string, 0)
	for _, v := range spl {
		vc := strings.ReplaceAll(v, " ", "")
		out = append(out, vc)
	}
	return out
}
