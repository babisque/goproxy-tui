package proxy

import (
	"fmt"
	"io"
	"net/http"
)

type ProxyHandler struct{}

func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.RequestURI = ""
	fmt.Println(r.Method, r.Host)

	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		fmt.Println("Error forwarding request:", err)
		http.Error(w, "Error forwarding request", http.StatusBadGateway)
		return
	}

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
	defer resp.Body.Close()
}
