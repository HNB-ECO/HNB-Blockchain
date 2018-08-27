package util

import (
	"time"
	"strconv"
)

func GetCurrentMillisecond() uint64 {
	return uint64(time.Now().UnixNano() / 1000000)
}

func GetCurrentTimestampString() string {
	return strconv.FormatUint(GetCurrentMillisecond(), 10)
}

func GetCurrentUnixNano() uint64 {
	return uint64(time.Now().UnixNano())
}

