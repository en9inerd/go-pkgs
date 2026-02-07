// Package longpoll provides a generic client for long polling HTTP requests.
//
// Long polling is a technique where the client makes a request to the server,
// and the server holds the request open until it has data to send or a timeout occurs.
// Once the server responds, the client immediately makes another request to continue
// receiving updates.
//
// This package is designed to work with various long polling APIs, including:
// - Telegram Bot API getUpdates
// - Custom long polling endpoints
// - Server-sent events alternatives
//
// Key features:
// - Dynamic URL updates (e.g., for offset parameters like Telegram Bot API)
// - Support for both GET and POST requests
// - Automatic retry with configurable backoff
// - Context cancellation support
// - Concurrent polling operations
//
// Example usage with static URL:
//
//	client := longpoll.NewWithConfig(longpoll.Config{
//		PollTimeout: 60 * time.Second,
//		RetryDelay:  1 * time.Second,
//		MaxRetries: -1, // unlimited
//	})
//
//	ctx := context.Background()
//	err := client.Poll(ctx, "https://api.example.com/events", func(resp *http.Response) (string, bool, error) {
//		var data map[string]interface{}
//		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
//			return "", false, err
//		}
//
//		// Process the data...
//		fmt.Printf("Received: %+v\n", data)
//
//		// Return empty string to reuse URL, true to continue polling
//		return "", true, nil
//	})
//
// Example with Telegram Bot API (dynamic URL updates):
//
//	client := longpoll.NewWithConfig(longpoll.Config{
//		PollTimeout: 50 * time.Second, // Telegram max is 50s
//	})
//
//	botToken := "YOUR_BOT_TOKEN"
//	baseURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", botToken)
//	offset := 0
//
//	err := client.Poll(ctx, fmt.Sprintf("%s?timeout=50&offset=%d", baseURL, offset),
//		func(resp *http.Response) (string, bool, error) {
//			var result struct {
//				OK     bool `json:"ok"`
//				Result []struct {
//					UpdateID int `json:"update_id"`
//				} `json:"result"`
//			}
//
//			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
//				return "", false, err
//			}
//
//			// Process updates...
//			if len(result.Result) > 0 {
//				lastUpdateID := result.Result[len(result.Result)-1].UpdateID
//				offset = lastUpdateID + 1
//			}
//
//			// Return new URL with updated offset
//			nextURL := fmt.Sprintf("%s?timeout=50&offset=%d", baseURL, offset)
//			return nextURL, true, nil
//		})
package longpoll
