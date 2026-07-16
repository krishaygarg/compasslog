package main

import (
	"fmt"
	"math/rand"
	"os"
	"sync/atomic"
	"time"
)

var (
	// generatorDelayMs is the sleep time between log generations (in milliseconds).
	// We use atomic operations to allow dynamic updates from the web socket/HTTP dashboard handlers.
	generatorDelayMs int64 = 300
)

var ipAddresses = []string{
	"192.168.1.10", "10.0.0.15", "172.16.0.4", "192.168.1.105",
	"8.8.8.8", "1.1.1.1", "142.250.190.46", "34.120.17.8",
}

var httpMethods = []string{"GET", "POST", "PUT", "DELETE"}

var endpoints = []string{
	"/index.html", "/api/v1/users", "/api/v1/login", "/api/v1/products",
	"/static/css/main.css", "/static/js/app.js", "/api/v1/checkout", "/images/logo.png",
}

var statusCodes = []struct {
	code   int
	weight int
}{
	{200, 80},
	{201, 5},
	{304, 5},
	{404, 6},
	{500, 3},
	{503, 1},
}

func selectStatusCode(r *rand.Rand) int {
	totalWeight := 0
	for _, sc := range statusCodes {
		totalWeight += sc.weight
	}
	val := r.Intn(totalWeight)
	current := 0
	for _, sc := range statusCodes {
		current += sc.weight
		if val < current {
			return sc.code
		}
	}
	return 200
}

// StartGenerator concurrently simulates log writes into the target file.
func StartGenerator(filePath string, stopChan chan struct{}) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("[Generator Error] failed to open log file: %v\n", err)
		return
	}
	defer file.Close()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	fmt.Printf("[Generator] Started mock traffic writing to %s...\n", filePath)

	for {
		select {
		case <-stopChan:
			fmt.Println("[Generator] Stopped mock traffic.")
			return
		default:
			ip := ipAddresses[r.Intn(len(ipAddresses))]
			method := httpMethods[r.Intn(len(httpMethods))]
			endpoint := endpoints[r.Intn(len(endpoints))]
			code := selectStatusCode(r)
			size := r.Intn(5000) + 100
			
			// Calculate simulated latency
			latency := r.Intn(80) + 15
			if endpoint == "/api/v1/checkout" || endpoint == "/api/v1/login" {
				latency += r.Intn(300) + 100
			}
			// Occasional latency spikes
			if r.Intn(100) < 6 {
				latency += r.Intn(1000) + 400
			}

			// Format log line: "IP - - [Time] "Method Path HTTP/1.1" Code Size Latency"
			timestamp := time.Now().Format("02/Jan/2006:15:04:05 -0700")
			logLine := fmt.Sprintf("%s - - [%s] \"%s %s HTTP/1.1\" %d %d %d\n",
				ip, timestamp, method, endpoint, code, size, latency)

			_, err := file.WriteString(logLine)
			if err != nil {
				fmt.Printf("[Generator Error] failed to write line: %v\n", err)
				return
			}
			_ = file.Sync() // Ensure immediate flush so tailer registers it

			// Retrieve the dynamic delay safely
			delay := atomic.LoadInt64(&generatorDelayMs)
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}
}
