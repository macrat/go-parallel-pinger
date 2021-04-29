go-parallel-ping
================

[![Documents](https://pkg.go.dev/badge/github.com/macrat/go-parallel-ping)](https://pkg.go.dev/github.com/macrat/go-parallel-ping)
![Supports Linux, Darwin, and Windows](https://img.shields.io/badge/platform-Linux%20%7C%20Darwin%20%7C%20Windows-lightgrey)
[![GitHub Actions CI Status](https://github.com/macrat/go-parallel-ping/actions/workflows/test.yml/badge.svg)](https://github.com/macrat/go-parallel-ping/actions/workflows/test.yml)
[![Codecov Test Coverage](https://img.shields.io/codecov/c/gh/macrat/go-parallel-ping)](https://app.codecov.io/gh/macrat/go-parallel-ping/)

A easy and thread-safe way to send ping in Go.

``` go
package main

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/macrat/go-parallel-pinger"
)

func main() {
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
```
