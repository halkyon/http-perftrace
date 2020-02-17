# http-perftrace

Experiment with the Go [httptrace](https://golang.org/pkg/net/http/httptrace/) library building a simple CLI tool to output
HTTP performance for a given URL. It's basically like siege or something, but written in Go, and much much simpler.

## TODO

* Double check hooks in `httptrace.ClientTrace` and their correct use in this tool
* Provide some more useful output at the end, maybe 95th percentiles for example
* Add colours
* Add interval between requests (or random interval)
* Add linter, tests, etc

## Installation

`go get github.com/halkyon/http-perftrace`

## Usage

`$ http-perftrace -u https://www.google.com [-c concurrency] [-d duration]`

## Example

```
$ http-perftrace -u https://www.google.com -c 1 -d 1s
Running for 1s with 1 concurrent workers

HTTP/2.0 200 OK - DNS: 3.9891ms, TCP: 53.363ms, TLS: 170.0507ms, Server processing: 187.4985ms, Total: 416.896ms
HTTP/2.0 200 OK - DNS: 997.3µs, TCP: 61.8344ms, TLS: 152.6072ms, Server processing: 191.4877ms, Total: 406.9266ms

Test ended. 2 requests made

Average DNS lookup: 2.4932ms
Average TCP connect: 57.5987ms
Average TLS handshake: 161.32895ms
Average server processing: 189.4931ms
Average round trip: 411.9113ms
```