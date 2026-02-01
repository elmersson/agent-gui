package agent

import (
	"context"
)

type Agent interface {
	Name() string
	Execute(ctx context.Context, input string) (<-chan string, <-chan error)
}

// TokenTrackingAgent is an optional interface for agents that support token tracking
type TokenTrackingAgent interface {
	Agent
	GetTokenUsageChan() <-chan TokenUsage
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Config struct {
	APIKey string
	Model  string
}

// TokenUsage tracks token consumption and cost for an agent execution
type TokenUsage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CacheRead    int     `json:"cache_read,omitempty"`
	CacheWrite   int     `json:"cache_write,omitempty"`
	TotalTokens  int     `json:"total_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// ModelPricing contains per-token pricing for a model (in USD per 1M tokens)
type ModelPricing struct {
	InputPricePerMillion  float64
	OutputPricePerMillion float64
}

// GetModelPricing returns pricing for known models
func GetModelPricing(model string) ModelPricing {
	// Pricing based on common LLM providers (USD per 1M tokens)
	pricing := map[string]ModelPricing{
		// Claude 4 models
		"claude-sonnet-4-20250514": {InputPricePerMillion: 3.00, OutputPricePerMillion: 15.00},
		"claude-opus-4-20250514":   {InputPricePerMillion: 15.00, OutputPricePerMillion: 75.00},
		// Claude 3.5 models
		"claude-3-5-sonnet-20241022": {InputPricePerMillion: 3.00, OutputPricePerMillion: 15.00},
		"claude-3-5-haiku-20241022":  {InputPricePerMillion: 0.80, OutputPricePerMillion: 4.00},
		// Claude 3 models
		"claude-3-opus-20240229":   {InputPricePerMillion: 15.00, OutputPricePerMillion: 75.00},
		"claude-3-sonnet-20240229": {InputPricePerMillion: 3.00, OutputPricePerMillion: 15.00},
		"claude-3-haiku-20240307":  {InputPricePerMillion: 0.25, OutputPricePerMillion: 1.25},
		// OpenAI models
		"gpt-4o":      {InputPricePerMillion: 2.50, OutputPricePerMillion: 10.00},
		"gpt-4o-mini": {InputPricePerMillion: 0.15, OutputPricePerMillion: 0.60},
		"gpt-4-turbo": {InputPricePerMillion: 10.00, OutputPricePerMillion: 30.00},
	}

	if p, ok := pricing[model]; ok {
		return p
	}
	// Default pricing (conservative estimate)
	return ModelPricing{InputPricePerMillion: 3.00, OutputPricePerMillion: 15.00}
}

// CalculateCost computes the cost in USD for given token usage
func (t *TokenUsage) CalculateCost(model string) float64 {
	pricing := GetModelPricing(model)
	inputCost := float64(t.InputTokens) * pricing.InputPricePerMillion / 1_000_000
	outputCost := float64(t.OutputTokens) * pricing.OutputPricePerMillion / 1_000_000
	t.CostUSD = inputCost + outputCost
	return t.CostUSD
}

// Add adds token counts from another TokenUsage
func (t *TokenUsage) Add(other TokenUsage) {
	t.InputTokens += other.InputTokens
	t.OutputTokens += other.OutputTokens
	t.TotalTokens = t.InputTokens + t.OutputTokens
	t.CostUSD += other.CostUSD
}
