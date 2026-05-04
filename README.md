# GoProxy TUI 🕵️‍♂️🌐

A powerful, fast, and terminal-based Man-In-The-Middle (MITM) HTTP/HTTPS proxy written in Go. GoProxy TUI allows developers and security researchers to intercept, inspect, and modify web traffic in real-time through an elegant Terminal User Interface.

## ✨ Features

* **HTTPS Interception:** Dynamically generates SSL certificates to decrypt and inspect HTTPS traffic (MITM).
* **Terminal UI (TUI):** Built with Charmbracelet's `bubbletea` and `lipgloss` for a responsive, keyboard-driven interface.
* **Real-Time Inspection:** View HTTP Methods, URLs, Status Codes, Headers, and formatted JSON bodies.
* **Traffic Manipulation:**
  * **Block:** Instantly block domains with a 403 Forbidden response.
  * **Ignore:** Bypass MITM for specific domains (passthrough).
  * **Intercept:** Inject or modify HTTP Request Headers on the fly.
  * **Modify:** Find and replace text inside HTTP Response bodies (e.g., replacing words on a live website).
* **Smart Parsing:** Automatically handles `gzip` decompression and sanitizes binary garbage to keep the terminal layout intact.

## 🚀 Installation

1. Clone the repository:
   ```bash
   git clone [https://github.com/yourusername/goproxy-tui.git](https://github.com/yourusername/goproxy-tui.git)
   cd goproxy-tui
   ```
2. Build the project:
   ```bash
   go build -o goproxy-tui ./cmd/proxy
   ```

## ⚙️ Usage & Setup

1. **Run the Proxy:**
   ```bash
   ./goproxy-tui
    ```
   *The proxy will start on `localhost:8080` (default).*

2. **Install the CA Certificate:**
   On the first run, GoProxy generates a `ca.crt` file in the root directory. To intercept HTTPS traffic without browser security warnings, you **must** import this certificate into your OS or Browser's "Trusted Root Certification Authorities" store.

3. **Configure your Browser:**
   Set your system or browser proxy to `HTTP`, Address: `127.0.0.1`, Port: `8080`.

## ⌨️ Keybindings

* `j` / `k` or `Up` / `Down` : Navigate the request list.
* `tab` : Swap focus between the list and details panel.
* `/` : Search/Filter requests by URL or Method.
* `b` : Block a domain.
* `i` : Ignore a domain (passthrough).
* `n` : Intercept & add headers (Format: `domain.com,Header:Value`).
* `m` : Modify response body (Format: `domain.com,OldText:NewText`).
* `r` : Remove a domain from block/ignore lists.
* `q` or `Ctrl+C` : Quit application.

## 🛠️ Built With
* [Go](https://go.dev/) - The core language
* [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
* [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
```