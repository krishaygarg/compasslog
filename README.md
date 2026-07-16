# CompassLog

A high-performance, real-time concurrent log parser, metrics aggregator, and telemetry dashboard written in Go.

`CompassLog` demonstrates how to leverage Go's powerful concurrency primitives (goroutines, channels, read-write mutexes, and atomic operations) to tail logs, compute real-time web metrics, stream data over Server-Sent Events (SSE), and interactively control system behavior from a dynamic frontend dashboard.

---

## 🏗️ Architecture & Concurrency Model

The project consists of three main pipelines running concurrently:
1. **Mock Traffic Generator (`generator.go`)**: Simulates structured web traffic requests and appends them to a shared log file.
2. **Concurrent Log Tailer & Parser (`parser.go`)**: Follows the log file, parses new entries on-the-fly using regular expressions, and updates rolling thread-safe metrics.
3. **HTTP Dashboard Server (`server.go`)**: Streams metric snapshots to the frontend using Server-Sent Events (SSE) and exposes endpoints to control generator speed dynamically.

### System Topology

```
+-------------------------------------------------------------+
|                     COMPASSLOG TOPOLOGY                    |
+-------------------------------------------------------------+
|                                                             |
|   +-----------------------+                                 |
|   |    Log Generator      | (Runs concurrently)             |
|   |    (generator.go)     |                                 |
|   +-----------+-----------+                                 |
|               |                                             |
|               v (Writes raw log lines)                      |
|       [ access.log File ]                                   |
|               |                                             |
|               v (Tails & reads EOF)                         |
|   +-----------+-----------+                                 |
|   |      Log Parser       | (Runs concurrently)             |
|   |      (parser.go)      |                                 |
|   +-----+-----------+-----+                                 |
|         |           |                                       |
|         |           v (Updates thread-safe metrics)         |
|         |       +---------------+                           |
|         |       | Metrics Store | (Aggregated via           |
|         |       | (sync.RWMutex)|  atomic states)           |
|         |       +-------+-------+                           |
|         |               ^                                   |
|         v (Drains raw)  | (Fetch snapshot)                  |
|     [ Channel Buffer ]  |                                   |
|                         v                                   |
|             +-----------+-----------+                       |
|             |      HTTP Server      | (server.go)           |
|             |    (port :8085)       |                       |
|             +-----------+-----------+                       |
|                         |                                   |
|                         v (Server-Sent Events)              |
|               [ Browser Dashboard ]                         |
|                   (index.html)                              |
|                                                             |
+-------------------------------------------------------------+
```

---

## ⚡ Concurrency Principles Used

- **Goroutines**: Runs the Traffic Generator, Log Parser, Rate Ticker, and HTTP Server on lightweight, independent thread contexts.
- **Channels**: Passes newly parsed logs between the processing stage and HTTP delivery mechanisms without blocking core operations.
- **Read-Write Mutex (`sync.RWMutex`)**: Ensures the central `MetricsSummary` can handle high-frequency concurrent writes from the parser while allowing the HTTP SSE thread to read metric snapshots race-free.
- **Atomic Operations (`sync/atomic`)**: Safely updates and reads the traffic generator sleep interval (`generatorDelayMs`) across HTTP handler threads and the generator loop without mutex locks.

---

## ✨ Features

- **Real-Time Visualization**: Beautiful dark-mode dashboard styled with CSS grids, linear gradients, custom Google fonts (`Outfit` & `JetBrains Mono`), and a custom live SVG latency chart.
- **Golden Signals Telemetry**: Tracks requests per second, average latency, success rate (status code distribution), and total requests.
- **Dynamic Speed Controls**: Instantly switch simulation speed (Slow, Medium, Hyper Speed) from the dashboard, communicating back to the Go server via POST requests to dynamically modify atomic thread parameters.
- **Live Tail Log Console**: Emulates a real shell terminal streaming new log outputs categorized by response codes (green for 200/300s, amber for 404s, red for 500s).

---

## 📂 Directory Layout

- `main.go`: Application entrypoint initializing channels, coordinating goroutines, loading `.env`, spawning the AI sidecar process, and starting the HTTP server.
- `generator.go`: Logic for simulating realistic mock HTTP traffic (endpoints, status code weights, and latency spikes).
- `parser.go`: Log parsing engine tailing the target log file, classifying requests via local sidecar, caching error diagnostics, and aggregating thread-safe telemetry.
- `anomaly_detector.py`: Python background service running a local Hugging Face zero-shot classifier model for real-time categorizations.
- `server.go`: REST endpoint setups (`/config`, `/rate`), HTML dashboard delivery, and SSE telemetry streams.
- `index.html`: Visual dark-mode dashboard displaying live statistics, SVG response latency, log console, and SRE AI diagnostics.
- `go.mod`: Go module declaration.
- `.env`: API configurations for deep Gemini incident analytics.

---

## 🚀 Getting Started & Configuration

### Prerequisites

- Go 1.21 or higher installed.
- Python 3.8+ with PyTorch and Transformers installed (required ONLY if using `-ai` mode):
  ```bash
  pip install transformers torch
  ```

### Command-Line Arguments

`CompassLog` can be configured completely through command-line flags:

| Flag | Type | Default | Description |
|---|---|---|---|
| `-file` | string | `"access.log"` | Path to the log file to monitor (absolute or relative). |
| `-generator` | bool | `true` | Set to `false` to disable the built-in mock traffic generator. |
| `-port` | int | `8085` | Port to start the HTTP web server and dashboard. |
| `-regex` | string | `""` | Custom regex pattern with named capture groups. |
| `-ai` | bool | `false` | Enable local AI anomaly detection & Gemini diagnostics telemetry. |

---

## 🧠 Hybrid AI SRE Diagnostics

When running in AI mode (`go run . -ai=true`), `CompassLog` uses a two-tier hybrid AI architecture:

### 1. Real-Time Local Tier (Hugging Face)
All parsed log lines are analyzed locally by a background Python sidecar process executing `typeform/distilbert-base-uncased-mnli` (a lightweight 268MB zero-shot classifier). It labels logs in real-time as:
* `NORMAL CLIENT REQUEST`
* `DATABASE ERROR`
* `AUTHENTICATION FAILURE`
* `NETWORK TIMEOUT`

This occurs entirely on CPU locally, introducing **zero token cost**.

### 2. Deep Diagnostic Tier (Gemini)
When a critical system anomaly occurs (e.g. status code `5xx` or high-confidence local database/auth/network failures), `CompassLog` calls the **Gemini 1.5 Flash** API:
* **Contextual Diagnosis**: Gemini analyzes the exact log signature and generates a concise, context-aware root cause diagnosis.
* **SRE Playbook**: Gemini outputs a step-by-step remediation playbook rendered directly in the **AI SRE Incident Diagnostic Center** card on the dashboard.

### 3. Token-Saving Cache
To remain extremely token-efficient and cost-effective, Gemini results are saved in an in-memory cache mapping unique signature keys (`Method|Path|Code`). If the same error signature recurs, it is instantly resolved from cache, **never calling the Gemini API twice for the same incident type**.

### 🔑 Setting up your Gemini API Key
To activate the Deep Diagnostic Tier, create a `.env` file in the project directory and supply your API key:
```env
GEMINI_API_KEY=AIzaSy...
```
*(If the key is missing or unset, the system will degrade gracefully, executing only the local Hugging Face real-time classification tier).*

---

## 🔍 Monitoring Your Own Logs & Services

To monitor your own live logs, run the application with the `-generator=false` flag and point it to your log file using the `-file` flag.

### 1. Monitoring Standard Nginx / Apache Access Logs

By default, standard combined access logs can be monitored with the default regex. Run:
```bash
go run . -generator=false -file=/var/log/nginx/access.log -port=9000
```

### 2. Custom Log Formats (Named Capture Groups)

For custom log formats, specify a custom regex pattern using `-regex`. To map fields to the dashboard, use Go's **named capture groups** in your regex:
- `(?P<ip>\S+)` - Client IP address
- `(?P<timestamp>.*?)` - Log timestamp
- `(?P<method>\S+)` - HTTP request method
- `(?P<path>\S+)` - Request path
- `(?P<code>\d+)` - HTTP status code
- `(?P<size>\d+)` - Response size in bytes
- `(?P<latency>\d+)` - Server latency / response time in milliseconds

#### Example: Monitoring Nginx Logs with Latency

If your Nginx log format is configured to output request time in milliseconds at the end of the line:
`log_format custom '$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent $request_time';`

You can monitor it using:
```bash
go run . -generator=false -file=/var/log/nginx/custom_access.log -regex='^(?P<ip>\S+) - \S+ \[(?P<timestamp>.*?)\] "(?P<method>\S+) (?P<path>\S+) \S+" (?P<code>\d+) (?P<size>\d+) (?P<latency>\d+)$'
```

#### Example: Basic Application Logs

If your service writes logs in the format `[TIMESTAMP] METHOD PATH STATUS LATENCYms (IP)` like this:
`[16/Jul/2026:10:58:06] GET /api/v1/health 200 12ms (192.168.1.5)`

Monitor it with:
```bash
go run . -generator=false -file=/var/log/my-app.log -regex='^\[(?P<timestamp>.*?)\] (?P<method>\S+) (?P<path>\S+) (?P<code>\d+) (?P<latency>\d+)ms \((?P<ip>\S+)\)$'
```

---

## 🔗 API Documentation

### GET `/`
Serves the web dashboard frontend.

### GET `/config`
Returns the server's current running configuration parameters.
**Response payload format:**
```json
{
  "generator_enabled": false,
  "log_file": "/var/log/nginx/access.log",
  "port": 8085
}
```

### GET `/events`
Initiates a persistent Server-Sent Events (SSE) connection that streams a JSON object containing the live statistics snapshot every 500ms.

**Response payload format:**
```json
{
  "total_requests": 1542,
  "status_codes": {
    "200": 1210,
    "404": 92,
    "500": 45
  },
  "path_hits": {
    "/api/v1/users": 342,
    "/index.html": 500
  },
  "ip_hits": {
    "192.168.1.10": 412
  },
  "avg_latency": 115.42,
  "request_rate": 20,
  "recent_logs": [
    {
      "ip": "192.168.1.105",
      "timestamp": "16/Jul/2026:10:53:34 -0700",
      "method": "GET",
      "path": "/api/v1/users",
      "code": 200,
      "size": 1500,
      "latency": 45
    }
  ]
}
```

### POST `/rate?delay=<ms>`
Updates the mock traffic generator speed dynamically (only applicable if `-generator=true`).
- `delay`: Number of milliseconds of sleep delay between generated traffic logs (must be an integer between `10` and `10000`).
- **Example**: `POST http://localhost:8085/rate?delay=100` updates generator interval to 100ms.
