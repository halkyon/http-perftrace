package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"runtime"
	"strconv"
	"time"
)

const (
	exitFail       = 1
	timeoutSeconds = 60
)

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(exitFail)
	}
}

type traceTimes struct {
	dnsStart          time.Time
	dnsDone           time.Time
	connectStart      time.Time
	connectDone       time.Time
	conn              time.Time
	firstResponseByte time.Time
	tlsHandshakeStart time.Time
	tlsHandshakeDone  time.Time
	roundTripStart    time.Time
	roundTripDone     time.Time
}

type result struct {
	dnsLookup        time.Duration
	tcpConnect       time.Duration
	tlsHandshake     time.Duration
	serverProcessing time.Duration
	roundTrip        time.Duration
}

func (d *result) summary() string {
	return fmt.Sprintf(
		"DNS: %s, TCP: %s, TLS: %s, Server processing: %s, Total: %s",
		d.dnsLookup,
		d.tcpConnect,
		d.tlsHandshake,
		d.serverProcessing,
		d.roundTrip,
	)
}

func (r *result) load(t *traceTimes) {
	r.dnsLookup = t.dnsDone.Sub(t.dnsStart)
	r.tcpConnect = t.connectDone.Sub(t.connectStart)
	r.tlsHandshake = t.tlsHandshakeDone.Sub(t.tlsHandshakeStart)
	r.serverProcessing = t.firstResponseByte.Sub(t.conn)
	r.roundTrip = t.roundTripDone.Sub(t.roundTripStart)
}

func run(args []string, stdout io.Writer) error {
	if len(args) < 2 {
		fmt.Fprintf(stdout, "Usage: %s http://someurl.com\n", args[0])
		return nil
	}

	var concurrency int
	if len(args) == 3 {
		concurrency, _ = strconv.Atoi(args[2])
	} else {
		concurrency = 1
	}

	runtime.GOMAXPROCS(concurrency)

	results := make(chan *result)
	for i := 0; i < concurrency; i++ {
		go func() {
			runTest(args[1], results, stdout)
		}()
	}

	timeout := time.After(timeoutSeconds * time.Second)

	for i := 0; i < concurrency; i++ {
		select {
		case result := <-results:
			fmt.Fprintln(stdout, result.summary())
		case <-timeout:
			return fmt.Errorf("timed out after %d seconds", timeoutSeconds)
		}
	}

	return nil
}

func runTest(addr string, results chan *result, stdout io.Writer) error {
	times := &traceTimes{}
	req, err := newRequest(addr, times)
	if err != nil {
		return err
	}

	times.roundTripStart = time.Now()
	_, err = http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return err
	}
	times.roundTripDone = time.Now()

	result := &result{}
	result.load(times)

	results <- result

	return nil
}

func newRequest(addr string, times *traceTimes) (*http.Request, error) {
	req, err := http.NewRequest("GET", addr, nil)
	if err != nil {
		return nil, err
	}

	ctx := req.Context()
	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			times.dnsStart = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			times.dnsDone = time.Now()
		},
		ConnectStart: func(_, _ string) {
			times.connectStart = time.Now()
		},
		ConnectDone: func(_, _ string, _ error) {
			times.connectDone = time.Now()
		},
		GotConn: func(_ httptrace.GotConnInfo) {
			times.conn = time.Now()
		},
		GotFirstResponseByte: func() {
			times.firstResponseByte = time.Now()
		},
		TLSHandshakeStart: func() {
			times.tlsHandshakeStart = time.Now()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			times.tlsHandshakeDone = time.Now()
		},
	}

	return req.WithContext(httptrace.WithClientTrace(ctx, trace)), nil
}
