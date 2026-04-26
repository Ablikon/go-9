package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"time"

	"assignment9/part1/client"
)

func main() {
	rand.Seed(time.Now().UnixNano()) // For jitter randomization

	// 1. Create a test server simulating the payment gateway failures
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount <= 3 {
			// Fail first 3 requests
			w.WriteHeader(http.StatusServiceUnavailable) // 503
			w.Write([]byte("Service Unavailable"))
			return
		}
		// Succeed on the 4th request
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success"}`))
	}))
	defer server.Close()

	// 2. Configure our retry client configuration
	cfg := client.RetryConfig{
		MaxRetries: 5,
		BaseDelay:  500 * time.Millisecond,
		MaxDelay:   5 * time.Second,
	}

	// Wait 10 seconds max for the entire operation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/pay", nil)
	if err != nil {
		panic(err)
	}

	fmt.Println("Starting payment execution...")
	httpClient := &http.Client{}

	resp, err := client.ExecutePayment(ctx, httpClient, req, cfg)

	if err != nil {
		fmt.Printf("Payment failed permanently: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		fmt.Printf("Attempt %d: Success! Response: %s\n", requestCount, string(body))
	} else {
		fmt.Printf("Failed: HTTP %d: %s\n", resp.StatusCode, string(body))
	}
}
