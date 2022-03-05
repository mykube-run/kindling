package utils

import (
	"crypto/md5"
	"encoding/hex"
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
