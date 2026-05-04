package proxy

import (
	"bytes"
	"io"
	"net/http"
)

type RequestLog struct {
	Method  string
	URL     string
	Status  int
	Headers http.Header
	Body    string
}

type ProxyHandler struct {
	LogChannel chan RequestLog
}

func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.RequestURI = ""

	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, "Error forwarding request", http.StatusBadGateway)
		return
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Error reading response body", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	resp.Body = io.NopCloser(io.MultiReader(io.NopCloser(bytes.NewBuffer(bodyBytes)), resp.Body))

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
