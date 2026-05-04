package proxy

import (
	"io"
	"net/http"
)

type RequestLog struct {
	Method string
	URL    string
	Status int
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
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)

	ph.LogChannel <- RequestLog{
		Method: r.Method,
		URL:    r.Host + r.URL.Path,
		Status: resp.StatusCode,
	}
}
