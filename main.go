package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"time"
)

const (
	exitFail = 1
)

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(exitFail)
	}
}

type timingData struct {
	dnsStartTime          time.Time
	dnsDoneTime           time.Time
	connectStartTime      time.Time
	connectDoneTime       time.Time
	connTime              time.Time
	firstResponseByteTime time.Time
	tlsHandshakeStartTime time.Time
	tlsHandshakeDoneTime  time.Time
	roundTripStartTime    time.Time
	roundTripDoneTime     time.Time
}

func (t *timingData) DNSLookupDuration() time.Duration {
	return t.dnsDoneTime.Sub(t.dnsStartTime)
}

func (t *timingData) TCPConnectDuration() time.Duration {
	return t.connectDoneTime.Sub(t.connectStartTime)
}

func (t *timingData) TLSHandshakeDuration() time.Duration {
	return t.tlsHandshakeDoneTime.Sub(t.tlsHandshakeStartTime)
}

func (t *timingData) ServerProcessingDuration() time.Duration {
	return t.firstResponseByteTime.Sub(t.connTime)
}

func (t *timingData) RoundTripDuration() time.Duration {
	return t.roundTripDoneTime.Sub(t.roundTripStartTime)
}

func run(args []string, stdout io.Writer) error {
	if len(args) < 2 {
		fmt.Fprintf(stdout, "Usage: %s http://someurl.com\n", args[0])
		return nil
	}

	timingData := &timingData{}
	req, err := newRequest(args[1], timingData)
	if err != nil {
		return err
	}

	timingData.roundTripStartTime = time.Now()
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return err
	}
	timingData.roundTripDoneTime = time.Now()

	fmt.Fprintf(stdout, "\n")

	fmt.Fprintf(stdout, "%s %s\n", resp.Proto, resp.Status)
	for name, value := range resp.Header {
		fmt.Fprintf(stdout, "%s: %s\n", name, value)
	}

	fmt.Fprintf(stdout, "\n")

	if timingData.DNSLookupDuration().Nanoseconds() > 0 {
		fmt.Fprintln(stdout, "DNS lookup:", timingData.DNSLookupDuration())
	} else {
		fmt.Fprintln(stdout, "DNS lookup: n/a")
	}

	fmt.Fprintln(stdout, "TCP connection:", timingData.TCPConnectDuration())

	if timingData.TLSHandshakeDuration().Nanoseconds() > 0 {
		fmt.Fprintln(stdout, "TLS handshake:", timingData.TLSHandshakeDuration())
	} else {
		fmt.Fprintln(stdout, "TLS handshake: n/a")
	}

	fmt.Println("Server processing:", timingData.ServerProcessingDuration())
	fmt.Println("Total round trip:", timingData.RoundTripDuration())

	return nil
}

func newRequest(addr string, timingData *timingData) (*http.Request, error) {
	req, err := http.NewRequest("GET", addr, nil)
	if err != nil {
		return nil, err
	}

	ctx := req.Context()
	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			timingData.dnsStartTime = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			timingData.dnsDoneTime = time.Now()
		},
		ConnectStart: func(_, _ string) {
			timingData.connectStartTime = time.Now()
		},
		ConnectDone: func(_, _ string, _ error) {
			timingData.connectDoneTime = time.Now()
		},
		GotConn: func(_ httptrace.GotConnInfo) {
			timingData.connTime = time.Now()
		},
		GotFirstResponseByte: func() {
			timingData.firstResponseByteTime = time.Now()
		},
		TLSHandshakeStart: func() {
			timingData.tlsHandshakeStartTime = time.Now()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			timingData.tlsHandshakeDoneTime = time.Now()
		},
	}

	return req.WithContext(httptrace.WithClientTrace(ctx, trace)), nil
}
