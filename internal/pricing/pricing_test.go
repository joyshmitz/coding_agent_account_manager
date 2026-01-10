package pricing

import (
	"math"
	"testing"

	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/logs"
)

func TestNormalizeModelName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: " CLAUDE-3-5-SONNET ", want: "claude-3.5-sonnet"},
		{in: "claude-3-opus-20240229", want: "claude-3-opus"},
		{in: "gpt4o-mini", want: "gpt-4o-mini"},
		{in: "gemini_pro", want: "gemini-pro"},
	}

	for _, tt := range tests {
		got := NormalizeModelName(tt.in)
		if got != tt.want {
			t.Errorf("NormalizeModelName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPriceForAliases(t *testing.T) {
	price, ok := PriceFor("claude", "claude-3-5-sonnet")
	if !ok {
		t.Fatal("PriceFor claude alias returned ok=false")
	}
	if price.InputPer1M != ClaudePricing["claude-3.5-sonnet"].InputPer1M {
		t.Fatalf("PriceFor claude alias mismatch: got %v", price)
	}

	price, ok = PriceFor("codex", "gpt4o")
	if !ok {
		t.Fatal("PriceFor openai alias returned ok=false")
	}
	if price.InputPer1M != OpenAIPricing["gpt-4o"].InputPer1M {
		t.Fatalf("PriceFor openai alias mismatch: got %v", price)
	}
}

func TestCalculateCostSingleModel(t *testing.T) {
	usage := &logs.TokenUsage{
		InputTokens:       1_000_000,
		OutputTokens:      2_000_000,
		CacheReadTokens:   1_000_000,
		CacheCreateTokens: 1_000_000,
	}

	got := CalculateCost(usage, "claude-3-opus", "claude")
	want := 185.25
	if !floatApproxEqual(got, want, 1e-6) {
		t.Fatalf("CalculateCost() = %f, want %f", got, want)
	}
}

func TestCalculateCostAllModels(t *testing.T) {
	usage := &logs.TokenUsage{
		ByModel: map[string]*logs.ModelTokenUsage{
			"gpt-4o":      {Model: "gpt-4o", InputTokens: 2_000_000, OutputTokens: 1_000_000},
			"gpt-4o-mini": {Model: "gpt-4o-mini", InputTokens: 1_000_000, OutputTokens: 2_000_000},
		},
	}

	got := CalculateCost(usage, "", "codex")
	want := 26.35
	if !floatApproxEqual(got, want, 1e-6) {
		t.Fatalf("CalculateCost(all models) = %f, want %f", got, want)
	}
}

func floatApproxEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}
