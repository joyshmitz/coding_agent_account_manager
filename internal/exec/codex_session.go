package exec

import (
	"bytes"
	"io"
	"regexp"
	"sync"
)

var codexResumeSessionIDRe = regexp.MustCompile(`\bcodex\s+resume\s+([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})\b`)

// maxBufferSize is the maximum buffer size before forcing a flush (64KB).
// This prevents unbounded memory growth when output contains no newlines.
const maxBufferSize = 64 * 1024

type codexSessionCapture struct {
	mu sync.Mutex
	id string
}

func (c *codexSessionCapture) ObserveLine(line string) {
	m := codexResumeSessionIDRe.FindStringSubmatch(line)
	if len(m) != 2 {
		return
	}
	c.mu.Lock()
	c.id = m[1]
	c.mu.Unlock()
}

func (c *codexSessionCapture) ID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.id
}

type lineObserverWriter struct {
	dst    io.Writer
	onLine func(line string)

	mu  sync.Mutex
	buf []byte
}

func newLineObserverWriter(dst io.Writer, onLine func(line string)) *lineObserverWriter {
	return &lineObserverWriter{dst: dst, onLine: onLine}
}

func (w *lineObserverWriter) Write(p []byte) (int, error) {
	n, err := w.dst.Write(p)
	if n <= 0 {
		return n, err
	}

	w.mu.Lock()
	w.buf = append(w.buf, p[:n]...)

	for {
		nl := bytes.IndexByte(w.buf, '\n')
		if nl < 0 {
			break
		}

		lineBytes := w.buf[:nl]
		if len(lineBytes) > 0 && lineBytes[len(lineBytes)-1] == '\r' {
			lineBytes = lineBytes[:len(lineBytes)-1]
		}
		w.onLine(string(lineBytes))

		w.buf = w.buf[nl+1:]
	}

	// Enforce buffer limit to prevent OOM on long lines without newlines
	if len(w.buf) > maxBufferSize {
		// Process oversized buffer as a partial line
		lineBytes := w.buf
		if len(lineBytes) > 0 && lineBytes[len(lineBytes)-1] == '\r' {
			lineBytes = lineBytes[:len(lineBytes)-1]
		}
		w.onLine(string(lineBytes))
		w.buf = nil
	}

	// Compact buffer if it has grown large but contains little data
	// (capacity > 4KB and usage < 25%)
	if cap(w.buf) > 4096 && len(w.buf) < cap(w.buf)/4 {
		newBuf := make([]byte, len(w.buf))
		copy(newBuf, w.buf)
		w.buf = newBuf
	}

	w.mu.Unlock()

	return n, err
}

func (w *lineObserverWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) == 0 {
		return
	}

	lineBytes := w.buf
	if len(lineBytes) > 0 && lineBytes[len(lineBytes)-1] == '\r' {
		lineBytes = lineBytes[:len(lineBytes)-1]
	}
	w.onLine(string(lineBytes))
	w.buf = nil
}
