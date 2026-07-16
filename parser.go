package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ParsedLog represents a single parsed log line.
type ParsedLog struct {
	IP        string    `json:"ip"`
	Timestamp string    `json:"timestamp"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Code      int       `json:"code"`
	Size      int       `json:"size"`
	Latency   int       `json:"latency"` // in milliseconds
}

// MetricsSummary aggregates statistics in a thread-safe manner.
type MetricsSummary struct {
	mu            sync.RWMutex
	TotalRequests int            `json:"total_requests"`
	StatusCodes   map[string]int `json:"status_codes"`
	PathHits      map[string]int `json:"path_hits"`
	IPHits        map[string]int `json:"ip_hits"`
	AvgLatency    float64        `json:"avg_latency"`
	RequestRate   int            `json:"request_rate"` // req/sec in rolling window
	RecentLogs    []ParsedLog    `json:"recent_logs"`
	
	// Helper channels and counters
	latencySum int64
	secCounter int
}

var (
	// regex matching: IP - - [Timestamp] "Method Path Protocol" Code Size Latency (with named capture groups)
	logPattern = regexp.MustCompile(`^(?P<ip>\S+) - - \[(?P<timestamp>.*?)\] "(?P<method>\S+) (?P<path>\S+) \S+" (?P<code>\d+) (?P<size>\d+) (?P<latency>\d+)$`)
	Metrics    = &MetricsSummary{
		StatusCodes: make(map[string]int),
		PathHits:    make(map[string]int),
		IPHits:      make(map[string]int),
		RecentLogs:  make([]ParsedLog, 0),
	}
)

// AddRecord updates the shared metrics safely.
func (m *MetricsSummary) AddRecord(record ParsedLog) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalRequests++
	m.StatusCodes[strconv.Itoa(record.Code)]++
	m.PathHits[record.Path]++
	m.IPHits[record.IP]++
	m.latencySum += int64(record.Latency)
	m.AvgLatency = float64(m.latencySum) / float64(m.TotalRequests)
	
	m.secCounter++

	// Keep only the last 30 logs in buffer
	m.RecentLogs = append(m.RecentLogs, record)
	if len(m.RecentLogs) > 30 {
		m.RecentLogs = m.RecentLogs[1:]
	}
}

// TickRequestRate calculates rolling requests per second.
func (m *MetricsSummary) TickRequestRate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RequestRate = m.secCounter
	m.secCounter = 0
}

// GetSnapshot returns a copy of metrics for JSON serialization.
func (m *MetricsSummary) GetSnapshot() *MetricsSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Clone maps to prevent race conditions during JSON marshalling
	statusCodesClone := make(map[string]int)
	for k, v := range m.StatusCodes {
		statusCodesClone[k] = v
	}

	pathHitsClone := make(map[string]int)
	for k, v := range m.PathHits {
		pathHitsClone[k] = v
	}

	ipHitsClone := make(map[string]int)
	for k, v := range m.IPHits {
		ipHitsClone[k] = v
	}

	logsClone := make([]ParsedLog, len(m.RecentLogs))
	copy(logsClone, m.RecentLogs)

	return &MetricsSummary{
		TotalRequests: m.TotalRequests,
		StatusCodes:   statusCodesClone,
		PathHits:      pathHitsClone,
		IPHits:        ipHitsClone,
		AvgLatency:    m.AvgLatency,
		RequestRate:   m.RequestRate,
		RecentLogs:    logsClone,
	}
}

// parseLogLine parses a single log line against a regex using named captures with positional fallback.
func parseLogLine(line string, re *regexp.Regexp) (ParsedLog, error) {
	matches := re.FindStringSubmatch(line)
	if matches == nil {
		return ParsedLog{}, fmt.Errorf("line does not match pattern")
	}

	// Sensible defaults
	record := ParsedLog{
		Timestamp: time.Now().Format("02/Jan/2006:15:04:05 -0700"),
		Method:    "GET",
		Path:      "/",
		Code:      200,
	}

	groupNames := re.SubexpNames()
	hasAnyNamedGroup := false
	for _, name := range groupNames {
		if name != "" {
			hasAnyNamedGroup = true
			break
		}
	}

	if !hasAnyNamedGroup {
		// Fallback for positional captures
		if len(matches) >= 5 {
			record.IP = matches[1]
			record.Timestamp = matches[2]
			record.Method = matches[3]
			record.Path = matches[4]
		}
		if len(matches) >= 6 {
			if code, err := strconv.Atoi(matches[5]); err == nil {
				record.Code = code
			}
		}
		if len(matches) >= 7 {
			if size, err := strconv.Atoi(matches[6]); err == nil {
				record.Size = size
			}
		}
		if len(matches) >= 8 {
			if latency, err := strconv.Atoi(matches[7]); err == nil {
				record.Latency = latency
			}
		}
		return record, nil
	}

	// Extract values using named groups
	for idx, name := range groupNames {
		if idx == 0 || idx >= len(matches) {
			continue
		}
		value := matches[idx]
		switch name {
		case "ip":
			record.IP = value
		case "timestamp":
			record.Timestamp = value
		case "method":
			record.Method = value
		case "path":
			record.Path = value
		case "code":
			if code, err := strconv.Atoi(value); err == nil {
				record.Code = code
			}
		case "size":
			if size, err := strconv.Atoi(value); err == nil {
				record.Size = size
			}
		case "latency":
			if latency, err := strconv.Atoi(value); err == nil {
				record.Latency = latency
			}
		}
	}

	return record, nil
}

// StartParser starts tailing the target log file and parsing appends.
func StartParser(filePath string, customRegex string, outputChan chan ParsedLog, stopChan chan struct{}) {
	// Wait momentarily to allow generator to create the file if generator is enabled
	if GeneratorEnabled {
		time.Sleep(500 * time.Millisecond)
	}

	// Choose appropriate regex pattern
	activePattern := logPattern
	if customRegex != "" {
		compiled, err := regexp.Compile(customRegex)
		if err != nil {
			fmt.Printf("[Parser Error] failed to compile custom regex: %v. Falling back to default pattern.\n", err)
		} else {
			activePattern = compiled
			fmt.Println("[Parser] Using custom regex pattern for log parsing.")
		}
	}

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("[Parser Error] failed to open file: %v\n", err)
		return
	}
	defer file.Close()

	// Seek to end of file to tail new logs only
	_, _ = file.Seek(0, io.SeekEnd)
	reader := bufio.NewReader(file)

	fmt.Printf("[Parser] Started tailing log file: %s...\n", filePath)

	for {
		select {
		case <-stopChan:
			fmt.Println("[Parser] Tailing stopped.")
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// File hasn't changed, wait briefly before retrying
					time.Sleep(50 * time.Millisecond)
					continue
				}
				fmt.Printf("[Parser Error] read line failed: %v\n", err)
				return
			}

			// Clean the line and strip trailing newline character
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Parse matching log line
			record, err := parseLogLine(line, activePattern)
			if err != nil {
				fmt.Printf("[Parser Warning] line failed to parse: %v. Line: %q\n", err, line)
				continue // Skip malformed lines
			}

			// Update centralized metrics
			Metrics.AddRecord(record)

			// Forward to output channel
			select {
			case outputChan <- record:
			default:
				// Avoid blocking if server is slow draining channel
			}
		}
	}
}
