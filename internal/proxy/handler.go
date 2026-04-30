package proxy

import (
	"fmt"
	"net/http"
)

type ProxyHandler struct{}

func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.Method, r.Host)
	w.Write([]byte("Intercepted by Go!"))
}
