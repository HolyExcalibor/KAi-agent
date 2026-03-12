// Package tools defines the built-in tools available to the Kai agent.
package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// Tool is a callable function the agent can invoke.
type Tool interface {
	// Definition returns the OpenAI function definition for this tool.
	Definition() openai.Tool
	// Execute runs the tool with the given JSON arguments and returns a result string.
	Execute(argsJSON string) (string, error)
}

// Registry holds all registered tools keyed by name.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a registry pre-loaded with all built-in tools.
func NewRegistry() *Registry {
	r := &Registry{tools: make(map[string]Tool)}
	r.Register(&WebSearchTool{})
	r.Register(&CalculatorTool{})
	r.Register(&DateTimeTool{})
	r.Register(&HTTPFetchTool{})
	return r
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.tools[t.Definition().Function.Name] = t
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Definitions returns all OpenAI tool definitions for use in chat completions.
func (r *Registry) Definitions() []openai.Tool {
	defs := make([]openai.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	return defs
}

// ---------------------------------------------------------------------------
// WebSearchTool — uses DuckDuckGo Instant Answer API (no key required)
// ---------------------------------------------------------------------------

// WebSearchTool performs a web search using DuckDuckGo's Instant Answer API.
type WebSearchTool struct{}

func (w *WebSearchTool) Definition() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "web_search",
			Description: "Search the web for current information. Returns a summary from DuckDuckGo.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "The search query"
					}
				},
				"required": ["query"]
			}`),
		},
	}
}

func (w *WebSearchTool) Execute(argsJSON string) (string, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("web_search: parse args: %w", err)
	}

	endpoint := "https://api.duckduckgo.com/?q=" + url.QueryEscape(args.Query) + "&format=json&no_html=1&skip_disambig=1"
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return "", fmt.Errorf("web_search: http get: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		AbstractText string `json:"AbstractText"`
		AbstractURL  string `json:"AbstractURL"`
		Answer       string `json:"Answer"`
		RelatedTopics []struct {
			Text string `json:"Text"`
		} `json:"RelatedTopics"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("web_search: parse response: %w", err)
	}

	var sb strings.Builder
	if result.Answer != "" {
		sb.WriteString("Answer: " + result.Answer + "\n")
	}
	if result.AbstractText != "" {
		sb.WriteString("Summary: " + result.AbstractText + "\n")
		if result.AbstractURL != "" {
			sb.WriteString("Source: " + result.AbstractURL + "\n")
		}
	}
	if sb.Len() == 0 {
		// Fall back to related topics
		for i, t := range result.RelatedTopics {
			if i >= 3 {
				break
			}
			sb.WriteString("- " + t.Text + "\n")
		}
	}
	if sb.Len() == 0 {
		return "No results found for: " + args.Query, nil
	}
	return sb.String(), nil
}

// ---------------------------------------------------------------------------
// CalculatorTool — evaluates simple math expressions
// ---------------------------------------------------------------------------

// CalculatorTool evaluates basic arithmetic expressions.
type CalculatorTool struct{}

func (c *CalculatorTool) Definition() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "calculator",
			Description: "Evaluate a mathematical expression. Supports +, -, *, /, ^, sqrt, abs.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"expression": {
						"type": "string",
						"description": "A math expression, e.g. '2 + 2', 'sqrt(144)', '3^4'"
					}
				},
				"required": ["expression"]
			}`),
		},
	}
}

func (c *CalculatorTool) Execute(argsJSON string) (string, error) {
	var args struct {
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("calculator: parse args: %w", err)
	}
	result, err := evalExpr(strings.TrimSpace(args.Expression))
	if err != nil {
		return "", fmt.Errorf("calculator: %w", err)
	}
	return strconv.FormatFloat(result, 'f', -1, 64), nil
}

// evalExpr is a minimal recursive-descent expression evaluator.
func evalExpr(expr string) (float64, error) {
	expr = strings.ReplaceAll(expr, " ", "")

	// Handle sqrt(x)
	if strings.HasPrefix(expr, "sqrt(") && strings.HasSuffix(expr, ")") {
		inner := expr[5 : len(expr)-1]
		v, err := evalExpr(inner)
		if err != nil {
			return 0, err
		}
		return math.Sqrt(v), nil
	}
	// Handle abs(x)
	if strings.HasPrefix(expr, "abs(") && strings.HasSuffix(expr, ")") {
		inner := expr[4 : len(expr)-1]
		v, err := evalExpr(inner)
		if err != nil {
			return 0, err
		}
		return math.Abs(v), nil
	}

	// Find lowest-precedence operator outside parentheses
	depth := 0
	for i := len(expr) - 1; i >= 0; i-- {
		switch expr[i] {
		case ')':
			depth++
		case '(':
			depth--
		case '+', '-':
			if depth == 0 && i > 0 {
				left, err := evalExpr(expr[:i])
				if err != nil {
					return 0, err
				}
				right, err := evalExpr(expr[i+1:])
				if err != nil {
					return 0, err
				}
				if expr[i] == '+' {
					return left + right, nil
				}
				return left - right, nil
			}
		}
	}
	for i := len(expr) - 1; i >= 0; i-- {
		switch expr[i] {
		case ')':
			depth++
		case '(':
			depth--
		case '*', '/':
			if depth == 0 {
				left, err := evalExpr(expr[:i])
				if err != nil {
					return 0, err
				}
				right, err := evalExpr(expr[i+1:])
				if err != nil {
					return 0, err
				}
				if expr[i] == '*' {
					return left * right, nil
				}
				if right == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				return left / right, nil
			}
		}
	}
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == '^' {
			left, err := evalExpr(expr[:i])
			if err != nil {
				return 0, err
			}
			right, err := evalExpr(expr[i+1:])
			if err != nil {
				return 0, err
			}
			return math.Pow(left, right), nil
		}
	}

	// Strip outer parentheses
	if len(expr) > 1 && expr[0] == '(' && expr[len(expr)-1] == ')' {
		return evalExpr(expr[1 : len(expr)-1])
	}

	return strconv.ParseFloat(expr, 64)
}

// ---------------------------------------------------------------------------
// DateTimeTool — returns current date/time
// ---------------------------------------------------------------------------

// DateTimeTool returns the current UTC date and time.
type DateTimeTool struct{}

func (d *DateTimeTool) Definition() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "get_datetime",
			Description: "Get the current UTC date and time.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
	}
}

func (d *DateTimeTool) Execute(_ string) (string, error) {
	return time.Now().UTC().Format(time.RFC3339), nil
}

// ---------------------------------------------------------------------------
// HTTPFetchTool — fetches a URL and returns its text content
// ---------------------------------------------------------------------------

// HTTPFetchTool fetches the raw text content of a URL.
type HTTPFetchTool struct{}

func (h *HTTPFetchTool) Definition() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "http_fetch",
			Description: "Fetch the text content of a URL. Useful for reading web pages or APIs.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"url": {
						"type": "string",
						"description": "The URL to fetch"
					}
				},
				"required": ["url"]
			}`),
		},
	}
}

func (h *HTTPFetchTool) Execute(argsJSON string) (string, error) {
	var args struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("http_fetch: parse args: %w", err)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(args.URL)
	if err != nil {
		return "", fmt.Errorf("http_fetch: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return "", fmt.Errorf("http_fetch: read body: %w", err)
	}
	return string(body), nil
}
