package proto

import (
	"bufio"
	"io"
	"sync"
	"time"
)

// Writer writes words to a RouterOS device.
type Writer interface {
	BeginSentence()
	WriteWord(word string)
	EndSentence() error
}

type WriterDeadline interface {
	io.Writer
	SetWriteDeadline(t time.Time) error
}

type writer struct {
	bufferedWriter   *bufio.Writer
	setWriteDeadline func(time.Time) error
	timeout          time.Duration
	err              error
	sync.Mutex
}

// NewWriter returns a new Writer to write to w.
func NewWriter(w WriterDeadline, timeout time.Duration) Writer {
	return &writer{
		bufferedWriter:   bufio.NewWriter(w),
		setWriteDeadline: w.SetWriteDeadline,
		timeout:          timeout,
	}
}

// BeginSentence prepares w for writing a sentence.
func (w *writer) BeginSentence() {
	w.Lock()
}

// EndSentence writes the end-of-sentence marker (an empty word).
// It returns the first error that occurred on calls to methods on w.
func (w *writer) EndSentence() error {
	defer w.Unlock()
	w.WriteWord("")
	w.flush()
	return w.err
}

// WriteWord writes one word.
func (w *writer) WriteWord(word string) {
	b := []byte(word)
	w.write(encodeLength(len(b)))
	w.write(b)
}

func (w *writer) setDeadline() {
	deadline := time.Now().Add(w.timeout)
	w.setWriteDeadline(deadline)
}

func (w *writer) flush() {
	if w.err != nil {
		return
	}
	w.setDeadline()
	err := w.bufferedWriter.Flush()
	if err != nil {
		w.err = err
	}
}

func (w *writer) write(b []byte) {
	if w.err != nil {
		return
	}
	w.setDeadline()
	_, err := w.bufferedWriter.Write(b)
	if err != nil {
		w.err = err
	}
}

func encodeLength(l int) []byte {
	switch {
	case l < 0x80:
		return []byte{byte(l)}
	case l < 0x4000:
		return []byte{byte(l>>8) | 0x80, byte(l)}
	case l < 0x200000:
		return []byte{byte(l>>16) | 0xC0, byte(l >> 8), byte(l)}
	case l < 0x10000000:
		return []byte{byte(l>>24) | 0xE0, byte(l >> 16), byte(l >> 8), byte(l)}
	default:
		return []byte{0xF0, byte(l >> 24), byte(l >> 16), byte(l >> 8), byte(l)}
	}
}
