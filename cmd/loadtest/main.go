package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// LoadTestConfig holds configuration for load testing
type LoadTestConfig struct {
	URL             string
	ConcurrentUsers int
	RequestsPerUser int
	Timeout         time.Duration
	TestDuration    time.Duration
	RampUpDuration  time.Duration
	ThinkTime       time.Duration
}

// LoadTestResult holds the result of a single request
type LoadTestResult struct {
	UserID     int
	RequestID  int
	StatusCode int
	Duration   time.Duration
	Success    bool
	Error      error
	Timestamp  time.Time
}

// LoadTestSummary holds the summary of load test results
type LoadTestSummary struct {
	TotalRequests       int
	SuccessfulRequests  int
	FailedRequests      int
	TotalDuration       time.Duration
	AverageResponseTime time.Duration
	MinResponseTime     time.Duration
	MaxResponseTime     time.Duration
	RequestsPerSecond   float64
	ErrorRate           float64
	ResponseTime95th    time.Duration
	ResponseTime99th    time.Duration
}

func main() {
	var config LoadTestConfig

	flag.StringVar(&config.URL, "url", "http://localhost:8081/api/v1/rates", "Target URL to test")
	flag.IntVar(&config.ConcurrentUsers, "users", 10, "Number of concurrent users")
	flag.IntVar(&config.RequestsPerUser, "requests", 100, "Number of requests per user")
	flag.DurationVar(&config.Timeout, "timeout", 30*time.Second, "Request timeout")
	flag.DurationVar(&config.TestDuration, "duration", 0, "Test duration (0 = run until all requests complete)")
	flag.DurationVar(&config.RampUpDuration, "rampup", 5*time.Second, "Ramp-up duration")
	flag.DurationVar(&config.ThinkTime, "think", 100*time.Millisecond, "Think time between requests")
	flag.Parse()

	fmt.Printf("Starting load test...\n")
	fmt.Printf("URL: %s\n", config.URL)
	fmt.Printf("Concurrent Users: %d\n", config.ConcurrentUsers)
	fmt.Printf("Requests per User: %d\n", config.RequestsPerUser)
	fmt.Printf("Timeout: %v\n", config.Timeout)
	fmt.Printf("Ramp-up Duration: %v\n", config.RampUpDuration)
	fmt.Printf("Think Time: %v\n", config.ThinkTime)
	fmt.Printf("Test Duration: %v\n", config.TestDuration)
	fmt.Println()

	// Run load test
	summary := runLoadTest(config)

	// Print results
	printSummary(summary)
}

func runLoadTest(config LoadTestConfig) LoadTestSummary {
	results := make(chan LoadTestResult, config.ConcurrentUsers*config.RequestsPerUser)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: config.Timeout,
	}

	// Start time
	startTime := time.Now()

	// Create context for test duration
	var ctx context.Context
	var cancel context.CancelFunc

	if config.TestDuration > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), config.TestDuration)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	// Launch user goroutines
	var wg sync.WaitGroup
	rampUpDelay := config.RampUpDuration / time.Duration(config.ConcurrentUsers)

	for userID := 0; userID < config.ConcurrentUsers; userID++ {
		wg.Add(1)
		go func(uid int) {
			defer wg.Done()

			// Ramp-up delay
			time.Sleep(time.Duration(uid) * rampUpDelay)

			// Make requests
			for reqID := 0; reqID < config.RequestsPerUser; reqID++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				result := makeRequest(client, config.URL, uid, reqID)
				results <- result

				// Think time
				if config.ThinkTime > 0 {
					time.Sleep(config.ThinkTime)
				}
			}
		}(userID)
	}

	// Wait for all users to complete
	wg.Wait()
	close(results)

	totalDuration := time.Since(startTime)

	// Process results
	return processResults(results, totalDuration)
}

func makeRequest(client *http.Client, url string, userID, requestID int) LoadTestResult {
	start := time.Now()

	resp, err := client.Get(url)
	duration := time.Since(start)

	result := LoadTestResult{
		UserID:    userID,
		RequestID: requestID,
		Duration:  duration,
		Timestamp: start,
		Error:     err,
	}

	if err != nil {
		result.Success = false
		result.StatusCode = 0
		return result
	}

	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode >= 200 && resp.StatusCode < 300

	// Read response body to ensure complete request
	if resp.Body != nil {
		resp.Body.Close()
	}

	return result
}

func processResults(results <-chan LoadTestResult, totalDuration time.Duration) LoadTestSummary {
	var summary LoadTestSummary
	var responseTimes []time.Duration

	summary.TotalDuration = totalDuration

	for result := range results {
		summary.TotalRequests++
		responseTimes = append(responseTimes, result.Duration)

		if result.Success {
			summary.SuccessfulRequests++
		} else {
			summary.FailedRequests++
		}
	}

	if summary.TotalRequests == 0 {
		return summary
	}

	// Calculate metrics
	summary.ErrorRate = float64(summary.FailedRequests) / float64(summary.TotalRequests) * 100
	summary.RequestsPerSecond = float64(summary.TotalRequests) / totalDuration.Seconds()

	// Calculate response time statistics
	if len(responseTimes) > 0 {
		var totalResponseTime time.Duration
		summary.MinResponseTime = responseTimes[0]
		summary.MaxResponseTime = responseTimes[0]

		for _, rt := range responseTimes {
			totalResponseTime += rt
			if rt < summary.MinResponseTime {
				summary.MinResponseTime = rt
			}
			if rt > summary.MaxResponseTime {
				summary.MaxResponseTime = rt
			}
		}

		summary.AverageResponseTime = totalResponseTime / time.Duration(len(responseTimes))

		// Calculate percentiles
		summary.ResponseTime95th = calculatePercentile(responseTimes, 95)
		summary.ResponseTime99th = calculatePercentile(responseTimes, 99)
	}

	return summary
}

func calculatePercentile(times []time.Duration, percentile int) time.Duration {
	if len(times) == 0 {
		return 0
	}

	// Simple sort (bubble sort for small datasets)
	for i := 0; i < len(times)-1; i++ {
		for j := 0; j < len(times)-i-1; j++ {
			if times[j] > times[j+1] {
				times[j], times[j+1] = times[j+1], times[j]
			}
		}
	}

	index := int(float64(len(times)) * float64(percentile) / 100.0)
	if index >= len(times) {
		index = len(times) - 1
	}

	return times[index]
}

func printSummary(summary LoadTestSummary) {
	fmt.Println("=== Load Test Results ===")
	fmt.Printf("Total Requests: %d\n", summary.TotalRequests)
	fmt.Printf("Successful Requests: %d (%.2f%%)\n", summary.SuccessfulRequests,
		float64(summary.SuccessfulRequests)/float64(summary.TotalRequests)*100)
	fmt.Printf("Failed Requests: %d (%.2f%%)\n", summary.FailedRequests, summary.ErrorRate)
	fmt.Printf("Total Duration: %v\n", summary.TotalDuration)
	fmt.Printf("Requests per Second: %.2f\n", summary.RequestsPerSecond)
	fmt.Printf("Average Response Time: %v\n", summary.AverageResponseTime)
	fmt.Printf("Min Response Time: %v\n", summary.MinResponseTime)
	fmt.Printf("Max Response Time: %v\n", summary.MaxResponseTime)
	fmt.Printf("95th Percentile Response Time: %v\n", summary.ResponseTime95th)
	fmt.Printf("99th Percentile Response Time: %v\n", summary.ResponseTime99th)

	// Performance assessment
	fmt.Println("\n=== Performance Assessment ===")
	if summary.ErrorRate > 5.0 {
		fmt.Printf("⚠️  High error rate: %.2f%% (target: < 5%%)\n", summary.ErrorRate)
	} else {
		fmt.Printf("✅ Error rate: %.2f%% (good)\n", summary.ErrorRate)
	}

	if summary.AverageResponseTime > 2*time.Second {
		fmt.Printf("⚠️  High average response time: %v (target: < 2s)\n", summary.AverageResponseTime)
	} else {
		fmt.Printf("✅ Average response time: %v (good)\n", summary.AverageResponseTime)
	}

	if summary.RequestsPerSecond < 10 {
		fmt.Printf("⚠️  Low throughput: %.2f req/s (target: > 10 req/s)\n", summary.RequestsPerSecond)
	} else {
		fmt.Printf("✅ Throughput: %.2f req/s (good)\n", summary.RequestsPerSecond)
	}
}
