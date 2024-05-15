package util

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
)

// Go 开始一个协程
func Go(wg *sync.WaitGroup, f func()) {
	if wg != nil {
		wg.Add(1)
	}

	go func() {
		if wg != nil {
			defer wg.Done()
		}

		f()
	}()
}

// GoChan 开始一个协程，从 ch 读取数据，调用 f 进行处理
func GoChan[T any](ctx context.Context, wg *sync.WaitGroup, ch <-chan T, f func(elem T) error) {
	if wg != nil {
		wg.Add(1)
	}

	go func() {
		if wg != nil {
			defer wg.Done()
		}

		for {
			select {
			case <-ctx.Done():
				return
			case elem, ok := <-ch:
				if !ok { // closed
					return
				}
				if err := f(elem); err != nil {
					if errors.Is(err, io.EOF) {
						return
					}
					log.Printf("E! GoFor invoke fn failed: %v", err)
				}
			}
		}
	}()
}
