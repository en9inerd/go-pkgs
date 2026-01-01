package longpoll

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClient_Poll(t *testing.T) {
	// Create a test server that simulates long polling
	var mu sync.Mutex
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		count := requestCount
		mu.Unlock()

		// Simulate long polling: wait a bit, then respond
		time.Sleep(50 * time.Millisecond)

		// Return different responses based on request count
		if count >= 3 {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "done",
				"count":   count,
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "polling",
			"count":   count,
		})
	}))
	defer server.Close()

	client := NewWithConfig(Config{
		PollTimeout: 1 * time.Second,
		RetryDelay:  100 * time.Millisecond,
		MaxRetries:  3,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var responses []map[string]interface{}
	var muResp sync.Mutex

	err := client.Poll(ctx, server.URL, func(resp *http.Response) (string, bool, error) {
		var data map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return "", false, err
		}

		muResp.Lock()
		responses = append(responses, data)
		muResp.Unlock()

		// Stop polling when we get "done" message
		if msg, ok := data["message"].(string); ok && msg == "done" {
			return "", false, nil
		}

		return "", true, nil
	})

	if err != nil {
		t.Fatalf("Poll failed: %v", err)
	}

	if len(responses) < 3 {
		t.Errorf("Expected at least 3 responses, got %d", len(responses))
	}

	// Check last response
	lastResp := responses[len(responses)-1]
	if msg, ok := lastResp["message"].(string); !ok || msg != "done" {
		t.Errorf("Expected last response to have message 'done', got %v", lastResp)
	}
}

func TestClient_Poll_ContextCancellation(t *testing.T) {
	// Create a server that never responds (simulating a long poll)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hold the connection open
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	}))
	defer server.Close()

	client := NewWithConfig(Config{
		PollTimeout: 5 * time.Second,
		RetryDelay:  100 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := client.Poll(ctx, server.URL, func(resp *http.Response) (string, bool, error) {
		return "", true, nil
	})

	if err == nil {
		t.Error("Expected context cancellation error")
	}
	if err != context.DeadlineExceeded && err != context.Canceled {
		t.Errorf("Expected context error, got: %v", err)
	}
}

func TestClient_Poll_HandlerStops(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewWithConfig(Config{
		PollTimeout: 1 * time.Second,
	})

	ctx := context.Background()
	callCount := 0

	err := client.Poll(ctx, server.URL, func(resp *http.Response) (string, bool, error) {
		callCount++
		// Stop after first response
		return "", false, nil
	})

	if err != nil {
		t.Fatalf("Poll failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected handler to be called once, got %d", callCount)
	}
}

func TestClient_Poll_RetryOnError(t *testing.T) {
	var mu sync.Mutex
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		attempt := attempts
		mu.Unlock()

		// Fail first two attempts, succeed on third
		if attempt < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewWithConfig(Config{
		PollTimeout: 1 * time.Second,
		RetryDelay:  50 * time.Millisecond,
		MaxRetries:  5,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	success := false
	err := client.Poll(ctx, server.URL, func(resp *http.Response) (string, bool, error) {
		success = true
		return "", false, nil // Stop after success
	})

	if err != nil {
		t.Fatalf("Poll failed: %v", err)
	}

	if !success {
		t.Error("Expected to eventually succeed after retries")
	}
}

func TestClient_Poll_MaxRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewWithConfig(Config{
		PollTimeout: 100 * time.Millisecond,
		RetryDelay:  50 * time.Millisecond,
		MaxRetries:  2,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Poll(ctx, server.URL, func(resp *http.Response) (string, bool, error) {
		return "", true, nil
	})

	if err == nil {
		t.Error("Expected error after max retries")
	}
}

func TestClient_StopAll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hold connection open
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewWithConfig(Config{
		PollTimeout: 5 * time.Second,
	})

	ctx := context.Background()

	// Start multiple polling operations
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.Poll(ctx, server.URL, func(resp *http.Response) (string, bool, error) {
				return "", true, nil
			})
		}()
	}

	// Give them time to start
	time.Sleep(100 * time.Millisecond)

	// Check active count
	if client.ActiveCount() != 3 {
		t.Errorf("Expected 3 active polls, got %d", client.ActiveCount())
	}

	// Stop all
	client.StopAll()

	// Wait for them to stop
	wg.Wait()

	// Give a moment for cleanup
	time.Sleep(50 * time.Millisecond)

	if client.ActiveCount() != 0 {
		t.Errorf("Expected 0 active polls after StopAll, got %d", client.ActiveCount())
	}
}

func TestClient_WithHeader(t *testing.T) {
	var receivedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Custom-Header")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok")
	}))
	defer server.Close()

	client := New().WithHeader("X-Custom-Header", "test-value")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	client.Poll(ctx, server.URL, func(resp *http.Response) (string, bool, error) {
		return "", false, nil
	})

	if receivedHeader != "test-value" {
		t.Errorf("Expected header 'test-value', got '%s'", receivedHeader)
	}
}

func TestClient_Poll_DynamicURL(t *testing.T) {
	// Simulate Telegram Bot API getUpdates with offset parameter
	var mu sync.Mutex
	offset := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		currentOffset := offset
		offset += 2 // Simulate receiving 2 updates
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"result": []map[string]interface{}{
				{"update_id": currentOffset + 1},
				{"update_id": currentOffset + 2},
			},
		})
	}))
	defer server.Close()

	client := NewWithConfig(Config{
		PollTimeout: 1 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	baseURL := server.URL
	currentOffset := 0
	updatesReceived := 0

	err := client.Poll(ctx, baseURL+"?offset=0", func(resp *http.Response) (string, bool, error) {
		var data map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return "", false, err
		}

		result, ok := data["result"].([]interface{})
		if !ok {
			return "", false, fmt.Errorf("invalid response format")
		}

		updatesReceived += len(result)

		// Update offset for next request (like Telegram Bot API)
		if len(result) > 0 {
			lastUpdate := result[len(result)-1].(map[string]interface{})
			currentOffset = int(lastUpdate["update_id"].(float64)) + 1
		}

		// Return new URL with updated offset
		nextURL := fmt.Sprintf("%s?offset=%d", baseURL, currentOffset)

		// Stop after receiving enough updates
		if updatesReceived >= 6 {
			return "", false, nil
		}

		return nextURL, true, nil
	})

	if err != nil {
		t.Fatalf("Poll failed: %v", err)
	}

	if updatesReceived < 6 {
		t.Errorf("Expected at least 6 updates, got %d", updatesReceived)
	}
}

func TestClient_PollSimple(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewWithConfig(Config{
		PollTimeout: 1 * time.Second,
	})

	ctx := context.Background()
	callCount := 0

	err := client.PollSimple(ctx, server.URL, func(resp *http.Response) (bool, error) {
		callCount++
		return callCount < 2, nil // Stop after 2 calls
	})

	if err != nil {
		t.Fatalf("PollSimple failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}

func TestClient_Poll_POST(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	bodyCount := 0
	client := NewWithConfig(Config{
		PollTimeout: 1 * time.Second,
		Method:      http.MethodPost,
		BodyBuilder: func() (io.Reader, error) {
			bodyCount++
			return strings.NewReader(fmt.Sprintf("data=%d", bodyCount)), nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := client.Poll(ctx, server.URL, func(resp *http.Response) (string, bool, error) {
		return "", false, nil // Stop after first request
	})

	if err != nil {
		t.Fatalf("Poll with POST failed: %v", err)
	}

	if receivedBody == "" {
		t.Error("Expected request body to be sent")
	}
}

func ExampleClient_Poll() {
	// Create a long polling client
	client := NewWithConfig(Config{
		PollTimeout: 60 * time.Second, // Each poll can take up to 60 seconds
		RetryDelay:  1 * time.Second,  // Wait 1 second between retries
		MaxRetries:  -1,                // Unlimited retries
		Logger:      slog.Default(),
	})

	// Start polling
	ctx := context.Background()
	err := client.Poll(ctx, "https://api.example.com/events", func(resp *http.Response) (string, bool, error) {
		// Process the response
		var data map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return "", false, err // Stop polling on decode error
		}

		fmt.Printf("Received: %+v\n", data)

		// Continue polling with the same URL (empty string = reuse URL)
		return "", true, nil
	})

	if err != nil {
		fmt.Printf("Polling stopped with error: %v\n", err)
	}
}

func ExampleClient_Poll_withContext() {
	client := New()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Poll with automatic cancellation after 5 minutes
	err := client.Poll(ctx, "https://api.example.com/updates", func(resp *http.Response) (string, bool, error) {
		// Process response...
		// Return empty string to reuse URL, false to stop, true to continue
		return "", true, nil
	})

	if err != nil {
		fmt.Printf("Polling error: %v\n", err)
	}
}

func ExampleClient_Poll_telegramBotAPI() {
	// Example: Using longpoll client with Telegram Bot API getUpdates
	client := NewWithConfig(Config{
		PollTimeout: 50 * time.Second, // Telegram max timeout is 50 seconds
		RetryDelay:  1 * time.Second,
		MaxRetries:  -1,
	})

	botToken := "YOUR_BOT_TOKEN"
	baseURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", botToken)
	offset := 0

	ctx := context.Background()
	err := client.Poll(ctx, fmt.Sprintf("%s?timeout=50&offset=%d", baseURL, offset),
		func(resp *http.Response) (string, bool, error) {
			var result struct {
				OK     bool `json:"ok"`
				Result []struct {
					UpdateID int `json:"update_id"`
				} `json:"result"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return "", false, err
			}

			if !result.OK {
				return "", false, fmt.Errorf("telegram API error")
			}

			// Process updates
			for _, update := range result.Result {
				fmt.Printf("Received update: %d\n", update.UpdateID)
				// Handle the update...
			}

			// Update offset for next request
			if len(result.Result) > 0 {
				lastUpdateID := result.Result[len(result.Result)-1].UpdateID
				offset = lastUpdateID + 1
			}

			// Return new URL with updated offset
			nextURL := fmt.Sprintf("%s?timeout=50&offset=%d", baseURL, offset)
			return nextURL, true, nil
		})

	if err != nil {
		fmt.Printf("Telegram polling error: %v\n", err)
	}
}
