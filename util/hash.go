package util

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func Sha256Sum(b []byte) string {
	h := sha256.New()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

func Sha256SumJson(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return Sha256Sum(b), nil
}

func MustSha256SumJson(v any) string {
	h, err := Sha256SumJson(v)
	if err != nil {
		panic(err)
	}
	return h
}
