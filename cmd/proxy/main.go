package main

import (
	"log"
	"net/http"

	"github.com/babisque/goproxy-tui/internal/proxy"
	"github.com/babisque/goproxy-tui/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	logChan := make(chan proxy.RequestLog)

	caCert, caKey, err := proxy.LoadOrCreateCA()
	if err != nil {
		log.Fatal("Error loading/creating CA:", err)
	}

	proxyHandler := proxy.NewProxyHandler(logChan, "config.json", caCert, caKey)

	go func() {
		err := http.ListenAndServe(":8080", proxyHandler)
		if err != nil {
			log.Fatal("Error starting proxy server:", err)
		}
	}()

	app := tui.NewApp(proxyHandler)

	if _, err := tea.NewProgram(app, tea.WithAltScreen()).Run(); err != nil {
		log.Fatal("Error running TUI:", err)
	}
}
