package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type Server struct{}

func NewServer() Server {
	return Server{}
}

// GetHello implements ServerInterface.
// (GET /hello)
func (Server) GetHello(w http.ResponseWriter, r *http.Request, params GetHelloParams) {
	resp := HelloResponse{
		Message: fmt.Sprintf("Hello, World %s", params.Name),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// PostChat implements ServerInterface.
// (POST /chat)
func (Server) PostChat(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get API key from environment
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		http.Error(w, "API_KEY not configured", http.StatusInternalServerError)
		return
	}

	// Build chat completion request for AI Builder API
	chatReq := map[string]interface{}{
		"model": "gpt-5",
		"messages": []map[string]string{
			{"role": "user", "content": req.Message},
		},
	}

	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		http.Error(w, "Failed to marshal request", http.StatusInternalServerError)
		return
	}

	// Create HTTP request to AI Builder API
	httpReq, err := http.NewRequest("POST", "https://space.ai-builders.com/backend/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	// Send request
	client := &http.Client{}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		http.Error(w, "Failed to call AI API: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer httpResp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		http.Error(w, "AI API error: "+string(respBody), httpResp.StatusCode)
		return
	}

	// Parse chat completion response
	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		http.Error(w, "Failed to parse AI response", http.StatusInternalServerError)
		return
	}

	// Extract content from response
	content := ""
	if len(chatResp.Choices) > 0 {
		content = chatResp.Choices[0].Message.Content
	}

	// Return response
	resp := ChatResponse{
		Content: content,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// Ensure Server implements ServerInterface
var _ ServerInterface = (*Server)(nil)
