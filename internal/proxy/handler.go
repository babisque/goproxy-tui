package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
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
	configFile     string
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

func (dl *DomainList) ToSlice() []string {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	var list []string
	for d := range dl.domains {
		list = append(list, d)
	}
	return list
}

type ConfigData struct {
	Blocked []string `json:"blocked_domains"`
	Ignored []string `json:"ignored_domains"`
}

func NewProxyHandler(ch chan RequestLog, configPath string) *ProxyHandler {
	ph := &ProxyHandler{
		LogChannel:     ch,
		IgnoredDomains: NewDomainList(),
		BlockedDomains: NewDomainList(),
		configFile:     configPath,
	}
	ph.LoadConfig()
	return ph
}

func (ph *ProxyHandler) LoadConfig() {
	data, err := os.ReadFile(ph.configFile)
	if err != nil {
		return
	}

	var config ConfigData
	if err := json.Unmarshal(data, &config); err != nil {
		return
	}

	for _, d := range config.Blocked {
		ph.BlockedDomains.Add(d)
	}
	for _, d := range config.Ignored {
		ph.IgnoredDomains.Add(d)
	}
}

func (ph *ProxyHandler) SaveConfig() {
	config := ConfigData{
		Blocked: ph.BlockedDomains.ToSlice(),
		Ignored: ph.IgnoredDomains.ToSlice(),
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	_ = os.WriteFile(ph.configFile, data, 0644)
}

func (ph *ProxyHandler) AddBlocked(domain string) {
	ph.BlockedDomains.Add(domain)
	ph.SaveConfig()
}

func (ph *ProxyHandler) AddIgnored(domain string) {
	ph.IgnoredDomains.Add(domain)
	ph.SaveConfig()
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
		w.Write([]byte("ACCESS DENIED: Blocked by Rodrigo's GoProxy!"))
		ph.LogChannel <- RequestLog{
			Method: r.Method, URL: r.Host + r.URL.Path,
			Status: http.StatusForbidden, Body: "Blocked by proxy rules",
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

	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)

	ph.LogChannel <- RequestLog{
		Method: r.Method, URL: r.Host + r.URL.Path,
		Status: resp.StatusCode, Headers: resp.Header.Clone(),
		Body: string(bodyBytes),
	}
}
