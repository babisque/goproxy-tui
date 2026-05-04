package proxy

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
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
	if r.Method == http.MethodConnect {
		ph.handleConnect(w, r)
		return
	}

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
	cleanHost := strings.Split(host, ":")[0]

	for _, rule := range ph.ResponseRules {
		if rule.Host == cleanHost {
			body = bytes.ReplaceAll(body, []byte(rule.OldText), []byte(rule.NewText))
		}
	}
	return body
}

func (ph *ProxyHandler) getCertificate(host string) (*tls.Certificate, error) {
	host = strings.Split(host, ":")[0]

	ph.certMu.RLock()
	cert, ok := ph.certCache[host]
	ph.certMu.RUnlock()
	if ok {
		return cert, nil
	}

	ph.certMu.Lock()
	defer ph.certMu.Unlock()

	if cert, ok := ph.certCache[host]; ok {
		return cert, nil
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	serialNumber, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: host,
		},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{host},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, ph.CACert, &priv.PublicKey, ph.CAKey)
	if err != nil {
		return nil, err
	}

	newCert := &tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  priv,
	}

	ph.certCache[host] = newCert
	return newCert, nil
}

func (ph *ProxyHandler) handleConnect(w http.ResponseWriter, r *http.Request) {
	cert, err := ph.getCertificate(r.Host)
	if err != nil {
		http.Error(w, "Fail to generate MITM certificate", http.StatusInternalServerError)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer clientConn.Close()

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{*cert}}
	tlsClientConn := tls.Server(clientConn, tlsConfig)
	if err := tlsClientConn.Handshake(); err != nil {
		return
	}
	defer tlsClientConn.Close()

	for {
		req, err := http.ReadRequest(bufio.NewReader(tlsClientConn))
		if err != nil {
			break
		}

		req.URL.Scheme = "https"
		req.URL.Host = r.Host

		respWriter := &mitmResponseWriter{conn: tlsClientConn, header: make(http.Header)}
		ph.ServeHTTP(respWriter, req)
	}
}

type mitmResponseWriter struct {
	conn   net.Conn
	header http.Header
	status int
}

func (m *mitmResponseWriter) Header() http.Header { return m.header }

func (m *mitmResponseWriter) Write(b []byte) (int, error) {
	if m.status == 0 {
		m.WriteHeader(http.StatusOK)
	}
	return m.conn.Write(b)
}

func (m *mitmResponseWriter) WriteHeader(status int) {
	if m.status != 0 {
		return
	}
	m.status = status

	fmt.Fprintf(m.conn, "HTTP/1.1 %d %s\r\n", status, http.StatusText(status))

	m.header.Write(m.conn)

	m.conn.Write([]byte("\r\n"))
}
