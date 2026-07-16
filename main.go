package main

import (
	"flag"
	"fmt"
	"time"
)

// Global Configuration Variables
var (
	LogFilePath      string
	GeneratorEnabled bool
	ServerPort       int
	CustomRegex      string
)

func main() {
	// Parse CLI parameters
	flag.StringVar(&LogFilePath, "file", "access.log", "Path to log file to monitor")
	flag.BoolVar(&GeneratorEnabled, "generator", true, "Enable mock traffic generator")
	flag.IntVar(&ServerPort, "port", 8085, "HTTP server port")
	flag.StringVar(&CustomRegex, "regex", "", "Custom regex pattern with named capture groups (e.g. (?P<ip>\\S+)...)")
	flag.Parse()

	fmt.Println("==================================================")
	fmt.Println("    Compass Concurrent Log Monitor & Dashboard    ")
	fmt.Println("==================================================")
	fmt.Printf("[Config] Log File:    %s\n", LogFilePath)
	fmt.Printf("[Config] Generator:   %v\n", GeneratorEnabled)
	fmt.Printf("[Config] Port:        %d\n", ServerPort)
	if CustomRegex != "" {
		fmt.Printf("[Config] Custom Regex: %s\n", CustomRegex)
	}
	fmt.Println("==================================================")

	// Communication channels
	parsedLogChan := make(chan ParsedLog, 100)
	stopChan := make(chan struct{})

	// 1. Spawn Log Writer (Simulated Traffic Generator) if enabled
	if GeneratorEnabled {
		go StartGenerator(LogFilePath, stopChan)
	} else {
		fmt.Println("[Generator] Disabled. Monitoring existing log file changes only.")
	}

	// 2. Spawn Log Reader & Parser (Concurrent Tailer)
	go StartParser(LogFilePath, CustomRegex, parsedLogChan, stopChan)

	// 3. Spawn a goroutine to calculate rolling Requests Per Second
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopChan:
				return
			case <-ticker.C:
				Metrics.TickRequestRate()
			}
		}
	}()

	// 4. Drain the parsedLogChan to prevent blocking
	// (Our SSE server uses Metrics.GetSnapshot() directly to fetch the atomic buffer state)
	go func() {
		for range parsedLogChan {
			// Draining channel silently
		}
	}()

	// 5. Start the HTTP web server and block main thread
	StartServer(ServerPort)
}
