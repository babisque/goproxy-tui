package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/babisque/goproxy-tui/internal/proxy"
)

func main() {
	proxyHandler := proxy.ProxyHandler{}

	fmt.Println("Starting proxy server on :8080")

	err := http.ListenAndServe(":8080", &proxyHandler)
	if err != nil {
		log.Fatal("Error starting server:", err)
	}
}
