package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PaymentHandler heavy business logic
func PaymentHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Processing started for request...")
	time.Sleep(2 * time.Second) // Heavy operation

	// Successfully paid
	resp := map[string]interface{}{
		"status":         "paid",
		"amount":         1000,
		"transaction_id": "uuid-12345-abcde",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
	fmt.Println("Processing completed.")
}
