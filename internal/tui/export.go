package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

func (a *App) exportCurrentRequest() string {
	filtered := a.FilteredRequests()
	if len(filtered) == 0 || a.cursor >= len(filtered) {
		return "No request selected"
	}

	req := filtered[a.cursor]

	safeURL := strings.ReplaceAll(req.URL, "/", "_")
	safeURL = strings.ReplaceAll(safeURL, ":", "_")
	safeURL = strings.ReplaceAll(safeURL, "?", "_")
	if len(safeURL) > 40 {
		safeURL = safeURL[:40]
	}

	fileName := fmt.Sprintf("dump_%s_%s_%d.json", req.Method, safeURL, time.Now().Unix())

	data, err := json.MarshalIndent(req, "", " ")
	if err != nil {
		return fmt.Sprintf("Error marshaling request: %v", err)
	}

	err = os.WriteFile(fileName, data, 0644)
	if err != nil {
		return fmt.Sprintf("Error writing file: %v", err)
	}

	return fmt.Sprintf("Request exported to %s", fileName)
}
