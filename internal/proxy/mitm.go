package proxy

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"
)

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

func (m *mitmResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	rw := bufio.NewReadWriter(bufio.NewReader(m.conn), bufio.NewWriter(m.conn))
	return m.conn, rw, nil
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

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		NextProtos:   []string{"http/1.1"},
	}
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
