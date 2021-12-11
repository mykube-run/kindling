package utils

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
)

func Md5(byt []byte) string {
	h := md5.New()
	h.Write(byt)
	return hex.EncodeToString(h.Sum(nil))
}

func If(cond bool, tv, fv interface{}) interface{} {
	if cond {
		return tv
	}
	return fv
}

func ParseCommaSeparated(in string) []string {
	spl := strings.Split(in, ",")
	out := make([]string, 0)
	for _, v := range spl {
		vc := strings.ReplaceAll(v, " ", "")
		out = append(out, vc)
	}
	return out
}
