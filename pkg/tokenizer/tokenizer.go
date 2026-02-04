package tokenizer

import (
	"strings"
	"unicode"
)

// Tokenizer provides token counting functionality
type Tokenizer struct {
	// avgCharsPerToken is the estimated average characters per token
	// This varies by model but ~4 chars/token is a reasonable estimate for most LLMs
	avgCharsPerToken float64
}

// NewTokenizer creates a new tokenizer instance
func NewTokenizer() *Tokenizer {
	return &Tokenizer{
		avgCharsPerToken: 4.0,
	}
}

// CountTokens estimates the number of tokens in a string
// This uses a heuristic approach based on whitespace, punctuation, and character count
// For more accurate counts, integrate with model-specific tokenizers (tiktoken, etc.)
func (t *Tokenizer) CountTokens(text string) int {
	if len(text) == 0 {
		return 0
	}

	// Count words (whitespace-separated)
	words := strings.Fields(text)
	wordCount := len(words)

	// Count punctuation marks that typically become separate tokens
	punctCount := 0
	for _, r := range text {
		if unicode.IsPunct(r) {
			punctCount++
		}
	}

	// Estimate based on character count divided by average chars per token
	charEstimate := int(float64(len(text)) / t.avgCharsPerToken)

	// Use a weighted average of different estimation methods
	// Words tend to undercount, char estimate is more reliable
	estimated := (wordCount + charEstimate + punctCount/2) / 2

	// Ensure at least 1 token for non-empty text
	if estimated < 1 {
		estimated = 1
	}

	return estimated
}

// CountTokensDelta calculates token delta for incremental updates
// This is useful for streaming scenarios where we want to track token additions
func (t *Tokenizer) CountTokensDelta(previousText, newText string) int {
	if len(newText) <= len(previousText) {
		return 0
	}

	// Only count tokens in the new portion
	delta := newText[len(previousText):]
	return t.CountTokens(delta)
}

// EstimateFromCharCount provides a quick estimate based only on character count
// Useful when full text analysis isn't needed
func (t *Tokenizer) EstimateFromCharCount(charCount int) int {
	if charCount <= 0 {
		return 0
	}
	tokens := int(float64(charCount) / t.avgCharsPerToken)
	if tokens < 1 {
		return 1
	}
	return tokens
}
