//go:build !windows

package util

import (
	"log"
	"syscall"
)

func Rss() uint64 {
	var mem syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &mem); err != nil {
		log.Printf("E! failed to call syscall.Getrusage, error: %v", err)
	}
	return uint64(mem.Maxrss)
}
