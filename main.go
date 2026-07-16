package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Global Configuration Variables
var (
	LogFilePath      string
	GeneratorEnabled bool
	ServerPort       int
	CustomRegex      string
	AIEnabled        bool
	aiCmd            *exec.Cmd
)

// loadEnv parses the .env file in the current directory and sets environment variables
func loadEnv() {
	file, err := os.Open(".env")
	if err != nil {
		// File not found is fine, fallback to shell environment
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// Strip optional quotes around value
			val = strings.Trim(val, `"'`)
			os.Setenv(key, val)
		}
	}
}

// startAISidecar starts the python anomaly detector process in the background
func startAISidecar() {
	fmt.Println("[AI] Spawning local Hugging Face sidecar process (python3 anomaly_detector.py)...")
	aiCmd = exec.Command("python3", "anomaly_detector.py")
	aiCmd.Stdout = os.Stdout
	aiCmd.Stderr = os.Stderr
	
	err := aiCmd.Start()
	if err != nil {
		fmt.Printf("[AI Error] Failed to start Python sidecar: %v. AI telemetry disabled.\n", err)
		AIEnabled = false
		return
	}
	fmt.Printf("[AI] Sidecar process started successfully (PID: %d)\n", aiCmd.Process.Pid)
}

func main() {
	// 0. Load environment variables from .env
	loadEnv()

	// Parse CLI parameters
	flag.StringVar(&LogFilePath, "file", "access.log", "Path to log file to monitor")
	flag.BoolVar(&GeneratorEnabled, "generator", true, "Enable mock traffic generator")
	flag.IntVar(&ServerPort, "port", 8085, "HTTP server port")
	flag.StringVar(&CustomRegex, "regex", "", "Custom regex pattern with named capture groups (e.g. (?P<ip>\\S+)...)")
	flag.BoolVar(&AIEnabled, "ai", false, "Enable local AI anomaly detection & Gemini diagnostics telemetry")
	flag.Parse()

	fmt.Println("==================================================")
	fmt.Println("    Compass Concurrent Log Monitor & Dashboard    ")
	fmt.Println("==================================================")
	fmt.Printf("[Config] Log File:    %s\n", LogFilePath)
	fmt.Printf("[Config] Generator:   %v\n", GeneratorEnabled)
	fmt.Printf("[Config] Port:        %d\n", ServerPort)
	fmt.Printf("[Config] AI Engine:   %v\n", AIEnabled)
	if os.Getenv("GEMINI_API_KEY") != "" && os.Getenv("GEMINI_API_KEY") != "YOUR_GEMINI_API_KEY_HERE" {
		fmt.Println("[Config] Gemini API:  Active")
	} else {
		fmt.Println("[Config] Gemini API:  Inactive (Set GEMINI_API_KEY in .env)")
	}
	if CustomRegex != "" {
		fmt.Printf("[Config] Custom Regex: %s\n", CustomRegex)
	}
	fmt.Println("==================================================")

	// Communication channels
	parsedLogChan := make(chan ParsedLog, 100)
	stopChan := make(chan struct{})

	// Handle graceful shutdown signals to cleanly kill Python sidecar
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n[Server] Shutting down...")
		close(stopChan)
		if aiCmd != nil && aiCmd.Process != nil {
			fmt.Println("[AI] Stopping Python sidecar process...")
			_ = aiCmd.Process.Kill()
		}
		os.Exit(0)
	}()

	// 1. Spawn Log Writer (Simulated Traffic Generator) if enabled
	if GeneratorEnabled {
		go StartGenerator(LogFilePath, stopChan)
	} else {
		fmt.Println("[Generator] Disabled. Monitoring existing log file changes only.")
	}

	// 2. Spawn Python AI Sidecar if enabled
	if AIEnabled {
		startAISidecar()
	}

	// 3. Spawn Log Reader & Parser (Concurrent Tailer)
	go StartParser(LogFilePath, CustomRegex, parsedLogChan, stopChan)

	// 4. Spawn a goroutine to calculate rolling Requests Per Second
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

	// 5. Drain the parsedLogChan to prevent blocking
	// (Our SSE server uses Metrics.GetSnapshot() directly to fetch the atomic buffer state)
	go func() {
		for range parsedLogChan {
			// Draining channel silently
		}
	}()

	// 6. Start the HTTP web server and block main thread
	StartServer(ServerPort)
}
