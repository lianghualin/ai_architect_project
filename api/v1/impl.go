package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
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

	// Default tool definitions
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

	readPageTool := map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "read_page",
			"description": "Fetch a webpage URL and extract the main text content. Use this when you need to read the content of a specific webpage.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The URL of the webpage to read",
					},
				},
				"required": []string{"url"},
			},
		},
	}

	runCommandTool := map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "run_command",
			"description": "Run a shell command on the system. Only whitelisted commands are allowed: ls, cd. Use this to list files or check directories.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The shell command to execute (e.g., 'ls -la', 'ls /tmp')",
					},
				},
				"required": []string{"command"},
			},
		},
	}

	// Build initial messages
	messages := []interface{}{
		map[string]string{"role": "user", "content": req.Message},
	}

	// First API call with all tools
	tools := []interface{}{searchTool, readPageTool, runCommandTool}
	log.Printf("%s[/chat] Tools configured:%s search, read_page, run_command", colorMagenta, colorReset)
	finalContent := callAIAPI(apiKey, model, messages, tools, w)
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
	log.Printf("%s[/chat] Calling AI API%s (model: %s, messages: %d, tools: %d)...", colorYellow, colorReset, model, len(messages), len(tools))

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
		log.Printf("%s[/chat] Executing tool:%s %s(%s)", colorMagenta, colorReset, tc.Function.Name, tc.Function.Arguments)

		var resultContent string

		switch tc.Function.Name {
		case "search":
			searchResults := callInternalSearchAPI(tc.Function.Arguments)
			if searchResults != nil {
				resultBytes, _ := json.Marshal(searchResults)
				resultContent = string(resultBytes)
				log.Printf("%s[/chat] Search tool executed successfully%s", colorGreen, colorReset)
			} else {
				resultContent = `{"error": "search failed"}`
				log.Printf("%s[/chat] Search tool execution failed%s", colorRed, colorReset)
			}

		case "read_page":
			pageContent := callInternalPageReaderAPI(tc.Function.Arguments)
			if pageContent != nil {
				resultBytes, _ := json.Marshal(pageContent)
				resultContent = string(resultBytes)
				log.Printf("%s[/chat] Read page tool executed successfully%s", colorGreen, colorReset)
			} else {
				resultContent = `{"error": "read_page failed"}`
				log.Printf("%s[/chat] Read page tool execution failed%s", colorRed, colorReset)
			}

		case "run_command":
			cmdResult := callInternalRunCommandAPI(tc.Function.Arguments)
			if cmdResult != nil {
				resultBytes, _ := json.Marshal(cmdResult)
				resultContent = string(resultBytes)
				log.Printf("%s[/chat] Run command tool executed successfully%s", colorGreen, colorReset)
			} else {
				resultContent = `{"error": "run_command failed"}`
				log.Printf("%s[/chat] Run command tool execution failed%s", colorRed, colorReset)
			}

		default:
			resultContent = fmt.Sprintf(`{"error": "unknown tool: %s"}`, tc.Function.Name)
			log.Printf("%s[/chat] Unknown tool: %s%s", colorRed, tc.Function.Name, colorReset)
		}

		// Add tool response message
		toolMsg := map[string]interface{}{
			"role":         "tool",
			"tool_call_id": tc.Id,
			"content":      resultContent,
		}
		messages = append(messages, toolMsg)
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

// callInternalPageReaderAPI calls the internal /page_reader API endpoint
func callInternalPageReaderAPI(arguments string) *PageReaderResponse {
	// Parse arguments to get url
	var args struct {
		Url string `json:"url"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		log.Printf("%s[/chat] Failed to parse read_page arguments: %v%s", colorRed, err, colorReset)
		return nil
	}

	log.Printf("%s[/chat] Calling /page_reader API%s with url: %s", colorYellow, colorReset, args.Url)

	// Build request body
	pageReq := PageReaderRequest{
		Url: args.Url,
	}
	reqBody, err := json.Marshal(pageReq)
	if err != nil {
		return nil
	}

	// Call internal /page_reader endpoint
	httpReq, err := http.NewRequest("POST", "http://localhost:8080/page_reader", bytes.NewReader(reqBody))
	if err != nil {
		return nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("%s[/chat] /page_reader API call failed: %v%s", colorRed, err, colorReset)
		return nil
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		log.Printf("%s[/chat] /page_reader API returned status: %d%s", colorRed, httpResp.StatusCode, colorReset)
		return nil
	}

	var pageResp PageReaderResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&pageResp); err != nil {
		log.Printf("%s[/chat] Failed to decode page_reader response: %v%s", colorRed, err, colorReset)
		return nil
	}

	log.Printf("%s[/chat] /page_reader API returned results%s", colorGreen, colorReset)
	return &pageResp
}

// callInternalRunCommandAPI calls the internal /run_command API endpoint
func callInternalRunCommandAPI(arguments string) *RunCommandResponse {
	// Parse arguments to get command
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		log.Printf("%s[/chat] Failed to parse run_command arguments: %v%s", colorRed, err, colorReset)
		return nil
	}

	log.Printf("%s[/chat] Calling /run_command API%s with command: %s", colorYellow, colorReset, args.Command)

	// Build request body
	cmdReq := RunCommandRequest{
		Command: args.Command,
	}
	reqBody, err := json.Marshal(cmdReq)
	if err != nil {
		return nil
	}

	// Call internal /run_command endpoint
	httpReq, err := http.NewRequest("POST", "http://localhost:8080/run_command", bytes.NewReader(reqBody))
	if err != nil {
		return nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("%s[/chat] /run_command API call failed: %v%s", colorRed, err, colorReset)
		return nil
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		log.Printf("%s[/chat] /run_command API returned status: %d%s", colorRed, httpResp.StatusCode, colorReset)
		return nil
	}

	var cmdResp RunCommandResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&cmdResp); err != nil {
		log.Printf("%s[/chat] Failed to decode run_command response: %v%s", colorRed, err, colorReset)
		return nil
	}

	log.Printf("%s[/chat] /run_command API returned results%s", colorGreen, colorReset)
	return &cmdResp
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

// PostPageReader implements ServerInterface.
// (POST /page_reader)
func (Server) PostPageReader(w http.ResponseWriter, r *http.Request) {
	var req PageReaderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	content, err := CallReadPage(req.Url)

	resp := PageReaderResponse{
		Url: &req.Url,
	}

	if err != nil {
		errMsg := err.Error()
		resp.Error = &errMsg
	} else {
		resp.Content = &content
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// CallReadPage fetches a URL and extracts plain text from HTML
func CallReadPage(url string) (string, error) {
	// Fetch the URL
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; PageReader/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	html := string(body)

	// Strip script tags and content
	scriptRe := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = scriptRe.ReplaceAllString(html, "")

	// Strip style tags and content
	styleRe := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = styleRe.ReplaceAllString(html, "")

	// Strip all HTML tags
	tagRe := regexp.MustCompile(`<[^>]*>`)
	text := tagRe.ReplaceAllString(html, "")

	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	// Normalize whitespace: replace multiple spaces/newlines with single space
	spaceRe := regexp.MustCompile(`\s+`)
	text = spaceRe.ReplaceAllString(text, " ")

	// Trim leading/trailing whitespace
	text = strings.TrimSpace(text)

	return text, nil
}

// Whitelisted commands for run_command
var allowedCommands = map[string]bool{
	"ls": true,
	"cd": true,
}

// PostRunCommand implements ServerInterface.
// (POST /run_command)
func (Server) PostRunCommand(w http.ResponseWriter, r *http.Request) {
	var req RunCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	output, err := CallRunCommand(req.Command)

	resp := RunCommandResponse{
		Command: &req.Command,
	}

	if err != nil {
		errMsg := err.Error()
		resp.Error = &errMsg
	} else {
		resp.Output = &output
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// CallRunCommand executes a whitelisted shell command
func CallRunCommand(command string) (string, error) {
	// Parse command to get the base command
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	baseCmd := parts[0]

	// Check whitelist
	if !allowedCommands[baseCmd] {
		return "", fmt.Errorf("command not allowed: %s (allowed: ls, cd)", baseCmd)
	}

	// Execute command
	var cmd *exec.Cmd
	if len(parts) == 1 {
		cmd = exec.Command(baseCmd)
	} else {
		cmd = exec.Command(baseCmd, parts[1:]...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w - %s", err, string(output))
	}

	return string(output), nil
}
