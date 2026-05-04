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
	InterceptRules []InterceptRule
	ResponseRules  []ResponseRule
	configFile     string
}

type DomainList struct {
	mu      sync.RWMutex
	domains map[string]bool
}

type ConfigData struct {
	Blocked        []string        `json:"blocked_domains"`
	Ignored        []string        `json:"ignored_domains"`
	InterceptRules []InterceptRule `json:"intercept_rules"`
	ResponseRules  []ResponseRule  `json:"response_rules"`
}

type InterceptRule struct {
	Host    string
	Headers map[string]string
}

type ResponseRule struct {
	Host    string
	OldText string
	NewText string
}

func (ph *ProxyHandler) AddResponseRule(rule ResponseRule) {
	ph.ResponseRules = append(ph.ResponseRules, rule)
	ph.SaveConfig()
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

func NewProxyHandler(ch chan RequestLog, configPath string) *ProxyHandler {
	ph := &ProxyHandler{
		LogChannel:     ch,
		IgnoredDomains: NewDomainList(),
		BlockedDomains: NewDomainList(),
		InterceptRules: []InterceptRule{},
		ResponseRules:  []ResponseRule{},
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
	ph.InterceptRules = config.InterceptRules
	ph.ResponseRules = config.ResponseRules
}

func (ph *ProxyHandler) SaveConfig() {
	config := ConfigData{
		Blocked:        ph.BlockedDomains.ToSlice(),
		Ignored:        ph.IgnoredDomains.ToSlice(),
		InterceptRules: ph.InterceptRules,
		ResponseRules:  ph.ResponseRules,
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

func (ph *ProxyHandler) RemoveBlocked(domain string) {
	ph.BlockedDomains.Remove(domain)
	ph.SaveConfig()
}

func (ph *ProxyHandler) RemoveIgnored(domain string) {
	ph.IgnoredDomains.Remove(domain)
	ph.SaveConfig()
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

	for _, rule := range ph.InterceptRules {
		if r.Host == rule.Host {
			for key, value := range rule.Headers {
				r.Header.Set(key, value)
			}
		}
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
	newBody := ph.applyResponseInterception(r.Host, bodyBytes)

	if len(newBody) != len(bodyBytes) {
		w.Header().Del("Content-Length")
	}

	w.WriteHeader(resp.StatusCode)
	w.Write(newBody)

	ph.LogChannel <- RequestLog{
		Method: r.Method, URL: r.Host + r.URL.Path,
		Status: resp.StatusCode, Headers: resp.Header.Clone(),
		Body: string(bodyBytes),
	}
}

func (ph *ProxyHandler) AddIntercept(host, key, val string) {
	found := false
	for i, rule := range ph.InterceptRules {
		if rule.Host == host {
			ph.InterceptRules[i].Headers[key] = val
			found = true
			break
		}
	}
	if !found {
		ph.InterceptRules = append(ph.InterceptRules, InterceptRule{
			Host:    host,
			Headers: map[string]string{key: val},
		})
	}
	ph.SaveConfig()
}

func (ph *ProxyHandler) applyResponseInterception(host string, body []byte) []byte {
	for _, rule := range ph.ResponseRules {
		if rule.Host == host {
			body = bytes.ReplaceAll(body, []byte(rule.OldText), []byte(rule.NewText))
		}
	}
	return body
}
