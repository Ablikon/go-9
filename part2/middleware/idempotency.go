package middleware

import (
	"database/sql"
	"log"
	"net/http"
	"net/http/httptest"
	"assignment9/part2/database"
)

// IdempotencyMiddleware wraps our HTTP handlers to prevent duplicate operations.
func IdempotencyMiddleware(store *database.DBStore, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			http.Error(w, "Idempotency-Key header required", http.StatusBadRequest)
			return
		}

		tx, err := store.DB.Begin()
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		var status string
		var responseCode sql.NullInt32
		var responseBody []byte

		// FOR UPDATE ensures only one transaction can access this row concurrently.
		err = tx.QueryRow(`
			SELECT status, response_code, response_body 
			FROM idempotency_keys 
			WHERE key = $1 FOR UPDATE
		`, key).Scan(&status, &responseCode, &responseBody)

		if err == sql.ErrNoRows {
			// Key not found: we are the first to execute this request.
			_, err = tx.Exec(`
				INSERT INTO idempotency_keys (key, status) 
				VALUES ($1, 'processing')
			`, key)

			if err != nil {
				// If another tx inserted it between our check and insert, handle the error
				http.Error(w, "Conflict inserting idempotency key", http.StatusConflict)
				return
			}
			tx.Commit() // Commit processing state so others see it

			// Execute main logic
			recorder := httptest.NewRecorder()
			next.ServeHTTP(recorder, r)

			// Logic completed. Save results back out of band.
			_, updateErr := store.DB.Exec(`
				UPDATE idempotency_keys 
				SET status = 'completed', response_code = $1, response_body = $2 
				WHERE key = $3
			`, recorder.Code, recorder.Body.Bytes(), key)

			if updateErr != nil {
				log.Printf("Failed to update idempotency store: %v", updateErr)
			}

			// Return response to client
			for k, v := range recorder.Header() {
				for _, vv := range v {
					w.Header().Add(k, vv)
				}
			}
			w.WriteHeader(recorder.Code)
			w.Write(recorder.Body.Bytes())
			return

		} else if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		// Key exists in DB
		if status == "processing" {
			http.Error(w, "Duplicate request in progress", http.StatusConflict)
			return
		}

		if status == "completed" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(int(responseCode.Int32))
			w.Write(responseBody)
			return
		}
	}
}
