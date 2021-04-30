package main

import (
	"context"
	"log"
	"net"
	"runtime"
	"time"
	"fmt"

	"github.com/macrat/go-parallel-pinger"
)

func main() {
	target, _ := net.ResolveIPAddr("ip", "127.0.0.1")

	p := pinger.NewIPv4()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := p.Start(ctx); err != nil {
		log.Fatalf("failed to start pinger: %s", err)
	}

	nextReport := time.Now()
	var mem runtime.MemStats

	for i := 0; ; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		if _, err := p.Ping(ctx, target, 2, 5*time.Millisecond); err != nil {
			log.Printf("failed to send ping: %s", err)
		}
		cancel()

		if time.Now().After(nextReport) {
			nextReport = time.Now().Add(5 * time.Second)
			runtime.ReadMemStats(&mem)
			fmt.Printf("%s\t%d\t%d\t%.3f\n", time.Now().Format(time.RFC3339), i + 1, runtime.NumGoroutine(), float64(mem.Alloc)/1024/1024)
		}
	}
}
