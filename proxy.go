package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// CLI bayrakları
var (
	port        = flag.Int("port", 8080, "dinlenecek port")
	enableCache = flag.Bool("cache", false, "GET istekleri için basit cache kullan")
	logLevel    = flag.String("loglevel", "info", "log seviyesi: debug/info")
)

// Cache yapısı
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

// main fonksiyonu
func main() {
	flag.Parse()
	addr := fmt.Sprintf(":%d", *port)
	http.HandleFunc("/", proxyHandler)

	log.Printf("Proxy başlatıldı: http://localhost%s (Cache: %v, Log seviyesi: %s)", addr, *enableCache, *logLevel)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// Proxy handler
func proxyHandler(w http.ResponseWriter, r *http.Request) {
	if *logLevel == "debug" {
		log.Println("[ISTEK]", r.Method, r.URL.String())
	}

	if r.Method == http.MethodConnect {
		http.Error(w, "HTTPS CONNECT desteklenmiyor", http.StatusMethodNotAllowed)
		return
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

	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, r.URL.String(), r.Body)
	if err != nil {
		http.Error(w, "İstek hazırlanamadı: "+err.Error(), http.StatusInternalServerError)
		return
	}
	copyHeaders(outReq.Header, r.Header)

	resp, err := doRequestWithRetry(outReq, 3, 500*time.Millisecond)
	if err != nil {
		http.Error(w, "Sunucu hatası: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	w.Write(body)

	if *enableCache && r.Method == http.MethodGet && resp.StatusCode == 200 {
		cache.Lock()
		cache.m[cacheKey] = &cacheEntry{
			body:      append([]byte(nil), body...),
			header:    resp.Header.Clone(),
			status:    resp.StatusCode,
			expiresAt: time.Now().Add(30 * time.Second),
		}
		cache.Unlock()
	}
	log.Println("[PROXY]", r.Method, r.URL.String(), "->", resp.StatusCode)
}

// Retry fonksiyonu
func doRequestWithRetry(req *http.Request, tries int, backoff time.Duration) (*http.Response, error) {
	var resp *http.Response
	var err error

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for i := 0; i < tries; i++ {
		r := req.Clone(context.Background())
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

// Header kopyalama
func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
