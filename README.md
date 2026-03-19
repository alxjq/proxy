# Go Hybrid Reverse + Forward Proxy

This project is a **hybrid proxy** written in Go, capable of acting as both a **reverse proxy** and a **forward proxy**.  

- By default, it works as a **reverse proxy**, forwarding requests to a backend.  
- With the `-forward` flag, it can act as a **forward proxy**, allowing clients to access any HTTP/HTTPS site.  
- Supports **self-signed TLS**, caching, and logging.

---

## Features

- Reverse Proxy (default)  
- Forward Proxy (`-forward=true`)  
- HTTPS support with self-signed certificate  
- GET request caching (TTL: 30 seconds)  
- Logging with debug/info levels  
- Concurrency-safe

---

## Requirements

- Go 1.20+  
- OpenSSL (for generating self-signed certificate)

---

## Setup

### 1️⃣ Generate self-signed certificate

```bash
openssl req -x509 -newkey rsa:2048 -nodes \
-keyout key.pem -out cert.pem \
-days 365 -subj "/CN=localhost"
