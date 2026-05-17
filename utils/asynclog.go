package utils

import (
	"bufio"
	"io"
	"os"
	"sync/atomic"
	"time"
)

// AsyncWriter is an io.Writer that hands bytes to a background goroutine so
// request handlers never block on stdout writes. Drops on overflow rather than
// applying backpressure to the request path (which is what we're trying to
// avoid in the first place).
type AsyncWriter struct {
	ch      chan []byte
	dropped atomic.Uint64
}

// NewAsyncWriter returns a writer that forwards to `out`. `bufferLines` is the
// channel depth (per-line slots). Flushes the underlying buffered writer every
// `flushInterval` so log lines don't sit around if traffic goes quiet.
func NewAsyncWriter(out io.Writer, bufferLines int, flushInterval time.Duration) *AsyncWriter {
	w := &AsyncWriter{ch: make(chan []byte, bufferLines)}
	go w.run(out, flushInterval)
	return w
}

func (w *AsyncWriter) Write(p []byte) (int, error) {
	// Copy because gin reuses the slice after Write returns.
	buf := make([]byte, len(p))
	copy(buf, p)
	select {
	case w.ch <- buf:
	default:
		w.dropped.Add(1)
	}
	return len(p), nil
}

func (w *AsyncWriter) Dropped() uint64 { return w.dropped.Load() }

func (w *AsyncWriter) run(out io.Writer, flushInterval time.Duration) {
	bw := bufio.NewWriterSize(out, 64*1024)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case b, ok := <-w.ch:
			if !ok {
				_ = bw.Flush()
				return
			}
			_, _ = bw.Write(b)
		case <-ticker.C:
			_ = bw.Flush()
		}
	}
}

// DefaultAsyncStdout returns an AsyncWriter sized for ~10k req/s burst into
// stdout with a 50ms flush cadence.
func DefaultAsyncStdout() *AsyncWriter {
	return NewAsyncWriter(os.Stdout, 4096, 50*time.Millisecond)
}
