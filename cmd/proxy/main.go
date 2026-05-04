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

	ignored := proxy.NewDomainList()
	ignored.Add("google.com")
	ignored.Add("neverssl.com")

	blocked := proxy.NewDomainList()
	blocked.Add("pudim.com.br")

	proxyHanler := proxy.ProxyHandler{
		LogChannel:     logChan,
		IgnoredDomains: ignored,
		BlockedDomains: blocked,
	}

	go func() {
		err := http.ListenAndServe(":8080", &proxyHanler)
		if err != nil {
			log.Fatal("Error starting proxy server:", err)
		}
	}()

	app := tui.NewApp(&proxyHanler)
	if _, err := tea.NewProgram(app, tea.WithAltScreen()).Run(); err != nil {
		log.Fatal("Error running TUI:", err)
	}
}
