# 🧭 CompassLog

> **Transform raw server access logs into a real-time, interactive telemetry dashboard with automated, token-efficient AI SRE incident diagnostics.**

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue?logo=go)](https://golang.org)
[![Python Version](https://img.shields.io/badge/Python-3.8%2B-green?logo=python)](https://python.org)
[![Hugging Face](https://img.shields.io/badge/%F0%9F%A4%97%20Hugging%20Face-Zero--Shot-orange)](https://huggingface.co)
[![Gemini](https://img.shields.io/badge/Gemini-1.5%20Flash-purple?logo=googlegemini)](https://gemini.google.com)

---

### 💡 What is CompassLog?

When an outage occurs, reading raw lines in a log terminal is slow and stressful. **CompassLog** is a lightweight developer-centric tool that tails your local/production log files and instantly streams structured, real-time analytics to a web dashboard. 

It acts as a **local diagnostic command center** for developers and SREs—parsing log patterns, aggregating Golden Signals metrics, classifying anomalies locally on CPU with zero token cost, and querying Gemini on-demand to suggest remediation playbooks for active incidents.

---

## 🛠️ Core SRE Telemetry & Diagnostics Features

If you are a developer doing SRE, `CompassLog` offers several core features built specifically to automate incident response and telemetry:

1. **Two-Tier Hybrid AI Diagnostics (Zero-Cost + Deep RCA)**:
   * **Real-time Local Tier (Hugging Face)**: Employs `typeform/distilbert-base-uncased-mnli` (a 268MB zero-shot classifier) in a local Python background sidecar to categorize all request streams instantly on CPU (zero token cost) as `normal`, `database error`, `authentication failure`, or `network timeout`.
   * **Deep Diagnostic Tier (Gemini 1.5 Flash)**: When a critical error (such as HTTP `5xx` or a severe database/auth/timeout failure) occurs, the Go backend queries Gemini to obtain a context-aware root cause diagnosis and step-by-step SRE remediation playbook.
2. **Token-Saving Incident Cache**:
   * To prevent high API token usage, Gemini diagnostics are cached in memory using a signature key composed of `Method|Path|Code`. The system **never calls the Gemini API twice for the same incident type**, serving subsequent repeats from the local cache instantly.
3. **Real-time Log Tailing & Custom Format Parsing**:
   * Tails actual service logs (e.g., Nginx, Apache, or custom application logs) using concurrent file seeks.
   * Leverages **named regex capture groups** (e.g. `(?P<ip>\S+)`, `(?P<latency>\d+)`) to map fields dynamically without code modification.
4. **Golden Signals Telemetry**:
   * Real-time rolling aggregations of request rates (RPS), average latency, total request volumes, and success rate percentages mapped dynamically.
5. **Programmatic Alert Integrations (SSE)**:
   * Exposes a persistent Server-Sent Events (SSE) stream (`/events`) that pushes telemetry updates every 500ms. SREs can write custom script clients to pipe these live alerts directly into Slack, Discord, or PagerDuty.

---

## 🏗️ Architecture & Flow

```
+---------------------------------------------------------------------------------------------+
|                         COMPASSLOG HYBRID AI SRE TELEMETRY                                  |
+---------------------------------------------------------------------------------------------+
|                                                                                             |
|                     +---------------------+                                                 |
|                     |     Go Parser       |                                                 |
|                     |    (parser.go)      |                                                 |
|                     +---+-------------+---+                                                 |
|                         |             |                                                     |
|           (Real-Time HTTP)           (Conditional POST + Cache)                             |
|                         v             v                                                     |
|      +--------------------+         +-----------------------+                               |
|      |  Python AI Sidecar |         |      Gemini API       |                               |
|      |    (Local HF)      |         |  (Context Diagnostic) |                               |
|      +--------------------+         +-----------------------+                               |
|                                                                                             |
|                           \         /                                                       |
|                            v       v                                                        |
|                       +-----------------+                                                   |
|                       |  Metrics Store  |                                                   |
|                       +--------+--------+                                                   |
|                                |                                                            |
|                                v (SSE Stream)                                               |
|                      +------------------+                                                   |
|                      |   Dashboard UI   |                                                   |
|                      +------------------+                                                   |
|                                                                                             |
+---------------------------------------------------------------------------------------------+
```

---

## ⚡ Concurrency Principles Used

- **Goroutines**: Runs the Traffic Generator, Log Parser, Rate Ticker, and HTTP Server on lightweight, independent thread contexts.
- **Channels**: Passes newly parsed logs between the processing stage and HTTP delivery mechanisms without blocking core operations.
- **Read-Write Mutex (`sync.RWMutex`)**: Ensures the central `MetricsSummary` can handle high-frequency concurrent writes from the parser while allowing the HTTP SSE thread to read metric snapshots race-free.
- **Atomic Operations (`sync/atomic`)**: Safely updates and reads the traffic generator sleep interval (`generatorDelayMs`) across HTTP handler threads and the generator loop without mutex locks.

---

## 🚀 Getting Started & Configuration

### Prerequisites

- Go 1.21 or higher installed on your machine.
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

### 🔑 Setting up your Gemini API Key
To activate the Gemini Deep Diagnostic Tier, create a `.env` file in the project directory:
```env
GEMINI_API_KEY=your_actual_gemini_api_key
```

---

## 🔍 Developer Guide: Monitoring Your Own Services

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

---

## 🛠️ Developer Guide: Extending the Tool

As a developer, you can customize and extend `CompassLog`'s telemetry, UI, or AI models. Here is how:

### 1. Adding New Metric Collectors in Go

To track new statistics (e.g., tracking the most active HTTP methods, bandwidth consumed, or geolocation patterns):

1. **Update the Structures** in [parser.go](file:///Users/krishayg/Projects/Projects_In_Different_Languages/GoProject/parser.go):
   * Add the new counter/map to `MetricsSummary` (e.g. `MethodHits map[string]int`).
   * Initialize it in the package-level `Metrics` declaration.
2. **Enrich Aggregations** in `AddRecord()`:
   * Safely update your new field under the mutex lock (e.g. `m.MethodHits[record.Method]++`).
3. **Update Snapshot Delivery**:
   * Update `GetSnapshot()` to clone your map/counter to prevent race conditions during JSON serialization.
4. **Update the UI**:
   * Parse the new field in `index.html` inside `updateDashboard(data)` and render it in a new card.

### 2. Swapping the Local Hugging Face AI Model

The real-time classification is run by a background Python process in [anomaly_detector.py](file:///Users/krishayg/Projects/Projects_In_Different_Languages/GoProject/anomaly_detector.py). You can swap the model or add new classification labels:

1. Open [anomaly_detector.py](file:///Users/krishayg/Projects/Projects_In_Different_Languages/GoProject/anomaly_detector.py).
2. Change the `model` in the `pipeline` instantiation:
   ```python
   classifier = pipeline("zero-shot-classification", model="your-preferred-model-here")
   ```
3. Update `CANDIDATE_LABELS` and `PLAYBOOKS` dictionary mapping. The Go backend and dashboard UI will dynamically read the labels returned by the sidecar.

### 3. Integrating Alerting & Custom Event Clients

Developers can programmatically subscribe to log events and AI diagnostics by writing script clients (e.g. Python, Node.js, bash) that connect to the SSE endpoint.

#### Example: Python Client subscribing to SSE Alerts
```python
import sseclient
import requests
import json

response = requests.get("http://localhost:8085/events", stream=True)
client = sseclient.SSEClient(response)

for event in client.events():
    data = json.loads(event.data)
    recent_logs = data.get("recent_logs", [])
    for log in recent_logs:
        # Check if the local AI classified this as an auth anomaly
        if log.get("ai_diagnosis") == "AUTHENTICATION FAILURE":
            print(f"🚨 ALERT: Suspicious request from {log['ip']} to {log['path']}")
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
  "port": 8085,
  "ai_enabled": true,
  "gemini_active": true
}
```

### GET `/events`
Initiates a persistent Server-Sent Events (SSE) connection that streams a JSON object containing the live statistics snapshot every 500ms.

---

## 📁 Directory Layout

- `main.go`: Application entrypoint initializing channels, coordinating goroutines, loading `.env`, spawning the AI sidecar process, and starting the HTTP server.
- `generator.go`: Logic for simulating realistic mock HTTP traffic (endpoints, status code weights, and latency spikes).
- `parser.go`: Log parsing engine tailing the target log file, classifying requests via local sidecar, caching error diagnostics, and aggregating thread-safe telemetry.
- `anomaly_detector.py`: Python background service running a local Hugging Face zero-shot classifier model for real-time categorizations.
- `server.go`: REST endpoint setups (`/config`, `/rate`), HTML dashboard delivery, and SSE telemetry streams.
- `index.html`: Visual dark-mode dashboard displaying live statistics, SVG response latency, log console, and SRE AI diagnostics.
- `go.mod`: Go module declaration.
- `.env`: API configurations for deep Gemini incident analytics.
