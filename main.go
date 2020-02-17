package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"runtime"
	"time"
)

const (
	exitFail       = 1
	timeoutSeconds = 60
)

func main() {
	if err := run(os.Stdout); err != nil {
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
	response         *http.Response
	dnsLookup        time.Duration
	tcpConnect       time.Duration
	tlsHandshake     time.Duration
	serverProcessing time.Duration
	roundTrip        time.Duration
}

func (r *result) summary() string {
	return fmt.Sprintf(
		"%s %s - DNS: %s, TCP: %s, TLS: %s, Server processing: %s, Total: %s",
		r.response.Proto,
		r.response.Status,
		r.dnsLookup,
		r.tcpConnect,
		r.tlsHandshake,
		r.serverProcessing,
		r.roundTrip,
	)
}

func (r *result) load(t *traceTimes) {
	r.dnsLookup = t.dnsDone.Sub(t.dnsStart)
	r.tcpConnect = t.connectDone.Sub(t.connectStart)
	r.tlsHandshake = t.tlsHandshakeDone.Sub(t.tlsHandshakeStart)
	r.serverProcessing = t.firstResponseByte.Sub(t.conn)
	r.roundTrip = t.roundTripDone.Sub(t.roundTripStart)
}

func run(stdout io.Writer) error {
	var url string
	var concurrency int

	flag.StringVar(&url, "u", "", "url to test")
	flag.IntVar(&concurrency, "c", 1, "number of concurrent requests")
	flag.Parse()

	if url == "" {
		return errors.New("-u not provided")
	}
	if concurrency < 1 {
		return errors.New("-c should be greater or equal to 1")
	}

	runtime.GOMAXPROCS(concurrency)

	results := make(chan *result)
	for i := 0; i < concurrency; i++ {
		go func() {
			result := &result{}
			runTest(result, url)
			results <- result
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

func runTest(result *result, url string) error {
	times := &traceTimes{}
	req, err := newRequest(url, times)
	if err != nil {
		return err
	}

	times.roundTripStart = time.Now()
	response, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return err
	}

	times.roundTripDone = time.Now()

	result.response = response
	result.load(times)

	return nil
}

func newRequest(url string, times *traceTimes) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
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
