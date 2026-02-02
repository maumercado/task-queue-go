// Package client provides a Go SDK for the Task Queue API.
//
// The client is generated from the OpenAPI specification and provides
// typed methods for all API operations, plus a WebSocket client for
// real-time event streaming.
//
// # Basic Usage
//
//	client, err := client.New("http://localhost:8080")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create a task
//	task, err := client.CreateTask(ctx, client.CreateTaskRequest{
//	    Type: "email",
//	    Payload: &map[string]interface{}{
//	        "to":      "user@example.com",
//	        "subject": "Hello",
//	    },
//	})
//
// # WebSocket Events
//
//	err := client.ConnectWebSocket(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.CloseWebSocket()
//
//	for event := range client.Events() {
//	    fmt.Printf("Event: %s\n", event.Type)
//	}
//
// # Configuration
//
// The client supports functional options for configuration:
//
//	client, err := client.New("http://localhost:8080",
//	    client.WithAPIKey("your-api-key"),
//	    client.WithTimeout(30 * time.Second),
//	)
package client
