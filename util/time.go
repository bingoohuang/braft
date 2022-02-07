package util

import (
	"log"
	"time"

	"github.com/bingoohuang/gg/pkg/randx"
	"github.com/bingoohuang/gg/pkg/thinktime"
)

// Sleep sleeps for a duration by envValue.
func Sleep(envValue string, defaultSleep time.Duration) {
	sleeping := defaultSleep
	if tt, _ := thinktime.ParseThinkTime(envValue); tt != nil {
		sleeping = tt.Think(false)
	}

	if sleeping > 0 {
		log.Printf("sleeping for %s", sleeping)
		time.Sleep(sleeping)
	}
}

// RandSleep sleeps for a random duration.
func RandSleep(min, max time.Duration, printLog bool) {
	sleeping := time.Duration(randx.IntBetween(int(min), int(max)))
	if printLog {
		log.Printf("sleeping for %s", sleeping)
	}
	time.Sleep(sleeping)
}
