// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.21

package quic

import (
	"context"
	"fmt"
	"io"
	"math"
	"testing"
)

// BenchmarkThroughput is based on the crypto/tls benchmark of the same name.
func BenchmarkThroughput(b *testing.B) {
	for size := 1; size <= 64; size <<= 1 {
		name := fmt.Sprintf("%dMiB", size)
		b.Run(name, func(b *testing.B) {
			throughput(b, int64(size<<20))
		})
	}
}

func throughput(b *testing.B, totalBytes int64) {
	// Same buffer size as crypto/tls's BenchmarkThroughput, for consistency.
	const bufsize = 32 << 10

	cli, srv := newLocalConnPair(b, &Config{}, &Config{})

	go func() {
		buf := make([]byte, bufsize)
		for i := 0; i < b.N; i++ {
			sconn, err := srv.AcceptStream(context.Background())
			if err != nil {
				panic(fmt.Errorf("AcceptStream: %v", err))
			}
			if _, err := io.CopyBuffer(sconn, sconn, buf); err != nil {
				panic(fmt.Errorf("CopyBuffer: %v", err))
			}
			sconn.Close()
		}
	}()

	b.SetBytes(totalBytes)
	buf := make([]byte, bufsize)
	chunks := int(math.Ceil(float64(totalBytes) / float64(len(buf))))
	for i := 0; i < b.N; i++ {
		cconn, err := cli.NewStream(context.Background())
		if err != nil {
			b.Fatalf("NewStream: %v", err)
		}
		closec := make(chan struct{})
		go func() {
			defer close(closec)
			buf := make([]byte, bufsize)
			if _, err := io.CopyBuffer(io.Discard, cconn, buf); err != nil {
				panic(fmt.Errorf("Discard: %v", err))
			}
		}()
		for j := 0; j < chunks; j++ {
			_, err := cconn.Write(buf)
			if err != nil {
				b.Fatalf("Write: %v", err)
			}
		}
		cconn.CloseWrite()
		<-closec
		cconn.Close()
	}
}

func BenchmarkStreamCreation(b *testing.B) {
	cli, srv := newLocalConnPair(b, &Config{}, &Config{})

	go func() {
		for i := 0; i < b.N; i++ {
			sconn, err := srv.AcceptStream(context.Background())
			if err != nil {
				panic(fmt.Errorf("AcceptStream: %v", err))
			}
			sconn.Close()
		}
	}()

	buf := make([]byte, 1)
	for i := 0; i < b.N; i++ {
		cconn, err := cli.NewStream(context.Background())
		if err != nil {
			b.Fatalf("NewStream: %v", err)
		}
		cconn.Write(buf)
		cconn.Flush()
		cconn.Read(buf)
		cconn.Close()
	}
}
