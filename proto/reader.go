package proto

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"time"
)

// Reader reads sentences from a RouterOS device.
type Reader interface {
	ReadSentence(setDeadline bool) (*Sentence, error)
}

type ReaderDeadline interface {
	io.Reader
	SetReadDeadline(t time.Time) error
}

type reader struct {
	bufferedReader  *bufio.Reader
	setReadDeadline func(time.Time) error
	timeout         time.Duration
}

// NewReader returns a new Reader to read from r.
func NewReader(r ReaderDeadline, timeout time.Duration) Reader {
	return &reader{
		bufferedReader:  bufio.NewReader(r),
		setReadDeadline: r.SetReadDeadline,
		timeout:         timeout,
	}
}

func (r *reader) setDeadline(setDeadline bool) {
	deadline := time.Time{}

	if setDeadline {
		deadline = time.Now().Add(r.timeout)
	}
	r.setReadDeadline(deadline)
}

func (r *reader) readFull(buf []byte, setDeadline bool) (int, error) {
	r.setDeadline(setDeadline)
	return io.ReadFull(r.bufferedReader, buf)
}

// ReadSentence reads a sentence.
func (r *reader) ReadSentence(setDeadline bool) (*Sentence, error) {
	sen := NewSentence()
	for {
		b, err := r.readWord(setDeadline)
		if err != nil {
			return nil, err
		}
		if len(b) == 0 {
			return sen, nil
		}
		// Ex.: !re, !done
		if sen.Word == "" {
			sen.Word = string(b)
			continue
		}
		// Command tag.
		if bytes.HasPrefix(b, []byte(".tag=")) {
			sen.Tag = string(b[5:])
			continue
		}
		// Ex.: =key=value, =key
		if bytes.HasPrefix(b, []byte("=")) {
			t := bytes.SplitN(b[1:], []byte("="), 2)
			if len(t) == 1 {
				t = append(t, []byte{})
			}
			p := Pair{string(t[0]), string(t[1])}
			sen.List = append(sen.List, p)
			sen.Map[p.Key] = p.Value
			continue
		}
		return nil, fmt.Errorf("invalid RouterOS sentence word: %#q", b)
	}
}

func (r *reader) readNumber(size int, setDeadline bool) (int64, error) {
	b := make([]byte, size)
	_, err := r.readFull(b, setDeadline)
	if err != nil {
		return -1, err
	}
	var num int64
	for _, ch := range b {
		num = num<<8 | int64(ch)
	}
	return num, nil
}

func (r *reader) readLength(setDeadline bool) (int64, error) {
	l, err := r.readNumber(1, setDeadline)
	if err != nil {
		return -1, err
	}
	var n int64
	switch {
	case l&0x80 == 0x00:
	case (l & 0xC0) == 0x80:
		n, err = r.readNumber(1, true)
		l = l & ^0xC0 << 8 | n
	case l&0xE0 == 0xC0:
		n, err = r.readNumber(2, true)
		l = l & ^0xE0 << 16 | n
	case l&0xF0 == 0xE0:
		n, err = r.readNumber(3, true)
		l = l & ^0xF0 << 24 | n
	case l&0xF8 == 0xF0:
		l, err = r.readNumber(4, true)
	}
	if err != nil {
		return -1, err
	}
	return l, nil
}

func (r *reader) readWord(setDeadline bool) ([]byte, error) {
	l, err := r.readLength(setDeadline)
	if err != nil {
		return nil, err
	}
	b := make([]byte, l)
	_, err = r.readFull(b, true)
	if err != nil {
		return nil, err
	}
	return b, nil
}
