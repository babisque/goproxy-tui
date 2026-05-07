package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/babisque/goproxy-tui/internal/proxy"
	"github.com/babisque/goproxy-tui/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	storagePath := getStoragePath()
	logChan := make(chan proxy.RequestLog)

	caCert, caKey, err := proxy.LoadOrCreateCA(storagePath)
	if err != nil {
		log.Fatal("Error loading/creating CA:", err)
	}

	intercepChan := make(chan proxy.InterceptRequest)
	configPath := filepath.Join(storagePath, "config.json")
	proxyHandler := proxy.NewProxyHandler(logChan, intercepChan, configPath, caCert, caKey)

	go func() {
		err := http.ListenAndServe(":8080", proxyHandler)
		if err != nil {
			log.Fatal("Error starting proxy server:", err)
		}
	}()

	app := tui.NewApp(proxyHandler, intercepChan)

	if _, err := tea.NewProgram(app, tea.WithAltScreen()).Run(); err != nil {
		log.Fatal("Error running TUI:", err)
	}
}

func getStoragePath() string {
	configDir, _ := os.UserConfigDir()
	path := filepath.Join(configDir, "goproxy-tui")

	_ = os.MkdirAll(path, 0755)
	return path
}
