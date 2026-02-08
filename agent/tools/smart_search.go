package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/dom"
	"github.com/mafredri/cdp/protocol/page"
	"github.com/smallnest/dogclaw/goclaw/internal/logger"
	"go.uber.org/zap"
)

// SmartSearch Smart search tool supporting web search and browser fallback
type SmartSearch struct {
	webTool    *WebTool
	timeout    time.Duration
	webEnabled bool
}

// NewSmartSearch Create smart search tool
func NewSmartSearch(webTool *WebTool, webEnabled bool, timeout int) *SmartSearch {
	var t time.Duration
	if timeout > 0 {
		t = time.Duration(timeout) * time.Second
	} else {
		t = 30 * time.Second
	}

	return &SmartSearch{
		webTool:    webTool,
		timeout:    t,
		webEnabled: webEnabled,
	}
}

// SmartSearchResult Smart search
func (s *SmartSearch) SmartSearchResult(ctx context.Context, params map[string]interface{}) (string, error) {
	query, ok := params["query"].(string)
	if !ok {
		return "Error: query parameter is required", nil
	}

	// Try web_search first
	if s.webEnabled {
		webResults, webErr := s.webTool.WebSearch(ctx, map[string]interface{}{"query": query})
		logger.Info("Web search returned",
			zap.String("query", query),
			zap.Int("result_length", len(webResults)),
			zap.Error(webErr),
			zap.String("result_preview", func() string {
				if len(webResults) > 200 {
					return webResults[:200] + "..."
				}
				return webResults
			}()))

		if webErr == nil && webResults != "" {
			// Check if warning message (no API key) or Mock result
			if !s.isWebSearchResultValid(webResults) {
				// web search unavailable, fallback to browser
				logger.Info("Web search result invalid, falling back to browser search",
					zap.String("reason", s.getInvalidReason(webResults)),
					zap.String("query", query))
				return s.fallbackToBrowser(ctx, query)
			}
			// web search successful
			return webResults, nil
		} else {
			// web search failed, fallback to browser
			logger.Info("Web search failed, falling back to browser search",
				zap.String("query", query),
				zap.Error(webErr))
			return s.fallbackToBrowser(ctx, query)
		}
	}

	// web search not enabled, use browser directly
	logger.Info("Web search not enabled, using browser search", zap.String("query", query))
	return s.fallbackToBrowser(ctx, query)
}

// isWebSearchResultValid Check if web search result is valid
func (s *SmartSearch) isWebSearchResultValid(results string) bool {
	if results == "" {
		return false
	}

	// Check if warning message
	if strings.Contains(results, "[Warning:") {
		return false
	}

	// Check if Mock result
	if strings.Contains(results, "Mock") {
		return false
	}

	// Check if actual content (at least Title or URL)
	if strings.Contains(results, "Title:") || strings.Contains(results, "http") {
		return true
	}

	// Simple check: if result too short and no URL, maybe invalid
	if len(results) < 50 && !strings.Contains(results, "http") {
		return false
	}

	return true
}

// getInvalidReason Get result invalid reason (for debugging)
func (s *SmartSearch) getInvalidReason(results string) string {
	if results == "" {
		return "empty result"
	}
	if strings.Contains(results, "[Warning:") {
		return "contains warning"
	}
	if strings.Contains(results, "Mock") {
		return "contains mock"
	}
	if len(results) < 50 && !strings.Contains(results, "http") {
		return fmt.Sprintf("too short (%d chars)", len(results))
	}
	if !strings.Contains(results, "Title:") && !strings.Contains(results, "http") {
		return "no Title or URL found"
	}
	return "unknown"
}

// fallbackToBrowser Fallback to browser search
func (s *SmartSearch) fallbackToBrowser(ctx context.Context, query string) (string, error) {
	// Get or create browser session
	sessionMgr := GetBrowserSession()
	if !sessionMgr.IsReady() {
		if err := sessionMgr.Start(s.timeout); err != nil {
			return fmt.Sprintf("Browser search failed: failed to start browser session: %v\n\nNote: Please ensure browser tools are properly configured.", err), nil
		}
	}

	// Get CDP client
	client, err := sessionMgr.GetClient()
	if err != nil {
		return fmt.Sprintf("Browser search failed: failed to get browser client: %v", err), nil
	}

	// Build Google search URL
	googleURL := fmt.Sprintf("https://www.google.com/search?q=%s", urlEncode(query))

	logger.Info("Navigating to Google search", zap.String("url", googleURL))

	// Navigate to Google search
	nav, err := client.Page.Navigate(ctx, page.NewNavigateArgs(googleURL))
	if err != nil {
		return fmt.Sprintf("Browser search failed: failed to navigate: %v", err), nil
	}

	// Wait for page load
	domContentLoaded, err := client.Page.DOMContentEventFired(ctx)
	if err != nil {
		logger.Warn("DOMContentEventFired failed", zap.Error(err))
	} else {
		defer domContentLoaded.Close()
		_, _ = domContentLoaded.Recv()
	}

	// Get page content
	doc, err := client.DOM.GetDocument(ctx, nil)
	if err != nil {
		return fmt.Sprintf("Browser search failed: failed to get document: %v", err), nil
	}

	html, err := client.DOM.GetOuterHTML(ctx, &dom.GetOuterHTMLArgs{
		NodeID: &doc.Root.NodeID,
	})
	if err != nil {
		return fmt.Sprintf("Browser search failed: failed to get page content: %v", err), nil
	}

	content := html.OuterHTML

	logger.Info("Page content retrieved", zap.Int("content_length", len(content)), zap.String("frame_id", string(nav.FrameID)))

	// Check if blocked by Google (verify page)
	if len(content) > 0 && (strings.Contains(content, "unusual traffic") ||
		strings.Contains(content, "CAPTCHA") ||
		strings.Contains(content, "verify you are human") ||
		strings.Contains(content, "I'm not a robot")) {
		logger.Warn("Google detected automated traffic, showing CAPTCHA page")
		return fmt.Sprintf("Google Search for: %s\n\n[Blocked by Google: CAPTCHA or anti-bot verification required. The search page shows 'unusual traffic' or 'I'm not a robot'.]\n\nNote: You may need to wait a moment and try again.", query), nil
	}

	// Extract search results
	searchResults := s.extractGoogleSearchResults(content)

	logger.Info("Search results extracted", zap.Int("results_length", len(searchResults)))

	if searchResults == "" {
		// Return partial content for debugging
		preview := content
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		return fmt.Sprintf("Google search completed for: %s\n\nNo results could be extracted. Page preview:\n%s\n\nThe page structure may have changed or search was blocked.\n\nTry using browser_navigate and browser_get_text tools directly.", query, preview), nil
	}

	return fmt.Sprintf("Google Search Results for: %s\n\n%s", query, searchResults), nil
}

// extractGoogleSearchResults Extract search results from Google search page
func (s *SmartSearch) extractGoogleSearchResults(pageText string) string {
	// Convert HTML to plain text
	text := htmlToTextForSearch(pageText)
	lines := strings.Split(text, "\n")

	var results []string
	var currentResult strings.Builder
	resultCount := 0

	// Google search result common patterns:
	// 1. Title line (shorter, meaningful text)
	// 2. URL line (starts with http:// or https://)
	// 3. Description line (longer text)

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Skip empty lines
		if line == "" {
			continue
		}

		// Skip Google UI elements
		if s.isGoogleUIElement(line) {
			continue
		}

		// Detect possible title
		if s.isResultTitle(line) {
			// If existing result, save it
			if currentResult.Len() > 0 {
				result := currentResult.String()
				if s.isValidResult(result) {
					results = append(results, result)
					resultCount++
					if resultCount >= 10 { // Limit to 10 results
						break
					}
				}
				currentResult.Reset()
			}
			currentResult.WriteString(fmt.Sprintf("Title: %s", line))
			continue
		}

		// If building result, add content
		if currentResult.Len() > 0 {
			if s.isURL(line) {
				currentResult.WriteString(fmt.Sprintf("\nURL: %s", line))
			} else if len(line) > 20 {
				currentResult.WriteString(fmt.Sprintf("\nDescription: %s", line))
			}
		}
	}

	// Add last result
	if currentResult.Len() > 0 {
		result := currentResult.String()
		if s.isValidResult(result) {
			results = append(results, result)
		}
	}

	if len(results) == 0 {
		return ""
	}

	return strings.Join(results, "\n\n---\n\n")
}

// isGoogleUIElement Check if Google UI element
func (s *SmartSearch) isGoogleUIElement(line string) bool {
	uiElements := []string{
		"Google", "Search", "Images", "Maps", "News", "Videos",
		"Shopping", "More", "Sign in", "Settings", "Privacy",
		"Terms", "About", "Advertising", "Business", "Cookies",
		"All", "Images", "News", "Videos", "Tools", "SafeSearch",
		"Related searches", "People also ask", "Top stories",
		"Page", "of", "Next", "Previous",
	}

	lowerLine := strings.ToLower(line)
	for _, elem := range uiElements {
		if lowerLine == strings.ToLower(elem) {
			return true
		}
	}

	return false
}

// isResultTitle Check if search result title
func (s *SmartSearch) isResultTitle(line string) bool {
	// Title usually shorter (10-100 chars)
	if len(line) < 5 || len(line) > 120 {
		return false
	}

	// Skip pure URL
	if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
		return false
	}

	// Skip common suffixes
	excludeSuffixes := []string{"... more", "cached", "similar", "translate"}
	for _, suffix := range excludeSuffixes {
		if strings.HasSuffix(strings.ToLower(line), suffix) {
			return false
		}
	}

	// Check if contains meaningful characters
	hasContent := false
	for _, r := range line {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || (r >= 0x4e00 && r <= 0x9fff) {
			hasContent = true
			break
		}
	}

	return hasContent
}

// isURL Check if URL
func (s *SmartSearch) isURL(line string) bool {
	return strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://")
}

// isValidResult Check if result is valid
func (s *SmartSearch) isValidResult(result string) bool {
	// Must contain title
	if !strings.Contains(result, "Title:") {
		return false
	}

	// Preferably contains URL or description
	return strings.Contains(result, "URL:") || strings.Contains(result, "Description:")
}

// GetTool Get smart search tool
func (s *SmartSearch) GetTool() Tool {
	return NewBaseTool(
		"smart_search",
		"Intelligent search that automatically falls back to Google browser search if web search fails or returns no results. Uses Chrome DevTools Protocol.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query to search for",
				},
			},
			"required": []string{"query"},
		},
		s.SmartSearchResult,
	)
}

// urlEncode URL encoding
func urlEncode(s string) string {
	var result strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			result.WriteRune(c)
		} else if c == ' ' {
			result.WriteString("+")
		} else {
			result.WriteString(fmt.Sprintf("%%%02X", c))
		}
	}
	return result.String()
}

// htmlToTextForSearch Convert HTML to plain text (for search result extraction)
func htmlToTextForSearch(html string) string {
	text := ""
	inTag := false
	inScript := false
	inStyle := false
	tagName := ""

	i := 0
	for i < len(html) {
		if html[i] == '<' {
			inTag = true
			tagName = ""
			j := i + 1
			for j < len(html) && html[j] != '>' && html[j] != ' ' {
				tagName += string(html[j])
				j++
			}
			if strings.ToLower(tagName) == "script" {
				inScript = true
			}
			if strings.ToLower(tagName) == "style" {
				inStyle = true
			}
			if strings.ToLower(tagName) == "/script" {
				inScript = false
			}
			if strings.ToLower(tagName) == "/style" {
				inStyle = false
			}
			i = j
			continue
		}

		if html[i] == '>' {
			inTag = false
			i++
			continue
		}

		if !inTag && !inScript && !inStyle {
			text += string(html[i])
		}

		i++
	}

	// Clean extra whitespace
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n")
}
