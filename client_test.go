package routeros

import (
	"context"
	"flag"
	"slices"
	"strings"
	"testing"
	"time"
)

var (
	routerosAddress  = flag.String("routeros.address", "", "RouterOS address:port")
	routerosUsername = flag.String("routeros.username", "admin", "RouterOS user name")
	routerosPassword = flag.String("routeros.password", "admin", "RouterOS password")
)

type liveTest struct {
	*testing.T
	c *Client
}

func newLiveTest(t *testing.T) *liveTest {
	tt := &liveTest{T: t}
	tt.connect()
	return tt
}

func (t *liveTest) connect() {
	if *routerosAddress == "" {
		t.Skip("Flag -routeros.address not set")
	}
	var err error
	t.c, err = Dial(*routerosAddress, *routerosUsername, *routerosPassword)
	if err != nil {
		t.Fatal(err)
	}
}

func (t *liveTest) run(sentence ...string) *Reply {
	t.Logf("Run: %#q", sentence)
	r, err := t.c.RunArgs(sentence)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Reply: %s", r)
	return r
}

func (t *liveTest) getUptime() {
	r := t.run("/system/resource/print")
	if len(r.Re) != 1 {
		t.Fatalf("len(!re)=%d; want 1", len(r.Re))
	}
	_, ok := r.Re[0].Map["uptime"]
	if !ok {
		t.Fatal("Missing uptime")
	}
}

func TestRunSync(tt *testing.T) {
	t := newLiveTest(tt)
	defer t.c.Close()
	t.getUptime()
}

func TestRunAsync(tt *testing.T) {
	t := newLiveTest(tt)
	defer t.c.Close()
	t.c.Async()
	t.getUptime()
}

func TestRunError(tt *testing.T) {
	t := newLiveTest(tt)
	defer t.c.Close()
	for i, sentence := range [][]string{
		{"/xxx"},
		{"/ip/address/add", "=address=127.0.0.2/32", "=interface=xxx"},
	} {
		t.Logf("#%d: Run: %#q", i, sentence)
		_, err := t.c.RunArgs(sentence)
		if err == nil {
			t.Error("Success; want error from RouterOS device trying to run an invalid command")
		}
	}
}

func TestRunEmptyWord(tt *testing.T) {
	t := newLiveTest(tt)
	defer t.c.Close()
	_, err := t.c.Run("/ip/address/add", "")
	if err != errEmptyWord {
		t.Errorf("expected error: %v, but got: %v", errEmptyWord, err)
	}
	_, err = t.c.Run("/ip/address/add", "   ")
	if err != errEmptyWord {
		t.Errorf("expected error: %v, but got: %v", errEmptyWord, err)
	}
}

func TestDialInvalidPort(t *testing.T) {
	c, err := Dial("127.0.0.1:xxx", "x", "x")
	if err == nil {
		c.Close()
		t.Fatalf("Dial succeeded; want error")
	}
	errors := []string{
		"dial tcp: lookup tcp/xxx: unknown port",
		"dial tcp: lookup tcp/xxx: Servname not supported for ai_socktype",
	}
	if !slices.Contains(errors, err.Error()) {
		t.Fatal(err)
	}
}

func TestDialTLSInvalidPort(t *testing.T) {
	c, err := DialTLS("127.0.0.1:xxx", "x", "x", nil)
	if err == nil {
		c.Close()
		t.Fatalf("Dial succeeded; want error")
	}
	errors := []string{
		"dial tcp: lookup tcp/xxx: unknown port",
		"dial tcp: lookup tcp/xxx: Servname not supported for ai_socktype",
	}
	if !slices.Contains(errors, err.Error()) {
		t.Fatal(err)
	}
}

func TestDialContextTimeoutWithBackroundContext(t *testing.T) {
	c, err := DialContext(context.Background(), "192.0.2.1:8729", "x", "x", time.Second)
	if err == nil {
		c.Close()
		t.Fatalf("TestDialContextTimeoutWithBackroundContext succeeded; want error")
	}

	if err.Error() != "dial tcp 192.0.2.1:8729: i/o timeout" {
		t.Fatalf("TestDialContextTimeoutWithBackroundContext: timeout expected. Has: %s", err)
	}
}

func TestDialContextTLSTimeoutWithBackroundContext(t *testing.T) {
	c, err := DialContextTLS(context.Background(), "192.0.2.1:8729", "x", "x", nil, time.Second)
	if err == nil {
		c.Close()
		t.Fatalf("TestDialContextTLSTimeoutWithBackroundContext succeeded; want error")
	}

	if err.Error() != "dial tcp 192.0.2.1:8729: i/o timeout" {
		t.Fatalf("TestDialContextTLSTimeoutWithBackroundContext: timeout expected. Has: %s", err)
	}
}

func TestDialContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	chErr := make(chan error)
	go func() {
		c, err := DialContext(ctx, "192.0.2.1:8729", "x", "x", 0)
		chErr <- err
		if err == nil {
			c.Close()
		}
	}()

	cancel()
	err := <-chErr
	if err == nil {
		t.Fatalf("TestDialContextCancel succeeded; want error")
	}

	if err.Error() != "dial tcp 192.0.2.1:8729: operation was canceled" {
		t.Fatalf("TestDialContextCancel: timeout expected. Has: %s", err)
	}
}

func TestDialContextTLSCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	chErr := make(chan error)
	go func() {
		c, err := DialContextTLS(ctx, "192.0.2.1:8729", "x", "x", nil, 0)
		chErr <- err
		if err == nil {
			c.Close()
		}
	}()

	cancel()
	err := <-chErr
	if err == nil {
		t.Fatalf("TestDialContextTLSCancel succeeded; want error")
	}

	if err.Error() != "dial tcp 192.0.2.1:8729: operation was canceled" {
		t.Fatalf("TestDialContextTLSCancel: timeout expected. Has: %s", err)
	}
}

func TestDialContextWithContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	chErr := make(chan error)
	go func() {
		c, err := DialContext(ctx, "192.0.2.1:8729", "x", "x", 0)
		chErr <- err
		if err == nil {
			c.Close()
		}
	}()

	err := <-chErr
	if err == nil {
		t.Fatalf("TestDialContextWithContextTimeout succeeded; want error")
	}

	if err.Error() != "dial tcp 192.0.2.1:8729: i/o timeout" {
		t.Fatalf("TestDialContextWithContextTimeout: timeout expected. Has: %s", err)
	}
}

func TestDialContextTLSWithContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	chErr := make(chan error)
	go func() {
		c, err := DialContextTLS(ctx, "192.0.2.1:8729", "x", "x", nil, 0)
		chErr <- err
		if err == nil {
			c.Close()
		}
	}()

	err := <-chErr
	if err == nil {
		t.Fatalf("TestDialContextTLSWithContextTimeout succeeded; want error")
	}

	if err.Error() != "dial tcp 192.0.2.1:8729: i/o timeout" {
		t.Fatalf("TestDialContextTLSWithContextTimeout: timeout expected. Has: %s", err)
	}
}

func TestInvalidLogin(t *testing.T) {
	if *routerosAddress == "" {
		t.Skip("Flag -routeros.address not set")
	}
	var err error
	c, err := Dial(*routerosAddress, "xxx", "APasswordThatWillNeverExistir")
	if err == nil {
		c.Close()
		t.Fatalf("Dial succeeded; want error")
	}
	if err.Error() != "from RouterOS device: cannot log in" &&
		err.Error() != "from RouterOS device: invalid user name or password (6)" {
		t.Fatal(err)
	}
}

func TestTrapHandling(tt *testing.T) {
	t := newLiveTest(tt)
	defer t.c.Close()

	cmd := []string{"/ip/dns/static/add", "=type=A", "=name=example.com", "=ttl=30", "=address=1.0.0.0"}

	_, _ = t.c.RunArgs(cmd)
	_, err := t.c.RunArgs(cmd)
	if err == nil {
		t.Fatal("Should've returned an error due to a duplicate")
	}
	devErr, ok := err.(*DeviceError)
	if !ok {
		t.Fatal("Should've returned a DeviceError")
	}
	message := devErr.Sentence.Map["message"]
	wanted := "entry already exists"
	if !strings.Contains(message, wanted) {
		t.Fatalf(`message=%#v; want %#v`, message, wanted)
	}
}
