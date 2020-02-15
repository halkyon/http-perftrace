# http-perftrace

Experiment with the Go [httptrace](https://golang.org/pkg/net/http/httptrace/) library building a simple CLI tool to output
HTTP performance for a given URL. It's basically like siege or something, but written in Go, and much much simpler.

## TODO

* Configurable timeout
* Exit signal
* Run for a duration instead of just for the number of concurrent requests
* Double check hooks in `httptrace.ClientTrace` and if they make sense for this implementation
* Output summary of all requests made (averages, 95th percentile, etc)
* Use flags instead of checking `os.Args` directly
* Report errors from `runTest()` back to main instead of ignoring them
* Check that the concurrency is correct for these kind of tests. If testing for a duration,
using the pool concurrency pattern is probably better for this use case.
* Add linter, tests, etc

## Installation

`go get github.com/halkyon/http-perftrace`

## Usage

`$ http-perftrace https://www.google.com [concurrent-requests]`

## Example response

```
DNS: 2.992ms, TCP: 59.8398ms, TLS: 171.5414ms, Server processing: 151.5942ms, Total: 386.9648ms
DNS: 2.992ms, TCP: 58.8424ms, TLS: 171.5414ms, Server processing: 152.5916ms, Total: 387.9622ms
DNS: 2.992ms, TCP: 58.8424ms, TLS: 171.5414ms, Server processing: 152.5916ms, Total: 387.9622ms
DNS: 2.992ms, TCP: 58.8424ms, TLS: 171.5414ms, Server processing: 153.5893ms, Total: 388.9599ms
DNS: 3.9894ms, TCP: 58.8424ms, TLS: 171.5414ms, Server processing: 153.5893ms, Total: 388.9599ms
```