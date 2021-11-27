package random

import (
	"math/rand"
	"time"
)

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func GetRandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[seededRand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}
