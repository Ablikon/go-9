package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"assignment9/part2/database"
	"assignment9/part2/handlers"
	"assignment9/part2/middleware"

	_ "github.com/lib/pq"
)

func main() {
	// Connect to local PostgreSQL (Based on standard configuration in workspace)
	// You may need to tweak "dbname" or "user" below!
	connStr := "postgres://abylajhanbegimkulov@localhost:5432/abylajhanbegimkulov?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Ensure our table exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS idempotency_keys (
			key VARCHAR(255) PRIMARY KEY,
			status VARCHAR(20) NOT NULL,
			response_code INT,
			response_body JSONB,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	store := database.NewDBStore(db)

	// Set up our HTTP server with middleware
	mux := http.NewServeMux()
	mux.HandleFunc("/pay", middleware.IdempotencyMiddleware(store, handlers.PaymentHandler))

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		fmt.Println("Server started at :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	time.Sleep(500 * time.Millisecond) // Let server boot

	// 3. Simulating a "Double-Click" Attack
	testKey := fmt.Sprintf("idemp-key-%d", time.Now().UnixNano())
	var wg sync.WaitGroup

	reqCount := 5
	fmt.Printf("\n--- Starting Double-Click Attack Simulation with %d requests ---\n", reqCount)

	for i := 0; i < reqCount; i++ {
		wg.Add(1)
		go func(reqID int) {
			defer wg.Done()

			req, _ := http.NewRequest(http.MethodPost, "http://localhost:8080/pay", nil)
			req.Header.Set("Idempotency-Key", testKey)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("Req %d Failed: %v\n", reqID, err)
				return
			}
			defer resp.Body.Close()

			fmt.Printf("Req %d - Status: %d\n", reqID, resp.StatusCode)
		}(i + 1)
	}

	// Wait for attack simulation to finish
	wg.Wait()

	// Fourth phase: Wait and retry after first completes
	time.Sleep(3 * time.Second)
	fmt.Println("\n--- Requesting again after completion ---")
	
	req, _ := http.NewRequest(http.MethodPost, "http://localhost:8080/pay", nil)
	req.Header.Set("Idempotency-Key", testKey)

	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()

	var p []byte
	if resp != nil {
		p = make([]byte, 100)
		n, _ := resp.Body.Read(p)
		p = p[:n]
	}
	fmt.Printf("Late Retry - Status: %d, Body: %s\n", resp.StatusCode, string(p))

	server.Close()
}
