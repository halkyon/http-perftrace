package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"
)

const (
	exitFail = 1
)

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(exitFail)
	}
}

func average(list []*time.Duration) time.Duration {
	var total time.Duration
	for _, r := range list {
		total += *r
	}
	return total / time.Duration(len(list))
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

func (r *result) load(t *traceTimes) {
	r.dnsLookup = t.dnsDone.Sub(t.dnsStart)
	r.tcpConnect = t.connectDone.Sub(t.connectStart)
	r.tlsHandshake = t.tlsHandshakeDone.Sub(t.tlsHandshakeStart)
	r.serverProcessing = t.firstResponseByte.Sub(t.conn)
	r.roundTrip = t.roundTripDone.Sub(t.roundTripStart)
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

type resultSummary struct {
	dnsLookups       []*time.Duration
	tcpConnects      []*time.Duration
	tlsHandshakes    []*time.Duration
	serverProcessing []*time.Duration
	roundTrips       []*time.Duration
}

func (s *resultSummary) load(r *result) {
	s.dnsLookups = append(s.dnsLookups, &r.dnsLookup)
	s.tcpConnects = append(s.tcpConnects, &r.tcpConnect)
	s.tlsHandshakes = append(s.tlsHandshakes, &r.tlsHandshake)
	s.serverProcessing = append(s.serverProcessing, &r.serverProcessing)
	s.roundTrips = append(s.roundTrips, &r.roundTrip)
}

func (s *resultSummary) String() string {
	var sb strings.Builder
	// todo: maybe use of templates would be better here
	sb.WriteString(fmt.Sprintf("Average DNS lookup: %s\n", average(s.dnsLookups)))
	sb.WriteString(fmt.Sprintf("Average TCP connect: %s\n", average(s.tcpConnects)))
	sb.WriteString(fmt.Sprintf("Average TLS handshake: %s\n", average(s.tlsHandshakes)))
	sb.WriteString(fmt.Sprintf("Average server processing: %s\n", average(s.serverProcessing)))
	sb.WriteString(fmt.Sprintf("Average round trip: %s\n", average(s.roundTrips)))
	return sb.String()
}

func run(stdout io.Writer) error {
	var url string
	var concurrency int
	var testDuration time.Duration

	flag.StringVar(&url, "u", "", "url to test")
	flag.IntVar(&concurrency, "c", 1, "number of concurrent requests")
	flag.DurationVar(&testDuration, "d", 10*time.Second, "time to run tests for")
	flag.Parse()

	if url == "" {
		return errors.New("-u not provided")
	}
	if concurrency < 1 {
		return errors.New("-c should be greater or equal to 1")
	}

	fmt.Fprintf(stdout, "Running for %s with %d concurrent workers\n\n", testDuration, concurrency)

	runtime.GOMAXPROCS(concurrency)

	results := make(chan *result)
	errs := make(chan error)
	done := time.After(testDuration)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	for i := 0; i < concurrency; i++ {
		go func(results chan *result) {
			for {
				result := &result{}
				err := runTest(result, url)
				if err != nil {
					errs <- err
					continue
				}
				results <- result
			}
		}(results)
	}

	summary := &resultSummary{}

	for {
		select {
		case <-done:
			fmt.Fprintf(stdout, "\nTest ended. %d requests made\n\n", len(summary.roundTrips))
			fmt.Fprintln(stdout, summary)
			return nil
		case <-interrupt:
			// todo: cleanup this duplication with the done case above
			fmt.Fprintf(stdout, "\nTest ended. %d requests made\n\n", len(summary.roundTrips))
			fmt.Fprintln(stdout, summary)
			signal.Stop(interrupt)
			return errors.New("interrupt signal received")
		case result := <-results:
			summary.load(result)
			fmt.Fprintln(stdout, result.summary())
		case err := <-errs:
			return err
		}
	}
}

func runTest(result *result, url string) error {
	times := &traceTimes{}
	req, err := newRequest(url, times)
	if err != nil {
		return err
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			DualStack: true,
		}).DialContext,
		DisableKeepAlives:     true,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	times.roundTripStart = time.Now()
	response, err := transport.RoundTrip(req)
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
