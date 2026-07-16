package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// Serves the index.html file
func serveIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

// Server-Sent Events (SSE) endpoint to stream real-time metrics to client
func streamEvents(w http.ResponseWriter, r *http.Request) {
	// Set Headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Channel to receive live records
	// Create a ticker to push stats snapshots at regular intervals
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Initial connect message
	fmt.Fprintf(w, "event: open\ndata: connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			// Connection closed by client
			return
		case <-ticker.C:
			// Fetch metrics copy and serialize to json
			snapshot := Metrics.GetSnapshot()
			data, err := json.Marshal(snapshot)
			if err != nil {
				continue
			}

			// Write in SSE format: "data: {JSON}\n\n"
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// Handles updating the generator delay dynamically
func changeGeneratorDelay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !GeneratorEnabled {
		http.Error(w, "Conflict: Mock traffic generator is disabled", http.StatusConflict)
		return
	}

	delayVal := r.URL.Query().Get("delay")
	delay, err := strconv.ParseInt(delayVal, 10, 64)
	if err != nil || delay < 10 || delay > 10000 {
		http.Error(w, "Bad Request: delay must be an integer between 10ms and 10000ms", http.StatusBadRequest)
		return
	}

	// Update atomic generator speed counter
	atomic.StoreInt64(&generatorDelayMs, delay)
	fmt.Printf("[Server] Traffic generator delay set to %dms\n", delay)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Traffic rate updated to %dms", delay)))
}

// Serves the server configuration JSON payload
func serveConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	config := map[string]interface{}{
		"log_file":          LogFilePath,
		"generator_enabled": GeneratorEnabled,
		"port":              ServerPort,
	}

	if err := json.NewEncoder(w).Encode(config); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// StartServer sets up and launches the HTTP service.
func StartServer(port int) {
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/config", serveConfig)
	http.HandleFunc("/events", streamEvents)
	http.HandleFunc("/rate", changeGeneratorDelay)

	address := fmt.Sprintf(":%d", port)
	fmt.Printf("[Server] Dashboard available at http://localhost%s\n", address)
	
	if err := http.ListenAndServe(address, nil); err != nil {
		fmt.Printf("[Server Error] listener failed: %v\n", err)
	}
}
