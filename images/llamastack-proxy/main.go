package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultLlamaStackURL = "http://llamastack.llm.svc.cluster.local:8321"
	defaultProxyPort     = "8322"
	cacheRefreshInterval = 5 * time.Minute
)

var (
	llamaStackURL string
	proxyPort     string
	toolCache     *ToolCache
)

// ToolCache stores tool groups and their tools with caching
type ToolCache struct {
	mu                sync.RWMutex
	toolGroups        []ToolGroupInfo
	toolsByGroup      map[string][]LlamaStackTool
	allTools          []LlamaStackTool
	lastRefresh       time.Time
	enableSmartFilter bool
	explicitGroups    []string
	blacklistedTools  []string // Tools to exclude from all requests
	// Generic keyword extraction from tool descriptions
	groupKeywords map[string][]string
}

type ToolGroupInfo struct {
	Identifier         string `json:"identifier"`
	ProviderID         string `json:"provider_id"`
	ProviderResourceID string `json:"provider_resource_id"`
	Type               string `json:"type"`
}

type ToolGroupsResponse struct {
	Data []ToolGroupInfo `json:"data"`
}

// LlamaStack Tool structures
type ToolParameter struct {
	Name          string      `json:"name"`
	ParameterType string      `json:"parameter_type"`
	Description   string      `json:"description"`
	Required      bool        `json:"required"`
	Default       interface{} `json:"default"`
}

type LlamaStackTool struct {
	Identifier         string          `json:"identifier"`
	ProviderResourceID *string         `json:"provider_resource_id"`
	ProviderID         string          `json:"provider_id"`
	Type               string          `json:"type"`
	ToolGroupID        string          `json:"toolgroup_id"`
	Description        string          `json:"description"`
	Parameters         []ToolParameter `json:"parameters"`
	Metadata           interface{}     `json:"metadata"`
}

type ListToolsResponse struct {
	Data []LlamaStackTool `json:"data"`
}

// OpenAI Tool structures
type OpenAIToolParameter struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

type OpenAIToolParameters struct {
	Type       string                         `json:"type"`
	Properties map[string]OpenAIToolParameter `json:"properties"`
	Required   []string                       `json:"required"`
}

type OpenAIFunction struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Parameters  OpenAIToolParameters `json:"parameters"`
}

type OpenAITool struct {
	Type     string         `json:"type"`
	Function OpenAIFunction `json:"function"`
}

// OpenAI Chat Completion structures
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Tools       []OpenAITool  `json:"tools,omitempty"`
	ToolChoice  interface{}   `json:"tool_choice,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	// Add other fields as needed
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage,omitempty"`
}

type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// LlamaStack tool invocation structures
type ToolInvocationRequest struct {
	ToolName string                 `json:"tool_name"`
	Kwargs   map[string]interface{} `json:"kwargs"`
}

type ToolInvocationResult struct {
	Content      interface{} `json:"content"`
	ErrorMessage *string     `json:"error_message,omitempty"`
	ErrorCode    interface{} `json:"error_code,omitempty"` // Can be string or number
}

// LlamaStack Model structures
type LlamaStackModel struct {
	Identifier         string                 `json:"identifier"`
	ProviderResourceID string                 `json:"provider_resource_id"`
	ProviderID         string                 `json:"provider_id"`
	Type               string                 `json:"type"`
	Metadata           map[string]interface{} `json:"metadata"`
	ModelType          string                 `json:"model_type"`
}

type LlamaStackModelsResponse struct {
	Data []LlamaStackModel `json:"data"`
}

// OpenAI Model structures
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

func init() {
	llamaStackURL = os.Getenv("LLAMASTACK_URL")
	if llamaStackURL == "" {
		llamaStackURL = defaultLlamaStackURL
	}
	proxyPort = os.Getenv("PROXY_PORT")
	if proxyPort == "" {
		proxyPort = defaultProxyPort
	}

	// Initialize tool cache with configuration
	toolCache = &ToolCache{
		toolsByGroup:  make(map[string][]LlamaStackTool),
		groupKeywords: make(map[string][]string),
		enableSmartFilter: os.Getenv("ENABLE_SMART_TOOL_FILTERING") == "true",
	}

	// Parse explicit tool groups if configured
	if groupsFilter := os.Getenv("TOOL_GROUPS_FILTER"); groupsFilter != "" {
		toolCache.explicitGroups = strings.Split(groupsFilter, ",")
		for i := range toolCache.explicitGroups {
			toolCache.explicitGroups[i] = strings.TrimSpace(toolCache.explicitGroups[i])
		}
	}

	// Parse blacklisted tools if configured
	if blacklist := os.Getenv("TOOLS_BLACKLIST"); blacklist != "" {
		toolCache.blacklistedTools = strings.Split(blacklist, ",")
		for i := range toolCache.blacklistedTools {
			toolCache.blacklistedTools[i] = strings.TrimSpace(toolCache.blacklistedTools[i])
		}
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractKeywords extracts meaningful keywords from text (tool descriptions)
func extractKeywords(text string) []string {
	// Common stop words to ignore
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "from": true, "as": true, "is": true, "was": true,
		"are": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true, "did": true,
		"will": true, "would": true, "could": true, "should": true, "may": true,
		"might": true, "must": true, "can": true, "this": true, "that": true,
		"these": true, "those": true, "i": true, "you": true, "he": true, "she": true,
		"it": true, "we": true, "they": true, "what": true, "which": true, "who": true,
		"when": true, "where": true, "why": true, "how": true, "all": true, "each": true,
		"every": true, "both": true, "few": true, "more": true, "most": true, "other": true,
		"some": true, "such": true, "no": true, "nor": true, "not": true, "only": true,
		"own": true, "same": true, "so": true, "than": true, "too": true, "very": true,
		"use": true, "using": true, "used": true, "tool": true, "tools": true,
	}

	// Convert to lowercase and split into words
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_')
	})

	keywords := make([]string, 0)
	seen := make(map[string]bool)

	for _, word := range words {
		// Skip if too short, is a stop word, or already seen
		if len(word) < 3 || stopWords[word] || seen[word] {
			continue
		}

		keywords = append(keywords, word)
		seen[word] = true
	}

	return keywords
}

// initializeToolCache fetches all tool groups and tools on startup
func initializeToolCache() error {
	log.Println("========================================")
	log.Println("Initializing Tool Cache...")
	log.Println("========================================")

	// Fetch tool groups
	if err := toolCache.refreshToolGroups(); err != nil {
		return fmt.Errorf("failed to fetch tool groups: %w", err)
	}

	// Fetch tools for each group
	if err := toolCache.refreshAllTools(); err != nil {
		return fmt.Errorf("failed to fetch tools: %w", err)
	}

	// Build keyword index for smart filtering
	toolCache.buildKeywordIndex()

	toolCache.logCacheSummary()
	return nil
}

// refreshToolGroups fetches available tool groups from LlamaStack
func (tc *ToolCache) refreshToolGroups() error {
	resp, err := http.Get(llamaStackURL + "/v1/toolgroups")
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var groupsResp ToolGroupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&groupsResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	tc.mu.Lock()
	tc.toolGroups = groupsResp.Data
	tc.mu.Unlock()

	log.Printf("✓ Found %d tool groups", len(groupsResp.Data))
	for _, group := range groupsResp.Data {
		log.Printf("  - %s (provider: %s)", group.Identifier, group.ProviderID)
	}

	return nil
}

// refreshAllTools fetches tools for all tool groups
func (tc *ToolCache) refreshAllTools() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	allTools := make([]LlamaStackTool, 0)
	tc.toolsByGroup = make(map[string][]LlamaStackTool)

	log.Println("\nFetching tools for each group:")

	for _, group := range tc.toolGroups {
		url := fmt.Sprintf("%s/v1/tools?toolgroup_id=%s", llamaStackURL, group.Identifier)
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("  ✗ %s: HTTP error: %v", group.Identifier, err)
			continue
		}

		var toolsResp ListToolsResponse
		if err := json.NewDecoder(resp.Body).Decode(&toolsResp); err != nil {
			resp.Body.Close()
			log.Printf("  ✗ %s: Decode error: %v", group.Identifier, err)
			continue
		}
		resp.Body.Close()

		// Filter out blacklisted tools
		filteredTools := make([]LlamaStackTool, 0)
		blacklistedCount := 0
		for _, tool := range toolsResp.Data {
			isBlacklisted := false
			for _, blacklisted := range tc.blacklistedTools {
				if tool.Identifier == blacklisted {
					isBlacklisted = true
					blacklistedCount++
					log.Printf("    ✗ %s - BLACKLISTED", tool.Identifier)
					break
				}
			}
			if !isBlacklisted {
				filteredTools = append(filteredTools, tool)
			}
		}

		tc.toolsByGroup[group.Identifier] = filteredTools
		allTools = append(allTools, filteredTools...)

		log.Printf("  ✓ %s: %d tools (%d blacklisted)", group.Identifier, len(filteredTools), blacklistedCount)
		for _, tool := range filteredTools {
			log.Printf("    • %s - %s", tool.Identifier, truncateString(tool.Description, 60))
		}
	}

	tc.allTools = allTools
	tc.lastRefresh = time.Now()

	return nil
}

// buildKeywordIndex builds a keyword index from tool descriptions for smart filtering
func (tc *ToolCache) buildKeywordIndex() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.groupKeywords = make(map[string][]string)

	log.Println("\nBuilding keyword index from tool descriptions:")

	for groupID, tools := range tc.toolsByGroup {
		// Collect all text from this group's tools
		textParts := []string{groupID} // Include group ID itself

		for _, tool := range tools {
			textParts = append(textParts, tool.Identifier)
			textParts = append(textParts, tool.Description)

			// Also include parameter names and descriptions
			for _, param := range tool.Parameters {
				textParts = append(textParts, param.Name)
				textParts = append(textParts, param.Description)
			}
		}

		// Extract keywords from combined text
		combinedText := strings.Join(textParts, " ")
		keywords := extractKeywords(combinedText)

		tc.groupKeywords[groupID] = keywords

		log.Printf("  %s: %d keywords", groupID, len(keywords))
		if len(keywords) > 0 {
			// Show first 10 keywords
			preview := keywords
			if len(preview) > 10 {
				preview = keywords[:10]
			}
			log.Printf("    Keywords: %s", strings.Join(preview, ", "))
		}
	}
}

// needsRefresh checks if cache needs to be refreshed
func (tc *ToolCache) needsRefresh() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return time.Since(tc.lastRefresh) > cacheRefreshInterval
}

// refreshIfNeeded refreshes cache if it's stale
func (tc *ToolCache) refreshIfNeeded() {
	if tc.needsRefresh() {
		log.Println("Tool cache is stale, refreshing...")
		if err := tc.refreshToolGroups(); err != nil {
			log.Printf("Warning: Failed to refresh tool groups: %v", err)
			return
		}
		if err := tc.refreshAllTools(); err != nil {
			log.Printf("Warning: Failed to refresh tools: %v", err)
			return
		}
		tc.buildKeywordIndex()
	}
}

// logCacheSummary prints a detailed summary of cached tools
func (tc *ToolCache) logCacheSummary() {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	log.Println("\n========================================")
	log.Println("Tool Cache Summary")
	log.Println("========================================")
	log.Printf("Total Tool Groups: %d", len(tc.toolGroups))
	log.Printf("Total Tools: %d", len(tc.allTools))
	log.Printf("Last Refresh: %s", tc.lastRefresh.Format(time.RFC3339))
	log.Printf("Cache TTL: %v", cacheRefreshInterval)

	log.Println("\n--- Configuration ---")
	log.Printf("Smart Filtering: %v", tc.enableSmartFilter)
	if len(tc.explicitGroups) > 0 {
		log.Printf("Explicit Groups Filter: %v", tc.explicitGroups)
	} else {
		log.Println("Explicit Groups Filter: <none>")
	}
	if len(tc.blacklistedTools) > 0 {
		log.Printf("Blacklisted Tools: %v", tc.blacklistedTools)
	} else {
		log.Println("Blacklisted Tools: <none>")
	}

	log.Println("\n--- Tools by Group ---")
	for _, group := range tc.toolGroups {
		tools := tc.toolsByGroup[group.Identifier]
		log.Printf("\n%s (%d tools):", group.Identifier, len(tools))
		for _, tool := range tools {
			paramCount := len(tool.Parameters)
			log.Printf("  • %s (%d params) - %s",
				tool.Identifier,
				paramCount,
				truncateString(tool.Description, 70))
		}

		// Show keywords for this group
		if keywords, ok := tc.groupKeywords[group.Identifier]; ok && len(keywords) > 0 {
			preview := keywords
			if len(preview) > 8 {
				preview = keywords[:8]
			}
			log.Printf("  Keywords: %s", strings.Join(preview, ", "))
		}
	}

	// Calculate average tool overhead
	if len(tc.allTools) > 0 {
		totalParams := 0
		for _, tool := range tc.allTools {
			totalParams += len(tool.Parameters)
		}
		avgParams := float64(totalParams) / float64(len(tc.allTools))

		log.Println("\n--- Overhead Estimates ---")
		log.Printf("Average parameters per tool: %.1f", avgParams)
		log.Printf("Estimated tokens (all tools): ~%d", len(tc.allTools)*200)

		if len(tc.toolGroups) > 0 {
			avgToolsPerGroup := len(tc.allTools) / len(tc.toolGroups)
			log.Printf("Estimated tokens (1 group avg): ~%d", avgToolsPerGroup*200)
		}
	}

	log.Println("========================================\n")
}

// scoreToolGroupRelevance calculates relevance score between user message and tool group
func (tc *ToolCache) scoreToolGroupRelevance(userMessage string, groupID string) int {
	// Extract keywords from user message
	userKeywords := extractKeywords(userMessage)
	if len(userKeywords) == 0 {
		return 0
	}

	// Get keywords for this tool group
	groupKeywords, exists := tc.groupKeywords[groupID]
	if !exists || len(groupKeywords) == 0 {
		return 0
	}

	// Count matching keywords
	score := 0
	for _, userKw := range userKeywords {
		for _, groupKw := range groupKeywords {
			// Exact match
			if userKw == groupKw {
				score += 10
				continue
			}
			// Partial match (one contains the other)
			if strings.Contains(userKw, groupKw) || strings.Contains(groupKw, userKw) {
				score += 5
			}
		}
	}

	return score
}

// selectRelevantToolGroups selects tool groups based on user message (completely generic)
func (tc *ToolCache) selectRelevantToolGroups(userMessage string) []string {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	// If explicit groups are configured, use those
	if len(tc.explicitGroups) > 0 {
		log.Printf("Using explicit groups filter: %v", tc.explicitGroups)
		return tc.explicitGroups
	}

	// If smart filtering is disabled, return all groups
	if !tc.enableSmartFilter {
		groups := make([]string, len(tc.toolGroups))
		for i, g := range tc.toolGroups {
			groups[i] = g.Identifier
		}
		log.Printf("Smart filtering disabled, using all %d groups", len(groups))
		return groups
	}

	// Smart filtering: score each tool group based on keyword matching
	type groupScore struct {
		groupID string
		score   int
	}

	scores := make([]groupScore, 0, len(tc.toolGroups))

	for _, group := range tc.toolGroups {
		score := tc.scoreToolGroupRelevance(userMessage, group.Identifier)
		scores = append(scores, groupScore{
			groupID: group.Identifier,
			score:   score,
		})
	}

	// Sort by score (simple bubble sort for small lists)
	for i := 0; i < len(scores); i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].score > scores[i].score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	// Select groups with score > 0, or top 1-2 groups if none match
	selectedGroups := make([]string, 0)
	for _, gs := range scores {
		if gs.score > 0 {
			selectedGroups = append(selectedGroups, gs.groupID)
			log.Printf("Selected group %s (score: %d)", gs.groupID, gs.score)
		}
	}

	// If no groups matched, use the top scored group as fallback
	if len(selectedGroups) == 0 && len(scores) > 0 {
		selectedGroups = []string{scores[0].groupID}
		log.Printf("No keyword matches, using fallback group: %s", scores[0].groupID)
	}

	// If still nothing (shouldn't happen), use all groups
	if len(selectedGroups) == 0 {
		for _, g := range tc.toolGroups {
			selectedGroups = append(selectedGroups, g.Identifier)
		}
		log.Printf("Fallback: using all %d groups", len(selectedGroups))
	}

	return selectedGroups
}

// getToolsForGroups retrieves tools for specified groups
func (tc *ToolCache) getToolsForGroups(groupIDs []string) []LlamaStackTool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	tools := make([]LlamaStackTool, 0)
	for _, groupID := range groupIDs {
		if groupTools, exists := tc.toolsByGroup[groupID]; exists {
			tools = append(tools, groupTools...)
		}
	}

	return tools
}

// getToolsForRequest intelligently selects tools based on user message
func getToolsForRequest(userMessage string) ([]OpenAITool, error) {
	// Refresh cache if needed
	toolCache.refreshIfNeeded()

	// Select relevant tool groups
	selectedGroups := toolCache.selectRelevantToolGroups(userMessage)

	log.Printf("Selected tool groups for request: %v", selectedGroups)

	// Get tools for selected groups
	tools := toolCache.getToolsForGroups(selectedGroups)

	log.Printf("Retrieved %d tools from %d groups", len(tools), len(selectedGroups))

	return convertToOpenAITools(tools), nil
}

func main() {
	log.Printf("Starting LlamaStack proxy on port %s", proxyPort)
	log.Printf("Forwarding to LlamaStack at %s", llamaStackURL)

	// Initialize OpenTelemetry
	shutdown, err := initTelemetry()
	if err != nil {
		log.Printf("WARNING: Failed to initialize telemetry: %v", err)
		log.Println("Continuing without telemetry")
	} else {
		defer func() {
			if err := shutdown(context.Background()); err != nil {
				log.Printf("Error shutting down telemetry: %v", err)
			}
		}()
	}

	// Initialize tool cache on startup
	if err := initializeToolCache(); err != nil {
		log.Printf("WARNING: Failed to initialize tool cache: %v", err)
		log.Println("Continuing without tool cache - tools will be fetched on demand")
	}

	// Create a new ServeMux for registering handlers
	mux := http.NewServeMux()

	// Chat completions with tool injection
	mux.HandleFunc("/openai/v1/chat/completions", handleChatCompletion)
	mux.HandleFunc("/v1/chat/completions", handleChatCompletion)

	// Models endpoint - forward to LlamaStack's OpenAI-compatible endpoint
	mux.HandleFunc("/openai/v1/models", handleModels)
	mux.HandleFunc("/v1/models", handleModels)

	// Proxy other OpenAI endpoints directly
	mux.HandleFunc("/openai/v1/embeddings", handleGenericProxy)
	mux.HandleFunc("/v1/embeddings", handleGenericProxy)

	// Health check
	mux.HandleFunc("/health", handleHealth)

	// Admin endpoint to view cache status
	mux.HandleFunc("/admin/cache/status", handleCacheStatus)
	mux.HandleFunc("/admin/cache/refresh", handleCacheRefresh)

	// Catch-all for other endpoints
	mux.HandleFunc("/", handleGenericProxy)

	// Wrap the mux with OpenTelemetry instrumentation
	handler := otelhttp.NewHandler(mux, "llamastack-proxy")

	addr := "0.0.0.0:" + proxyPort
	log.Printf("Listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func handleCacheStatus(w http.ResponseWriter, r *http.Request) {
	toolCache.mu.RLock()
	defer toolCache.mu.RUnlock()

	status := map[string]interface{}{
		"tool_groups":        len(toolCache.toolGroups),
		"total_tools":        len(toolCache.allTools),
		"last_refresh":       toolCache.lastRefresh.Format(time.RFC3339),
		"cache_age_seconds":  time.Since(toolCache.lastRefresh).Seconds(),
		"smart_filter":       toolCache.enableSmartFilter,
		"explicit_groups":    toolCache.explicitGroups,
		"blacklisted_tools":  toolCache.blacklistedTools,
		"groups":             toolCache.toolGroups,
		"tools_per_group":    make(map[string]int),
		"keywords_per_group": make(map[string]int),
	}

	for groupID, tools := range toolCache.toolsByGroup {
		status["tools_per_group"].(map[string]int)[groupID] = len(tools)
	}

	for groupID, keywords := range toolCache.groupKeywords {
		status["keywords_per_group"].(map[string]int)[groupID] = len(keywords)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func handleCacheRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("Manual cache refresh requested")

	if err := toolCache.refreshToolGroups(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to refresh tool groups: %v", err), http.StatusInternalServerError)
		return
	}

	if err := toolCache.refreshAllTools(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to refresh tools: %v", err), http.StatusInternalServerError)
		return
	}

	toolCache.buildKeywordIndex()
	toolCache.logCacheSummary()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Cache refreshed successfully",
	})
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	// Forward to LlamaStack's OpenAI-compatible models endpoint
	targetURL := llamaStackURL + "/v1/openai/v1/models"
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	log.Printf("Proxying %s %s to %s", r.Method, r.URL.Path, targetURL)

	proxyReq, err := http.NewRequest(r.Method, targetURL, nil)
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// Forward request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Printf("Error forwarding request: %v", err)
		http.Error(w, "Failed to forward request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	io.Copy(w, resp.Body)
}

func handleGenericProxy(w http.ResponseWriter, r *http.Request) {
	// Simple passthrough proxy for non-chat endpoints
	targetURL := llamaStackURL + r.URL.Path
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	log.Printf("Proxying %s %s to %s", r.Method, r.URL.Path, targetURL)

	// Read request body if present
	var body []byte
	var err error
	if r.Body != nil {
		body, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		r.Body.Close()
	}

	// Create proxy request
	proxyReq, err := http.NewRequest(r.Method, targetURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// Forward request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Printf("Error forwarding request: %v", err)
		http.Error(w, "Failed to forward request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	io.Copy(w, resp.Body)
}

func handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse request
	var chatReq ChatCompletionRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		log.Printf("Error parsing request: %v", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	log.Printf("Received chat completion request for model: %s", chatReq.Model)

	// Debug: log the raw request body if it contains tool messages
	for _, msg := range chatReq.Messages {
		if msg.Role == "tool" {
			log.Printf("DEBUG: Raw request body (has tool messages): %s", string(body))
			break
		}
	}

	// Fix tool messages - ensure all tool messages have content
	// (Open WebUI sometimes sends tool messages without content which breaks LlamaStack)
	// When we find empty content, re-execute the tool server-side
	for i := range chatReq.Messages {
		if chatReq.Messages[i].Role == "tool" && chatReq.Messages[i].Content == "" {
			log.Printf("WARNING: Tool message %d has empty content, attempting to re-execute tool", i)

			// Find the corresponding tool call in previous messages
			toolCallID := chatReq.Messages[i].ToolCallID
			var foundToolCall *ToolCall

			for j := i - 1; j >= 0; j-- {
				if chatReq.Messages[j].Role == "assistant" && len(chatReq.Messages[j].ToolCalls) > 0 {
					for _, tc := range chatReq.Messages[j].ToolCalls {
						if tc.ID == toolCallID {
							foundToolCall = &tc
							break
						}
					}
					if foundToolCall != nil {
						break
					}
				}
			}

			if foundToolCall != nil {
				log.Printf("Re-executing tool: %s with args: %s", foundToolCall.Function.Name, foundToolCall.Function.Arguments)
				result, err := executeToolCall(r.Context(), *foundToolCall)
				if err != nil {
					log.Printf("Error re-executing tool: %v", err)
					chatReq.Messages[i].Content = fmt.Sprintf("Error executing tool: %v", err)
				} else {
					chatReq.Messages[i].Content = result
					log.Printf("Successfully re-executed tool, result length: %d bytes. Content preview: %s", len(result), truncateString(result, 200))
				}
			} else {
				log.Printf("Could not find tool call for tool_call_id: %s", toolCallID)
				chatReq.Messages[i].Content = "Tool executed successfully (no content provided by client)"
			}
		}
	}

	// Fetch and inject tools if not already present
	if len(chatReq.Tools) == 0 {
		// Get last user message for context-aware tool selection
		var lastUserMessage string
		for i := len(chatReq.Messages) - 1; i >= 0; i-- {
			if chatReq.Messages[i].Role == "user" {
				lastUserMessage = chatReq.Messages[i].Content
				break
			}
		}

		tools, err := getToolsForRequest(lastUserMessage)
		if err != nil {
			log.Printf("Warning: Failed to fetch tools: %v", err)
			// Continue without tools rather than failing
		} else {
			chatReq.Tools = tools
			log.Printf("Injected %d tools into request", len(tools))
		}
	} else {
		log.Printf("Request already has %d tools, skipping injection", len(chatReq.Tools))
	}

	// Fix for Qwen3/models with tools: ensure all messages have content field
	// Some templates fail when content is omitted from JSON (omitempty behavior)
	if len(chatReq.Tools) > 0 {
		for i := range chatReq.Messages {
			if chatReq.Messages[i].Content == "" {
				// Use a space instead of empty string to prevent omitempty from removing the field
				chatReq.Messages[i].Content = " "
			}
		}
	}

	// Handle streaming separately
	if chatReq.Stream {
		handleStreamingRequest(w, r, chatReq)
		return
	}

	// Non-streaming: handle tool execution loop
	maxIterations := 10
	for i := 0; i < maxIterations; i++ {
		resp, err := forwardToLlamaStack(r.Context(), chatReq)
		if err != nil {
			log.Printf("Error forwarding to LlamaStack: %v", err)
			http.Error(w, "Failed to process request", http.StatusInternalServerError)
			return
		}

		// Check if model wants to call tools
		if len(resp.Choices) > 0 && resp.Choices[0].FinishReason == "tool_calls" {
			toolCalls := resp.Choices[0].Message.ToolCalls
			if len(toolCalls) == 0 {
				// No tool calls despite finish reason, return response
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
				return
			}

			log.Printf("Model requested %d tool call(s)", len(toolCalls))

			// Add assistant message with tool calls
			chatReq.Messages = append(chatReq.Messages, resp.Choices[0].Message)

			// Execute tools and add results
			for _, toolCall := range toolCalls {
				result, err := executeToolCall(r.Context(), toolCall)
				if err != nil {
					log.Printf("Error executing tool %s: %v", toolCall.Function.Name, err)
					result = fmt.Sprintf("Error: %v", err)
				}

				// Ensure content is never empty (LlamaStack requires it)
				if result == "" {
					result = "Tool executed successfully with no output"
				}

				log.Printf("Tool %s returned: %s", toolCall.Function.Name, truncateString(result, 100))

				// Add tool result message
				chatReq.Messages = append(chatReq.Messages, ChatMessage{
					Role:       "tool",
					Content:    result,
					ToolCallID: toolCall.ID,
					Name:       toolCall.Function.Name,
				})
			}

			// Continue loop to get final response
			continue
		}

		// No more tool calls, return final response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Max iterations reached
	http.Error(w, "Maximum tool execution iterations reached", http.StatusInternalServerError)
}

func handleStreamingRequest(w http.ResponseWriter, r *http.Request, chatReq ChatCompletionRequest) {
	// For streaming, we forward directly without tool execution
	// (Tool execution in streaming mode is more complex and would require SSE parsing)
	log.Printf("Streaming request - forwarding directly")

	reqBody, _ := json.Marshal(chatReq)
	proxyReq, err := http.NewRequest("POST", llamaStackURL+"/v1/openai/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}
	proxyReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Printf("Error forwarding streaming request: %v", err)
		http.Error(w, "Failed to forward request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Stream response
	io.Copy(w, resp.Body)
}

func convertToOpenAITools(llamaTools []LlamaStackTool) []OpenAITool {
	openaiTools := make([]OpenAITool, 0, len(llamaTools))

	for _, tool := range llamaTools {
		properties := make(map[string]OpenAIToolParameter)
		required := make([]string, 0)

		for _, param := range tool.Parameters {
			properties[param.Name] = OpenAIToolParameter{
				Type:        mapParameterType(param.ParameterType),
				Description: param.Description,
			}
			if param.Required {
				required = append(required, param.Name)
			}
		}

		openaiTools = append(openaiTools, OpenAITool{
			Type: "function",
			Function: OpenAIFunction{
				Name:        tool.Identifier,
				Description: tool.Description,
				Parameters: OpenAIToolParameters{
					Type:       "object",
					Properties: properties,
					Required:   required,
				},
			},
		})
	}

	return openaiTools
}

func mapParameterType(llamaType string) string {
	// Map LlamaStack types to JSON Schema types
	switch strings.ToLower(llamaType) {
	case "string", "str":
		return "string"
	case "integer", "int":
		return "integer"
	case "number", "float":
		return "number"
	case "boolean", "bool":
		return "boolean"
	case "array", "list":
		return "array"
	case "object", "dict":
		return "object"
	default:
		return "string" // Default to string
	}
}

func forwardToLlamaStack(ctx context.Context, chatReq ChatCompletionRequest) (*ChatCompletionResponse, error) {
	tracer := otel.Tracer("llamastack-proxy")
	ctx, span := tracer.Start(ctx, "forwardToLlamaStack",
		trace.WithAttributes(
			attribute.String("model", chatReq.Model),
			attribute.Int("message_count", len(chatReq.Messages)),
			attribute.Int("tools_count", len(chatReq.Tools)),
			attribute.Bool("stream", chatReq.Stream),
		),
	)
	defer span.End()

	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal request")
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", llamaStackURL+"/v1/openai/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create request")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Use otelhttp transport to propagate trace context
	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to forward request")
		return nil, fmt.Errorf("failed to forward request: %w", err)
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("LlamaStack returned status %d: %s", resp.StatusCode, string(body))
		span.RecordError(err)
		span.SetStatus(codes.Error, "llamastack error")
		return nil, err
	}

	var chatResp ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode response")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	span.SetStatus(codes.Ok, "request successful")
	if len(chatResp.Choices) > 0 {
		span.SetAttributes(
			attribute.String("finish_reason", chatResp.Choices[0].FinishReason),
			attribute.Int("tool_calls", len(chatResp.Choices[0].Message.ToolCalls)),
		)
	}

	return &chatResp, nil
}

func executeToolCall(ctx context.Context, toolCall ToolCall) (string, error) {
	tracer := otel.Tracer("llamastack-proxy")
	ctx, span := tracer.Start(ctx, "executeToolCall",
		trace.WithAttributes(
			attribute.String("tool.name", toolCall.Function.Name),
			attribute.String("tool.call_id", toolCall.ID),
		),
	)
	defer span.End()

	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to parse tool arguments")
		return "", fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	// Call LlamaStack tool runtime
	invocationReq := ToolInvocationRequest{
		ToolName: toolCall.Function.Name,
		Kwargs:   args,
	}

	reqBody, err := json.Marshal(invocationReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal invocation request")
		return "", fmt.Errorf("failed to marshal invocation request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", llamaStackURL+"/v1/tool-runtime/invoke", bytes.NewReader(reqBody))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create request")
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Use otelhttp transport to propagate trace context
	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to invoke tool")
		return "", fmt.Errorf("failed to invoke tool: %w", err)
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("tool invocation failed with status %d: %s", resp.StatusCode, string(body))
		span.RecordError(err)
		span.SetStatus(codes.Error, "tool invocation failed")
		return "", err
	}

	// Read response body for debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to read tool response")
		return "", fmt.Errorf("failed to read tool response: %w", err)
	}

	var result ToolInvocationResult
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		log.Printf("Failed to decode tool result. Raw response: %s", string(bodyBytes))
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode tool result")
		return "", fmt.Errorf("failed to decode tool result: %w", err)
	}

	if result.ErrorMessage != nil {
		log.Printf("Tool execution error: %s (code: %v)", *result.ErrorMessage, result.ErrorCode)
		err := fmt.Errorf("tool error: %s", *result.ErrorMessage)
		span.RecordError(err)
		span.SetStatus(codes.Error, "tool execution error")
		return "", err
	}

	// Convert result to string
	resultJSON, err := json.Marshal(result.Content)
	if err != nil {
		return fmt.Sprintf("%v", result.Content), nil
	}

	span.SetStatus(codes.Ok, "tool executed successfully")
	span.SetAttributes(attribute.Int("tool.response_size", len(resultJSON)))
	return string(resultJSON), nil
}
