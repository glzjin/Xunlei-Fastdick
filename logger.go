package main

import (
	"log"
	"time"
)

func logWithTimezone(format string, v ...interface{}) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		log.Printf("Failed to load location: %v", err)
		return
	}

	now := time.Now().In(location)
	timeStr := now.Format("15:04:05")
	log.Printf(timeStr+" "+format, v...)
}

func main() {
	logWithTimezone("This is a log message with timezone.")
}
