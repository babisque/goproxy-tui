package proxy

import (
	"bytes"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

type RequestLog struct {
	Method  string
	URL     string
	Status  int
	Headers http.Header
	Body    string
}

type InterceptAction struct {
	Allow        bool
	ModifiedBody *string
}

type InterceptRequest struct {
	Log      RequestLog
	ActionCh chan InterceptAction
}

type ProxyHandler struct {
	LogChannel    chan RequestLog
	InterceptChan chan InterceptRequest
	InterceptMode bool
	InterceptMu   sync.RWMutex

	IgnoredDomains *DomainList
	BlockedDomains *DomainList
	InterceptRules []InterceptRule
	ResponseRules  []ResponseRule
	RequestRules   []RequestRule
	configFile     string
	CACert         *x509.Certificate
	CAKey          *rsa.PrivateKey
	certCache      map[string]*tls.Certificate
	certMu         sync.RWMutex
}

func NewProxyHandler(ch chan RequestLog, interceptCh chan InterceptRequest, configPath string, caCert *x509.Certificate, caKey *rsa.PrivateKey) *ProxyHandler {
	ph := &ProxyHandler{
		LogChannel:     ch,
		InterceptChan:  interceptCh,
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

	if strings.ToLower(r.Header.Get("Upgrade")) == "websocket" {
		ph.handleWebSocket(w, r)
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

	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
		if r.Body != nil {
			bodyBytes, _ := io.ReadAll(r.Body)
			r.Body.Close()

			newBody := ph.applyRequestInterception(r.Host, bodyBytes)

			r.Body = io.NopCloser(bytes.NewReader(newBody))

			r.ContentLength = int64(len(newBody))
			r.Header.Set("Content-Length", fmt.Sprint(len(newBody)))
		}
	}

	ph.InterceptMu.RLock()
	isIntercepting := ph.InterceptMode
	ph.InterceptMu.RUnlock()

	if isIntercepting && r.Method != http.MethodConnect {
		actionCh := make(chan InterceptAction)

		ph.InterceptChan <- InterceptRequest{
			Log: RequestLog{
				Method:  r.Method,
				URL:     r.Host + r.URL.Path,
				Status:  0,
				Headers: r.Header.Clone(),
				Body:    "[INTERCEPTED] Request is paused for review.",
			},
			ActionCh: actionCh,
		}

		action := <-actionCh

		if !action.Allow {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Request dropped by interceptor"))
			ph.LogChannel <- RequestLog{
				Method: r.Method, URL: r.Host + r.URL.Path, Status: 403, Body: "DROPPED",
			}
			return
		}

		if action.ModifiedBody != nil {
			newBody := []byte(*action.ModifiedBody)
			r.Body = io.NopCloser(bytes.NewReader(newBody))
			r.ContentLength = int64(len(newBody))
			r.Header.Set("Content-Length", fmt.Sprint(len(newBody)))
		}
	}

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

func (ph *ProxyHandler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		ph.LogChannel <- RequestLog{
			Method: "WSS", URL: r.Host,
			Status: 500, Body: "[ERROR] ResponseWriter does not support Hijack interface!",
		}
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		ph.LogChannel <- RequestLog{Method: "WSS", URL: r.Host, Status: 500, Body: "[ERROR] Failed to extract client socket: " + err.Error()}
		return
	}
	defer clientConn.Close()

	targetURL := r.URL.Host
	if targetURL == "" {
		targetURL = r.Host
	}
	cleanHost := strings.Split(targetURL, ":")[0]
	if !strings.Contains(targetURL, ":") {
		targetURL += ":443"
	}

	targetConn, err := tls.Dial("tcp", targetURL, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         cleanHost,
		NextProtos:         []string{"http/1.1"},
	})
	if err != nil {
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		ph.LogChannel <- RequestLog{Method: "WSS", URL: targetURL, Status: 502, Body: "[ERROR] TLS Dial failed: " + err.Error()}
		return
	}
	defer targetConn.Close()

	path := r.URL.Path
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	if path == "" {
		path = "/"
	}

	var reqBuilder strings.Builder
	reqBuilder.WriteString(fmt.Sprintf("%s %s HTTP/1.1\r\n", r.Method, path))
	reqBuilder.WriteString(fmt.Sprintf("Host: %s\r\n", cleanHost))
	for k, vv := range r.Header {
		if strings.ToLower(k) == "host" {
			continue
		}
		for _, v := range vv {
			reqBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
		}
	}
	reqBuilder.WriteString("\r\n")

	_, err = targetConn.Write([]byte(reqBuilder.String()))
	if err != nil {
		ph.LogChannel <- RequestLog{Method: "WSS", URL: targetURL, Status: 500, Body: "[ERROR] Failed to send raw bytes."}
		return
	}

	ph.LogChannel <- RequestLog{
		Method: "WSS", URL: r.Host + r.URL.Path,
		Status: 101, Body: "Tunnel established and active.",
	}

	errc := make(chan error, 2)
	cp := func(dst io.Writer, src io.Reader) {
		_, err := io.Copy(dst, src)
		errc <- err
	}

	go cp(targetConn, clientConn)
	go cp(clientConn, targetConn)

	<-errc

	ph.LogChannel <- RequestLog{
		Method: "WSS", URL: r.Host + r.URL.Path,
		Status: 101, Body: "WebSocket tunnel ended.",
	}
}

func (ph *ProxyHandler) ToggleIntercept() bool {
	ph.InterceptMu.Lock()
	defer ph.InterceptMu.Unlock()
	ph.InterceptMode = !ph.InterceptMode
	return ph.InterceptMode
}
