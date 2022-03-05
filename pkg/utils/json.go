package utils

import "encoding/json"

func IndentedJSON(v interface{}) string {
	byt, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "<Invalid JSON>"
	}
	return string(byt)
}

func CompactJSON(v interface{}) string {
	byt, err := json.Marshal(v)
	if err != nil {
		return "<Invalid JSON>"
	}
	return string(byt)
}
