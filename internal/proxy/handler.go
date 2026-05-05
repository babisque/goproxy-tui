package proxy

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
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
	InterceptRules []InterceptRule
	ResponseRules  []ResponseRule
	configFile     string
	CACert         *x509.Certificate
	CAKey          *rsa.PrivateKey
	certCache      map[string]*tls.Certificate
	certMu         sync.RWMutex
}

func NewProxyHandler(ch chan RequestLog, configPath string, caCert *x509.Certificate, caKey *rsa.PrivateKey) *ProxyHandler {
	ph := &ProxyHandler{
		LogChannel:     ch,
		IgnoredDomains: NewDomainList(),
		BlockedDomains: NewDomainList(),
		InterceptRules: []InterceptRule{},
		ResponseRules:  []ResponseRule{},
		configFile:     configPath,
		CACert:         caCert,
		CAKey:          caKey,
		certCache:      make(map[string]*tls.Certificate),
	}
	ph.LoadConfig()
	return ph
}

func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		ph.handleConnect(w, r)
		return
	}

	r.RequestURI = ""

	if ph.BlockedDomains.Contains(r.Host) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("ACCESS DENIED: Blocked by GoProxy!"))
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

	r.Header.Del("Accept-Encoding")

	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, "Error forwarding request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	resp.Header.Del("Transfer-Encoding")

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

	w.Header().Set("Content-Length", fmt.Sprint(len(newBody)))
	w.Header().Del("Content-Encoding")

	w.WriteHeader(resp.StatusCode)
	w.Write(newBody)

	ph.LogChannel <- RequestLog{
		Method: r.Method, URL: r.Host + r.URL.Path,
		Status: resp.StatusCode, Headers: resp.Header.Clone(),
		Body: string(newBody),
	}
}
