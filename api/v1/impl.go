package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// ANSI color codes for terminal output
const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorBold    = "\033[1m"
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
	log.Printf("%s%s[/chat] ========== New request ==========%s", colorBold, colorCyan, colorReset)

	// Parse request body
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("%s[/chat] Received message:%s %q", colorGreen, colorReset, req.Message)

	// Get API key from environment
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		http.Error(w, "API_KEY not configured", http.StatusInternalServerError)
		return
	}

	// Determine model (default to gpt-5)
	model := "gpt-5"
	if req.Model != nil && *req.Model != "" {
		model = *req.Model
	}

	// Default search tool definition
	searchTool := map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "search",
			"description": "Search the web for real-time information like weather, news, current events",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"keywords": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "Search keywords",
					},
				},
				"required": []string{"keywords"},
			},
		},
	}

	// Build initial messages
	messages := []interface{}{
		map[string]string{"role": "user", "content": req.Message},
	}

	// First API call
	finalContent := callAIAPI(apiKey, model, messages, []interface{}{searchTool}, w)
	if finalContent == nil {
		return // Error already written to response
	}

	resp := ChatResponse{
		Content: finalContent,
	}

	log.Printf("%s%s[/chat] ========== Request complete ==========%s", colorBold, colorCyan, colorReset)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// callAIAPI calls the AI Builder API and handles tool calls recursively
func callAIAPI(apiKey, model string, messages []interface{}, tools []interface{}, w http.ResponseWriter) *string {
	log.Printf("%s[/chat] Calling AI API%s (model: %s, messages: %d)...", colorYellow, colorReset, model, len(messages))

	chatReq := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"tools":       tools,
		"tool_choice": "auto",
	}

	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		http.Error(w, "Failed to marshal request", http.StatusInternalServerError)
		return nil
	}

	httpReq, err := http.NewRequest("POST", "https://space.ai-builders.com/backend/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return nil
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		http.Error(w, "Failed to call AI API: "+err.Error(), http.StatusInternalServerError)
		return nil
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return nil
	}

	if httpResp.StatusCode != http.StatusOK {
		http.Error(w, "AI API error: "+string(respBody), httpResp.StatusCode)
		return nil
	}

	log.Printf("%s[/chat] AI API response received%s", colorYellow, colorReset)

	// Parse response
	var chatResp struct {
		Choices []struct {
			Message struct {
				Content   *string `json:"content"`
				ToolCalls []struct {
					Id       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls,omitempty"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		http.Error(w, "Failed to parse AI response", http.StatusInternalServerError)
		return nil
	}

	if len(chatResp.Choices) == 0 {
		http.Error(w, "No response from AI", http.StatusInternalServerError)
		return nil
	}

	choice := chatResp.Choices[0]

	// If no tool calls, return the content directly
	if len(choice.Message.ToolCalls) == 0 {
		log.Printf("%s%s[/chat] LLM returned final answer (no tool calls)%s", colorBold, colorGreen, colorReset)
		return choice.Message.Content
	}

	// Handle tool calls
	log.Printf("%s[/chat] LLM returned %d tool call(s)%s", colorMagenta, len(choice.Message.ToolCalls), colorReset)

	// Build assistant message with tool_calls
	assistantMsg := map[string]interface{}{
		"role":       "assistant",
		"content":    choice.Message.Content,
		"tool_calls": choice.Message.ToolCalls,
	}
	messages = append(messages, assistantMsg)

	// Execute each tool call and add tool response
	for _, tc := range choice.Message.ToolCalls {
		if tc.Function.Name == "search" {
			log.Printf("%s[/chat] Executing tool:%s %s(%s)", colorMagenta, colorReset, tc.Function.Name, tc.Function.Arguments)

			searchResults := callInternalSearchAPI(tc.Function.Arguments)
			var resultContent string
			if searchResults != nil {
				resultBytes, _ := json.Marshal(searchResults)
				resultContent = string(resultBytes)
				log.Printf("%s[/chat] Search tool executed successfully%s", colorGreen, colorReset)
			} else {
				resultContent = `{"error": "search failed"}`
				log.Printf("%s[/chat] Search tool execution failed%s", colorRed, colorReset)
			}

			// Add tool response message
			toolMsg := map[string]interface{}{
				"role":         "tool",
				"tool_call_id": tc.Id,
				"content":      resultContent,
			}
			messages = append(messages, toolMsg)
		}
	}

	// Make second API call with tool results
	log.Printf("%s[/chat] Sending tool results back to LLM...%s", colorBlue, colorReset)
	return callAIAPI(apiKey, model, messages, tools, w)
}

// Ensure Server implements ServerInterface
var _ ServerInterface = (*Server)(nil)

// callInternalSearchAPI calls the internal /search API endpoint
func callInternalSearchAPI(arguments string) *SearchResponse {
	// Parse arguments to get keywords
	var args struct {
		Keywords []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		log.Printf("%s[/chat] Failed to parse search arguments: %v%s", colorRed, err, colorReset)
		return nil
	}

	log.Printf("%s[/chat] Calling /search API%s with keywords: %v", colorYellow, colorReset, args.Keywords)

	// Build request body
	searchReq := SearchRequest{
		Keywords: args.Keywords,
	}
	reqBody, err := json.Marshal(searchReq)
	if err != nil {
		return nil
	}

	// Call internal /search endpoint
	httpReq, err := http.NewRequest("POST", "http://localhost:8080/search", bytes.NewReader(reqBody))
	if err != nil {
		return nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("%s[/chat] /search API call failed: %v%s", colorRed, err, colorReset)
		return nil
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		log.Printf("%s[/chat] /search API returned status: %d%s", colorRed, httpResp.StatusCode, colorReset)
		return nil
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&searchResp); err != nil {
		log.Printf("%s[/chat] Failed to decode search response: %v%s", colorRed, err, colorReset)
		return nil
	}

	log.Printf("%s[/chat] /search API returned results%s", colorGreen, colorReset)
	return &searchResp
}

// PostSearch implements ServerInterface.
// (POST /search)
func (Server) PostSearch(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	maxResults := 6
	if req.MaxResults != nil && *req.MaxResults > 0 {
		maxResults = *req.MaxResults
	}

	resp, err := CallSearchAPI(req.Keywords, maxResults)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// CallSearchAPI calls the AI Builder search API
func CallSearchAPI(keywords []string, maxResults int) (*SearchResponse, error) {
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("API_KEY not configured")
	}

	if maxResults <= 0 {
		maxResults = 6 // default
	}

	// Use a local struct for the request since remote API expects int, not *int
	searchReq := struct {
		Keywords   []string `json:"keywords"`
		MaxResults int      `json:"max_results,omitempty"`
	}{
		Keywords:   keywords,
		MaxResults: maxResults,
	}

	reqBody, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", "https://space.ai-builders.com/backend/v1/search/", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call search API: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search API error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	var searchResp SearchResponse
	if err := json.Unmarshal(respBody, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	return &searchResp, nil
}
