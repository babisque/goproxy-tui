# Changelog

## [2.0.0] - 2026-05-08
### 🚀 Major Breakthrough: The Interception Update
- **Live Interceptor Engine**: A core architectural change in the proxy handler that allows pausing requests in-flight for manual inspection or modification.
- **Brutalist UI Overhaul**: Reimagined the entire TUI experience with a high-contrast, industrial monochrome aesthetic.
- **Live Payload Tampering**: Integrated a full-featured terminal editor to modify JSON payloads on the fly with automatic JSON formatting.

### ✨ New Arsenal Features
- **cURL Export (`C`)**: Instant conversion of captured requests into ready-to-run `curl` commands, copied directly to the system clipboard.
- **Turbo Flood / Race Condition Mode (`F`)**: A new concurrency testing tool that fires 50 simultaneous goroutines for stress-testing endpoints.
- **Smart Replay (`R`)**: Quick re-execution of any logged request.

### 🛠️ Technical Improvements
- **Windows Terminal Optimization**: Fixed height and layout calculations to prevent UI flickering and "ghost scrolling" in PowerShell and Windows Terminal.
- **Dynamic Protocol Handling**: Automatic `Content-Length` recalculation for tampered requests and improved Gzip decompression.