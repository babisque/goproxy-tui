package proxy

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"sync"
)

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

func NewDomainList() *DomainList {
	return &DomainList{
		domains: make(map[string]bool),
	}
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

func (ph *ProxyHandler) RemoveBlocked(domain string) {
	ph.BlockedDomains.Remove(domain)
	ph.SaveConfig()
}

func (ph *ProxyHandler) AddIgnored(domain string) {
	ph.IgnoredDomains.Add(domain)
	ph.SaveConfig()
}

func (ph *ProxyHandler) RemoveIgnored(domain string) {
	ph.IgnoredDomains.Remove(domain)
	ph.SaveConfig()
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

func (ph *ProxyHandler) AddResponseRule(rule ResponseRule) {
	ph.ResponseRules = append(ph.ResponseRules, rule)
	ph.SaveConfig()
}

func (ph *ProxyHandler) applyResponseInterception(host string, body []byte) []byte {
	cleanHost := strings.Split(host, ":")[0]

	for _, rule := range ph.ResponseRules {
		if rule.Host == cleanHost {
			body = bytes.ReplaceAll(body, []byte(rule.OldText), []byte(rule.NewText))
		}
	}
	return body
}
