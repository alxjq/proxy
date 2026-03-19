package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// CLI flags
var (
	port        = flag.Int("port", 8080, "Port to listen on")
	enableCache = flag.Bool("cache", false, "Enable simple cache for GET requests")
	logLevel    = flag.String("loglevel", "info", "Log level: debug/info")
	forwardMode = flag.Bool("forward", false, "Enable forward proxy mode (CONNECT + any site)")
)

// Cache structure
type cacheEntry struct {
	body      []byte
	header    http.Header
	status    int
	expiresAt time.Time
}

var cache = struct {
	sync.RWMutex
	m map[string]*cacheEntry
}{m: make(map[string]*cacheEntry)}

func main() {
	flag.Parse()
	addr := fmt.Sprintf(":%d", *port)
	http.HandleFunc("/", proxyHandler)

	mode := "Reverse Proxy"
	if *forwardMode {
		mode = "Forward Proxy"
	}

	log.Printf("%s started: https://localhost%s (Cache: %v, Log level: %s)", mode, addr, *enableCache, *logLevel)
	log.Fatal(http.ListenAndServeTLS(addr, "cert.pem", "key.pem", nil))
}

// Proxy handler
func proxyHandler(w http.ResponseWriter, r *http.Request) {
	if *forwardMode && r.Method == http.MethodConnect {
		handleConnect(w, r)
		return
	}

	if *logLevel == "debug" {
		log.Println("[REQUEST]", r.Method, r.URL.String())
	}

	cacheKey := r.Method + ":" + r.URL.String()
	if *enableCache && r.Method == http.MethodGet {
		cache.RLock()
		ce, ok := cache.m[cacheKey]
		cache.RUnlock()
		if ok && time.Now().Before(ce.expiresAt) {
			copyHeaders(w.Header(), ce.header)
			w.WriteHeader(ce.status)
			w.Write(ce.body)
			log.Println("[CACHE HIT]", r.URL.String())
			return
		}
	}

	var targetURL string
	if *forwardMode {
		// Forward proxy mode: request URL is taken as is
		targetURL = r.URL.String()
	} else {
		// Reverse proxy mode: use original request host (or set your backend here)
		targetURL = r.URL.String()
	}

	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body.Close()

	outReq, err := http.NewRequest(r.Method, targetURL, io.NopCloser(bytes.NewReader(bodyBytes)))
	if err != nil {
		http.Error(w, "Failed to create request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	copyHeaders(outReq.Header, r.Header)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(outReq)
	if err != nil {
		http.Error(w, "Upstream server error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)

	if *enableCache && r.Method == http.MethodGet && resp.StatusCode == 200 {
		cache.Lock()
		cache.m[cacheKey] = &cacheEntry{
			body:      append([]byte(nil), respBody...),
			header:    resp.Header.Clone(),
			status:    resp.StatusCode,
			expiresAt: time.Now().Add(30 * time.Second),
		}
		cache.Unlock()
	}

	log.Println("[PROXY]", r.Method, r.URL.String(), "->", resp.StatusCode)
}

// Handle HTTPS CONNECT for forward proxy
func handleConnect(w http.ResponseWriter, r *http.Request) {
	destConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, "Failed to connect to host: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer destConn.Close()

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "Hijacking failed: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	// Send 200 OK to client
	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		return
	}

	// Bi-directional copy
	go io.Copy(destConn, clientConn)
	io.Copy(clientConn, destConn)
}

// Copy headers helper
func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
