package main

import (
	//"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)
var (
	upstreamURL      *url.URL
)

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	// Check if upstream is alive
	if !isUpstreamAlive() {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Upstream server is not available. Your application is either starting up or has errors."))
		return
	}
	
	// Create a reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	
	// Update the headers to allow for SSL redirection
	r.URL.Host = upstreamURL.Host
	r.URL.Scheme = upstreamURL.Scheme
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Host = upstreamURL.Host

	proxy.ServeHTTP(w, r)
}

func isUpstreamAlive() bool {
	client := http.Client{
		Timeout: 100 * time.Millisecond,
	}
	resp, err := client.Get(upstreamURL.String())
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500 // Consider any status below 500 as "alive"
}

