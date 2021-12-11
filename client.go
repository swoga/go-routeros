/*
Package routeros is a pure Go client library for accessing Mikrotik devices using the RouterOS API.
*/
package routeros

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/swoga/go-routeros/proto"
)

// Client is a RouterOS API client.
type Client struct {
	Queue int

	conn    net.Conn
	r       proto.Reader
	w       proto.Writer
	closing bool
	async   bool
	nextTag int64
	tags    map[string]sentenceProcessor
	mu      sync.Mutex
}

// NewClient returns a new Client over rwc. Login must be called.
func NewClient(conn net.Conn) (*Client, error) {
	return &Client{
		conn: conn,
		r:    proto.NewReader(conn),
		w:    proto.NewWriter(conn),
	}, nil
}

// Dial connects and logs in to a RouterOS device.
func Dial(address, username, password string) (*Client, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	return newClientAndLogin(conn, username, password)
}

// DialContext connects and logs in to a RouterOS device.
func DialContext(ctx context.Context, address, username, password string, timeout time.Duration) (*Client, error) {
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	return newClientAndLogin(conn, username, password)
}

// DialTLS connects and logs in to a RouterOS device using TLS.
func DialTLS(address, username, password string, tlsConfig *tls.Config) (*Client, error) {
	conn, err := tls.Dial("tcp", address, tlsConfig)
	if err != nil {
		return nil, err
	}
	return newClientAndLogin(conn, username, password)
}

// DialContextTls connects and logs in to a RouterOS device using TLS.
func DialContextTLS(ctx context.Context, address, username, password string, tlsConfig *tls.Config, timeout time.Duration) (*Client, error) {
	dialer := net.Dialer{Timeout: timeout}
	tlsDialer := tls.Dialer{NetDialer: &dialer, Config: tlsConfig}

	conn, err := tlsDialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	return newClientAndLogin(conn, username, password)
}

func newClientAndLogin(conn net.Conn, username, password string) (*Client, error) {
	c, err := NewClient(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	err = c.Login(username, password)
	if err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// Close closes the connection to the RouterOS device.
func (c *Client) Close() {
	c.mu.Lock()
	if c.closing {
		c.mu.Unlock()
		return
	}
	c.closing = true
	c.mu.Unlock()
	c.conn.Close()
}

// Login runs the /login command. Dial and DialTLS call this automatically.
func (c *Client) Login(username, password string) error {
	r, err := c.Run("/login", "=name="+username, "=password="+password)
	if err != nil {
		return err
	}
	ret, ok := r.Done.Map["ret"]
	if !ok {
		// Login method post-6.43 one stage, cleartext and no challenge
		if r.Done != nil {
			return nil
		}
		return errors.New("RouterOS: /login: no ret (challenge) received")
	}

	// Login method pre-6.43 two stages, challenge
	b, err := hex.DecodeString(ret)
	if err != nil {
		return fmt.Errorf("RouterOS: /login: invalid ret (challenge) hex string received: %s", err)
	}

	_, err = c.Run("/login", "=name="+username, "=response="+c.challengeResponse(b, password))
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) challengeResponse(cha []byte, password string) string {
	h := md5.New()
	h.Write([]byte{0})
	io.WriteString(h, password)
	h.Write(cha)
	return fmt.Sprintf("00%x", h.Sum(nil))
}
