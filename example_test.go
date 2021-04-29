package pinger_test

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"github.com/macrat/go-parallel-pinger"
)

func Example() {
	p := pinger.NewIPv4()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		log.Fatalf("failed to start pinger: %s", err)
	}

	target, _ := net.ResolveIPAddr("ip", "127.0.0.1")
	result, err := p.Ping(ctx, target, 4, 100*time.Millisecond)
	if err != nil {
		log.Fatalf("failed to send ping: %s", err)
	}

	log.Printf("sent %d packets and received %d packets", result.Sent, result.Recv)
	log.Printf("RTT: min=%s / avg=%s / max=%s", result.MinRTT, result.AvgRTT, result.MaxRTT)
}

func Example_parallel() {
	p := pinger.NewIPv4()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		log.Fatalf("failed to start pinger: %s", err)
	}

	wg := &sync.WaitGroup{}

	for _, target := range []string{"127.0.0.1", "127.0.0.2", "127.0.0.3", "127.0.0.4", "127.0.0.5"} {
		wg.Add(1)

		go func(target string) {
			t, _ := net.ResolveIPAddr("ip", target)
			result, err := p.Ping(ctx, t, 4, 100*time.Millisecond)

			if err != nil {
				log.Printf("%s: failed to send ping: %s", target, err)
			} else {
				log.Printf("%s: min=%s / avg=%s / max=%s", target, result.MinRTT, result.AvgRTT, result.MaxRTT)
			}

			wg.Done()
		}(target)
	}

	wg.Wait()
}
