package main

import (
	"time"
)

func checkUpgradeTime(timestamp int64) bool {
	location, _ := time.LoadLocation("Asia/Shanghai")
	currentTime := time.Now().In(location)
	requestTime := time.Unix(timestamp, 0).In(location)

	// 添加适当的时间比较逻辑
	return currentTime.After(requestTime)
}
