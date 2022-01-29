package util

import (
	"log"
	"time"

	"github.com/bingoohuang/gg/pkg/thinktime"
)

func Sleep(env string, defaultSleep time.Duration) {
	sleeping := defaultSleep
	if tt, _ := thinktime.ParseThinkTime(env); tt != nil {
		sleeping = tt.Think(false)

	}

	if sleeping > 0 {
		log.Printf("sleeping for %s", sleeping)
		time.Sleep(sleeping)
	}
}
