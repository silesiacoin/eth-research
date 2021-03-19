package context_test

import (
	"context"
	"time"
)

func ContextWithCancel() {
	gen := func(ctx context.Context, cancel context.CancelFunc) <- chan int {
		dst := make(chan int)
		n := 1
		go func() {
			for {
				select {
				case <- ctx.Done():
					println("done!")
					return
				case dst <- n:
					n++
					println("incrementing: ", n)
				}
			}
		}()
		return dst
	}


	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for n := range gen(ctx, cancel) {
		if n == 500 {
			break
		}
	}
}

func sleepAndTalk(ctx context.Context, d time.Duration, msg string) {
	time.Sleep(d)
	println(msg)
}
