package util

import (
	"log"
	"time"

	"github.com/bingoohuang/gg/pkg/randx"
	"github.com/bingoohuang/gg/pkg/thinktime"
)

// Think sleeps for a duration by envValue.
func Think(thinkTime string) {
	if tt, _ := thinktime.ParseThinkTime(thinkTime); tt != nil {
		if sleeping := tt.Think(false); sleeping > 0 {
			log.Printf("sleeping for thinkTime %s", sleeping)
			time.Sleep(sleeping)
		}
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
