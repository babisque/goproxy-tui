package proxy

import (
	"io"
	"net/http"
	"strings"
	"time"
)

func (ph *ProxyHandler) ReplayRequest(reqLog RequestLog) {
	targetURL := reqLog.URL
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}

	req, err := http.NewRequest(reqLog.Method, targetURL, strings.NewReader(reqLog.Body))
	if err != nil {
		ph.LogChannel <- RequestLog{
			Method: reqLog.Method,
			URL:    reqLog.URL,
			Status: 500,
			Body:   "Internal Error building replay request: " + err.Error(),
		}
		return
	}

	for key, values := range reqLog.Headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		ph.LogChannel <- RequestLog{
			Method: reqLog.Method,
			URL:    reqLog.URL,
			Status: 502,
			Body:   "Bad Gateway: " + err.Error(),
		}
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	ph.LogChannel <- RequestLog{
		Method:  req.Method,
		URL:     req.URL.String(),
		Status:  resp.StatusCode,
		Headers: resp.Header.Clone(),
		Body:    string(bodyBytes),
	}
}
