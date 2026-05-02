package main

import (
	"log"

	"github.com/babisque/goproxy-tui/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// proxyHandler := proxy.ProxyHandler{}

	// fmt.Println("Starting proxy server on :8080")

	// err := http.ListenAndServe(":8080", &proxyHandler)
	// if err != nil {
	// 	log.Fatal("Error starting server:", err)
	// }

	app := tui.NewApp()
	if _, err := tea.NewProgram(app, tea.WithAltScreen()).Run(); err != nil {
		log.Fatal("Error running TUI:", err)
	}
}
