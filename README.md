# GoProxy TUI 2.0 🕵️‍♂️🌐

GoProxy TUI is an elite, terminal-based Man-In-The-Middle (MITM) HTTP/HTTPS proxy written in Go. It provides security researchers and developers with a powerful, brutalist interface to intercept, tamper with, and analyze web traffic in real-time without leaving the terminal.

## ⚔️ The Arsenal (New in 2.0)

* **Live Interceptor:** Freeze time. Pause any incoming request to inspect it before it hits the internet.
* **Payload Tampering:** Use the built-in Live Editor to forge request bodies with real-time JSON auto-formatting.
* **Race Condition Testing:** Fire 50+ concurrent requests with one keystroke (Turbo Flood) to find concurrency bugs in APIs.
* **cURL Export:** Instantly copy any captured request to your system clipboard as a ready-to-run `curl` command.
* **Brutalist Aesthetic:** A distraction-free, high-contrast monochrome industrial UI designed for focus and speed.

## ✨ Core Features

* **HTTPS Interception:** Dynamically generates SSL certificates to decrypt and inspect HTTPS traffic (MITM).
* **Traffic Manipulation:**
  * **Block:** Instantly block domains with a 403 Forbidden response.
  * **Ignore:** Bypass MITM for specific domains (passthrough).
  * **Intercept:** Inject or modify HTTP Request Headers on the fly.
  * **Modify:** Find and replace text inside HTTP Response/Request bodies automatically.
* **Smart Parsing:** Automatically handles `gzip` decompression and sanitizes binary garbage to keep the terminal layout intact.

## 🚀 Installation

### Option 1: Pre-compiled Binaries (Recommended)
You can download the ready-to-use binaries for Windows, Linux, and macOS directly from the [Releases page](https://github.com/babisque/goproxy-tui/releases). No Go installation required!

### Option 2: Build from Source
1. Clone the repository:
   ```bash
   git clone [https://github.com/babisque/goproxy-tui.git](https://github.com/babisque/goproxy-tui.git)
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

   *The proxy will start on `localhost:8080` by default.*
2. **Install the CA Certificate:** On the first run, GoProxy generates a `ca.crt` file in your OS user configuration directory:
      * **Windows:** `%AppData%\goproxy-tui\ca.crt`
      * **Linux:** `~/.config/goproxy-tui/ca.crt`
      * **macOS:** `~/Library/Application Support/goproxy-tui/ca.crt`

   To intercept HTTPS traffic without browser security warnings, you **must** import this certificate into your OS or Browser's "Trusted Root Certification Authorities" store.

3. **Configure your Browser:** Set your system or browser proxy to `HTTP`, Address: `127.0.0.1`, Port: `8080`.

## ⌨️ Pro Keybindings

### Navigation & View

| Key | Action |
| --- | --- |
| `j` / `k` | Navigate the request list (Up/Down) |
| `tab` | Swap focus between Left/Right panels |
| `/` | Search/Filter requests by URL or Method |
| `c` | Clear the requests list |
| `v` | View active rules and configuration |
| `q` | Quit application |

### Intercept & Attack

| Key | Action |
| --- | --- |
| `I` | **Toggle Intercept Mode** (Pause/Resume time) |
| `e` | **Live Edit** (Open editor for the paused payload) |
| `a` / `d` | **Accept / Drop** a paused request |
| `C` | **Copy as cURL** to system clipboard |
| `F` | **Flood Mode** (Fire 50x concurrent requests) |
| `R` | **Replay** selected request |
| `d` | **Dump/Export** request to JSON file |

### Automated Rules

| Key | Action |
| --- | --- |
| `b` | Block a domain |
| `i` | Ignore a domain (passthrough) |
| `n` | Intercept & add headers (Format: `domain.com,Header:Value`) |
| `m` / `M` | Modify response/request body (Format: `domain.com,OldText:NewText`) |
| `r` | Remove a domain from block/ignore lists |

## 🛠️ Built With

* [Go](https://go.dev/) - The core language
* [Bubble Tea](https://github.com/charmbracelet/bubbletea) - The powerful TUI framework
* [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Terminal styling