# Self-Signed HTTPS Proxy (Go)

This project is a **simple HTTPS reverse proxy** written in Go.  
It uses a **self-signed certificate** for local development and testing, providing encrypted TLS communication.

## Features

- HTTPS (self-signed certificate)
- Basic GET request caching (with TTL)
- Retry mechanism for idempotent requests
- Logging with debug/info levels
- Concurrency-safe cache

---

## Requirements

- Go 1.20+ (or latest)
- OpenSSL (for generating certificates)

---

## Setup & Run

### 1️⃣ Generate self-signed certificate

Run the following in your terminal:

```bash
openssl req -x509 -newkey rsa:2048 -nodes \
-keyout key.pem -out cert.pem \
-days 365 -subj "/CN=localhost"
