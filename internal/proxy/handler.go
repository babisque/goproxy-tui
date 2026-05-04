package proxy

import (
	"bytes"
	"io"
	"net/http"
	"sync"
)

type RequestLog struct {
	Method  string
	URL     string
	Status  int
	Headers http.Header
	Body    string
}

type ProxyHandler struct {
	LogChannel     chan RequestLog
	IgnoredDomains *DomainList
	BlockedDomains *DomainList
}

type DomainList struct {
	mu      sync.RWMutex
	domains map[string]bool
}

func (dl *DomainList) Add(domain string) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	dl.domains[domain] = true
}

func (dl *DomainList) Remove(domain string) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	delete(dl.domains, domain)
}

func (dl *DomainList) Contains(domain string) bool {
	dl.mu.RLock()
	defer dl.mu.RUnlock()

	_, exists := dl.domains[domain]
	return exists
}

func NewDomainList() *DomainList {
	return &DomainList{
		domains: make(map[string]bool),
	}
}

func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.RequestURI = ""

	if ph.BlockedDomains.Contains(r.Host) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Access to " + r.Host + " is blocked by the proxy rules."))

		ph.LogChannel <- RequestLog{
			Method:  r.Method,
			URL:     r.Host + r.URL.Path,
			Status:  http.StatusForbidden,
			Headers: http.Header{},
			Body:    "Blocked by proxy rules",
		}
		return
	}

	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, "Error forwarding request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	if ph.IgnoredDomains.Contains(r.Host) {
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Error reading response body", http.StatusInternalServerError)
		return
	}

	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)

	ph.LogChannel <- RequestLog{
		Method:  r.Method,
		URL:     r.Host + r.URL.Path,
		Status:  resp.StatusCode,
		Headers: resp.Header.Clone(),
		Body:    string(bodyBytes),
	}
}
