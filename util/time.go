package util

import (
	"log"
	"time"

	"github.com/bingoohuang/gg/pkg/thinktime"
)

func EnvSleep() {
	if tt, _ := thinktime.ParseThinkTime(Env("THINK_TIME")); tt != nil {
		sleeping := tt.Think(false)
		log.Printf("sleeping for %s", sleeping)
		time.Sleep(sleeping)
	}
}
