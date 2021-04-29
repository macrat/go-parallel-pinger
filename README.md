go-parallel-pinger
==================

[![Documents](https://pkg.go.dev/badge/github.com/macrat/go-parallel-pinger)](https://pkg.go.dev/github.com/macrat/go-parallel-pinger)
![Supports Linux, Darwin, and Windows](https://img.shields.io/badge/platform-Linux%20%7C%20Darwin%20%7C%20Windows-lightgrey)
[![GitHub Actions CI Status](https://github.com/macrat/go-parallel-pinger/actions/workflows/test.yml/badge.svg)](https://github.com/macrat/go-parallel-pinger/actions/workflows/test.yml)
[![Codecov Test Coverage](https://img.shields.io/codecov/c/gh/macrat/go-parallel-pinger)](https://app.codecov.io/gh/macrat/go-parallel-pinger/)

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

func Example() {
	target, _ := net.ResolveIPAddr("ip", "127.0.0.1")

	// 1. make Pinger for IPv4 (or, you can use NewIPv6)
	p := pinger.NewIPv4()

	// 2. make context for handle timeout or cancel
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 3. start Pinger for send/receive ICMP packet before send ping
	if err := p.Start(ctx); err != nil {
		log.Fatalf("failed to start pinger: %s", err)
	}

	// 4. send ping for the target, and wait until receive all reply or context canceled
	result, err := p.Ping(ctx, target, 4, 100*time.Millisecond)
	if err != nil {
		log.Fatalf("failed to send ping: %s", err)
	}

	// 5. check the result
	log.Printf("sent %d packets and received %d packets", result.Sent, result.Recv)
	log.Printf("RTT: min=%s / avg=%s / max=%s", result.MinRTT, result.AvgRTT, result.MaxRTT)
```
