package pinger_test

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/macrat/go-parallel-pinger"
)

func TestPinger_Ping(t *testing.T) {
	tests := []struct {
		Name   string
		Pinger *pinger.Pinger
		Target string
	}{
		{"IPv4", pinger.NewIPv4(), "127.0.0.1"},
		{"IPv6", pinger.NewIPv6(), "::1"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			if err := tt.Pinger.Start(ctx); err != nil {
				t.Fatalf("failed to start pinger: %s", err)
			}

			target, _ := net.ResolveIPAddr("ip", tt.Target)
			result, err := tt.Pinger.Ping(ctx, target, 4, 100*time.Millisecond)
			if err != nil {
				t.Fatalf("failed to send ping: %s", err)
			}

			if result.Sent != 4 {
				t.Errorf("expected send 4 packets but sent only %d packets", result.Sent)
			}
			if result.Recv != 4 {
				t.Errorf("expected receive 4 packets but received only %d packets", result.Recv)
			}
			if result.Loss != 0 {
				t.Errorf("expected loss 0 packet but loss %d packets", result.Loss)
			}
			if len(result.RTTs) != 4 {
				t.Errorf("expected 4 RTT records but %d RTT records found: %v", len(result.RTTs), result.RTTs)
			}
		})
	}
}

func TestPinger_flooding(t *testing.T) {
	tests := []struct {
		Name   string
		Target func(*testing.T, int) string
	}{
		{"same_host", func(t *testing.T, i int) string {
			return "127.0.0.1"
		}},
		{"many_hosts", func(t *testing.T, i int) string {
			return fmt.Sprintf("127.0.0.%d", i)
		}},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			if runtime.GOOS == "darwin" {
				t.Skip("darwin is only supported ping to 127.0.0.1")
			}

			p := pinger.NewIPv4()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := p.Start(ctx); err != nil {
				t.Fatalf("failed to start pinger: %s", err)
			}

			wg := &sync.WaitGroup{}

			for i := 1; i <= 254; i++ {
				wg.Add(1)

				go func(i int) {
					target := tt.Target(t, i)
					addr, _ := net.ResolveIPAddr("ip", target)
					result, err := p.Ping(ctx, addr, 8, 100*time.Millisecond)

					if err != nil {
						t.Errorf("%s: failed to send ping: %s", target, err)
					}

					if result.Loss != 0 {
						t.Errorf("%s: lose %d packets", target, result.Loss)
					}

					wg.Done()
				}(i)
			}

			wg.Wait()
		})
	}
}

func TestPinger_timeout(t *testing.T) {
	t.Parallel()

	p := pinger.NewIPv4()

	ctxLong, cancelLong := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancelLong()

	if err := p.Start(ctxLong); err != nil {
		t.Fatalf("failed to start pinger: %s", err)
	}

	ctxShort, cancelShort := context.WithTimeout(ctxLong, 150*time.Millisecond)
	defer cancelShort()

	target, _ := net.ResolveIPAddr("ip", "127.0.0.1")
	result, err := p.Ping(ctxShort, target, 4, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to send ping: %s", err)
	}

	if result.Sent != 2 {
		t.Errorf("expected send 2 packets but sent %d packets", result.Sent)
	}
	if result.Recv > 2 {
		t.Errorf("expected receive maximum 2 packets but received %d packets", result.Recv)
	}
	if len(result.RTTs) > 2 {
		t.Errorf("expected maximum 2 RTT records but %d RTT records found: %v", len(result.RTTs), result.RTTs)
	}
}

func TestPinger_changePrevileged(t *testing.T) {
	p := pinger.NewIPv4()

	if p.Privileged() != pinger.DEFAULT_PRIVILEGED {
		t.Errorf("unexpected default privileged: %t != %t", p.Privileged(), pinger.DEFAULT_PRIVILEGED)
	}

	if err := p.SetPrivileged(!pinger.DEFAULT_PRIVILEGED); err != nil {
		t.Errorf("failed to set privileged mode: %s", err)
	}
	if p.Privileged() != !pinger.DEFAULT_PRIVILEGED {
		t.Errorf("unexpected privileged mode: %t != %t", p.Privileged(), !pinger.DEFAULT_PRIVILEGED)
	}

	if err := p.SetPrivileged(pinger.DEFAULT_PRIVILEGED); err != nil {
		t.Errorf("failed to set privileged mode: %s", err)
	}
	if p.Privileged() != pinger.DEFAULT_PRIVILEGED {
		t.Errorf("unexpected privileged mode: %t != %t", p.Privileged(), pinger.DEFAULT_PRIVILEGED)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("failed to start pinger: %s", err)
	}
	if err := p.SetPrivileged(!pinger.DEFAULT_PRIVILEGED); err == nil {
		t.Errorf("expected failure to set privileged because pinger is started, but succeed")
	} else if err != pinger.ErrAlreadyStarted {
		t.Errorf("expected failure to set privileged because pinger is started, but got unexpected another error: %s", err)
	}
}

func TestPinger_alreadyStarted(t *testing.T) {
	p := pinger.NewIPv4()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("failed to start pinger: %s", err)
	}

	if err := p.Start(ctx); err == nil {
		t.Errorf("expected failure to start pinger because pinger is already started, but succeed")
	} else if err != pinger.ErrAlreadyStarted {
		t.Errorf("expected failure to start pinger because pinger is already started, but got unexpected another error: %s", err)
	}
}

func TestPinger_notStarted(t *testing.T) {
	p := pinger.NewIPv4()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	target, _ := net.ResolveIPAddr("ip", "127.0.0.1")
	if _, err := p.Ping(ctx, target, 4, 100*time.Millisecond); err == nil {
		t.Errorf("expected failure to start pinger because pinger is not yet started, but succeed")
	} else if err != pinger.ErrNotStarted {
		t.Errorf("expected failure to start pinger because pinger is not yet started, but got unexpected another error: %s", err)
	}
}

func TestPinger_cancelContext(t *testing.T) {
	p := pinger.NewIPv4()

	before := runtime.NumGoroutine()

	for i := 0; i < 1000; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		p.Start(ctx)
		cancel()
	}

	time.Sleep(100 * time.Millisecond) // wait for stop handler

	after := runtime.NumGoroutine()

	if before < after {
		t.Errorf("goroutines should not increase but increased: %d -> %d", before, after)
	}
}

func TestPinger_Stop(t *testing.T) {
	p := pinger.NewIPv4()

	before := runtime.NumGoroutine()

	for i := 0; i < 1000; i++ {
		p.Start(context.Background())
		if err := p.Stop(); err != nil {
			t.Fatalf("failed to stop Pinger: %s", err)
		}
	}

	after := runtime.NumGoroutine()

	if before < after {
		t.Errorf("goroutines should not increase but increased: %d -> %d", before, after)
	}
}

func TestPinger_Stop_notStarted(t *testing.T) {
	p := pinger.NewIPv4()

	err := p.Stop()
	if err != pinger.ErrNotStarted {
		t.Errorf("expected ErrNotStarted but got %s", err)
	}
}

func BenchmarkPinger_Ping_v4(b *testing.B) {
	target, _ := net.ResolveIPAddr("ip", "127.0.0.1")

	p := pinger.NewIPv4()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		b.Fatalf("failed to start pinger: %s", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Ping(ctx, target, 10, 1*time.Nanosecond)
	}
}

func BenchmarkPinger_Ping_v6(b *testing.B) {
	target, _ := net.ResolveIPAddr("ip", "::1")

	p := pinger.NewIPv6()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		b.Fatalf("failed to start pinger: %s", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Ping(ctx, target, 10, 1*time.Nanosecond)
	}
}
