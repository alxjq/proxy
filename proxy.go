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
	upstream    = flag.String("upstream", "http://localhost:9000", "Upstream server for reverse proxy")
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

	// Cache check
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

	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body.Close()

	var targetURL string

	if *forwardMode {
		// Forward proxy (full URL gerekir)
		if r.URL.Scheme == "" {
			targetURL = "http://" + r.Host + r.RequestURI
		} else {
			targetURL = r.URL.String()
		}
	} else {
		// Reverse proxy (FIX)
		targetURL = *upstream + r.RequestURI
	}

	outReq, err := http.NewRequestWithContext(
		r.Context(),
		r.Method,
		targetURL,
		io.NopCloser(bytes.NewReader(bodyBytes)),
	)
	if err != nil {
		http.Error(w, "Failed to create request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	copyHeaders(outReq.Header, r.Header)

	resp, err := doRequestWithRetry(outReq, 3, 500*time.Millisecond)
	if err != nil {
		http.Error(w, "Upstream server error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)

	// Cache store
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

// Handle HTTPS CONNECT
func handleConnect(w http.ResponseWriter, r *http.Request) {
	destConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, "Failed to connect: "+err.Error(), http.StatusServiceUnavailable)
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
		http.Error(w, "Hijack failed: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	go io.Copy(destConn, clientConn)
	io.Copy(clientConn, destConn)
}

// Retry logic
func doRequestWithRetry(req *http.Request, tries int, backoff time.Duration) (*http.Response, error) {
	var resp *http.Response
	var err error

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for i := 0; i < tries; i++ {
		r := req.Clone(req.Context())

		resp, err = client.Do(r)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		if resp != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}

		time.Sleep(backoff)
		backoff *= 2
	}

	return resp, err
}

// Header copy helper
func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
