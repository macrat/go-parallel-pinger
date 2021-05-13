// +build !github

package pinger_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/macrat/go-parallel-pinger"
)

func TestPinger_rttCalculate(t *testing.T) {
	p := pinger.NewIPv4()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("failed to start pinger: %s", err)
	}

	target, _ := net.ResolveIPAddr("ip", "8.8.8.8")
	result, err := p.Ping(ctx, target, 4, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to send ping: %s", err)
	}

	if len(result.RTTs) == 0 {
		t.Fatalf("8.8.8.8 did not response")
	}

	if result.MinRTT <= 0 {
		t.Errorf("expected MinRTT greater than 0 but got %d", result.MinRTT)
	}

	if result.MaxRTT <= 0 {
		t.Errorf("expected MaxRTT greater than 0 but got %d", result.MaxRTT)
	}

	if result.AvgRTT <= 0 {
		t.Errorf("expected AvgRTT greater than 0 but got %d", result.AvgRTT)
	}
}
