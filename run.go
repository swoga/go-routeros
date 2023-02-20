package routeros

import (
	"strconv"
	"strings"
	"time"

	"github.com/swoga/go-routeros/proto"
)

type asyncReply struct {
	chanReply
	Reply
}

// Run simply calls RunArgs().
func (c *Client) Run(sentence ...string) (*Reply, error) {
	return c.RunArgs(sentence)
}

// RunArgs sends a sentence to the RouterOS device and waits for the reply.
func (c *Client) RunArgs(sentence []string) (*Reply, error) {
	for _, word := range sentence {
		// check if word is empty or only contains spaces
		if len(strings.Trim(word, " ")) == 0 {
			return nil, errEmptyWord
		}
	}
	c.w.BeginSentence()
	for _, word := range sentence {
		c.w.WriteWord(word)
	}
	if !c.async {
		return c.endCommandSync()
	}
	a, err := c.endCommandAsync()
	if err != nil {
		return nil, err
	}

readAllSentences:
	for {
		timeout, timer := newTimeoutTimer(c.timeout)
		select {
		case _, open := <-a.reC:
			if timer != nil {
				timer.Stop()
			}
			if !open {
				break readAllSentences
			}
		case <-timeout:
			return nil, errAsyncTimeout
		}
	}
	return &a.Reply, a.err
}

func (c *Client) endCommandSync() (*Reply, error) {
	err := c.w.EndSentence()
	if err != nil {
		return nil, err
	}
	return c.readReply()
}

func (c *Client) endCommandAsync() (*asyncReply, error) {
	a := &asyncReply{}
	a.reC = make(chan *proto.Sentence)
	a.tag = "r" + strconv.FormatUint(c.nextTag(), 10)
	c.w.WriteWord(".tag=" + a.tag)

	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.w.EndSentence()
	if err != nil {
		return nil, err
	}
	if c.tags == nil {
		return nil, errAsyncLoopEnded
	}
	c.tags[a.tag] = a
	return a, nil
}

func newTimeoutTimer(d time.Duration) (timeout <-chan time.Time, timer *time.Timer) {
	if d > 0 {
		timer = time.NewTimer(d)
		timeout = timer.C
	}
	return
}
