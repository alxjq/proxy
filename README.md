Go Hybrid Reverse + Forward Proxy

A hybrid proxy written in Go, capable of acting as both a reverse proxy and a forward proxy.

Default: Reverse Proxy (forwards requests to a backend)

With -forward flag: Forward Proxy (supports HTTP/HTTPS)

Supports self-signed TLS, GET caching, and logging

Features

Reverse Proxy (default)

Forward Proxy (-forward=true)

HTTPS support (self-signed certificate)

GET request caching (TTL: 30 seconds)

Logging (info / debug)

Concurrency-safe

Requirements

Go 1.20+

OpenSSL (for generating self-signed certificate)

Setup
1️⃣ Generate self-signed certificate
openssl req -x509 -newkey rsa:2048 -nodes \
-keyout key.pem -out cert.pem \
-days 365 -subj "/CN=localhost"
2️⃣ Run or build
# Run directly
go run main.go

# Or build binary
go build -o go-proxy main.go
./go-proxy
Usage
Reverse Proxy (default)
go run main.go -port=8080 -upstream=http://localhost:9000 -cache=true -loglevel=info
Forward Proxy
go run main.go -forward=true -port=8888 -cache=true -loglevel=debug

Example client request:

curl -x https://localhost:8888 -k https://example.com
Cache

Only GET requests are cached

Cache TTL: 30 seconds

Cache hits logged in debug mode

Retry Logic

Retries up to 3 times for 5xx upstream errors

Backoff: 500ms → doubles each retry

4xx errors are not retried

Logging

Info: Basic request/response logs

Debug: Includes request URL + cache hits

[REQUEST] GET https://localhost:8080/api/test
[CACHE HIT] https://localhost:8080/api/test
[PROXY] GET /api/test -> 200
