package usage

import (
	"testing"
	"time"
)

func TestUsageInfo_AvailabilityScore(t *testing.T) {
	tests := []struct {
		name     string
		info     *UsageInfo
		expected int
	}{
		{
			name:     "nil returns 0",
			info:     nil,
			expected: 0,
		},
		{
			name:     "error returns 0",
			info:     &UsageInfo{Error: "some error"},
			expected: 0,
		},
		{
			name:     "empty info returns 100",
			info:     &UsageInfo{},
			expected: 100,
		},
		{
			name: "primary 50% used",
			info: &UsageInfo{
				PrimaryWindow: &UsageWindow{Utilization: 0.5},
			},
			expected: 75, // 100 - 50*0.5 = 75
		},
		{
			name: "primary 100% used",
			info: &UsageInfo{
				PrimaryWindow: &UsageWindow{Utilization: 1.0},
			},
			expected: 50, // 100 - 50*1.0 = 50
		},
		{
			name: "all windows at 50%",
			info: &UsageInfo{
				PrimaryWindow:   &UsageWindow{Utilization: 0.5},
				SecondaryWindow: &UsageWindow{Utilization: 0.5},
				TertiaryWindow:  &UsageWindow{Utilization: 0.5},
			},
			expected: 55, // 100 - 25 - 12.5 - 7.5 = 55
		},
		{
			name: "all at 100% with no credits",
			info: &UsageInfo{
				PrimaryWindow:   &UsageWindow{Utilization: 1.0},
				SecondaryWindow: &UsageWindow{Utilization: 1.0},
				TertiaryWindow:  &UsageWindow{Utilization: 1.0},
				Credits:         &CreditInfo{HasCredits: false, Unlimited: false},
			},
			expected: 0, // 100 - 50 - 25 - 15 - 10 = 0
		},
		{
			name: "uses UsedPercent when Utilization is 0",
			info: &UsageInfo{
				PrimaryWindow: &UsageWindow{UsedPercent: 50},
			},
			expected: 75,
		},
		{
			name: "unlimited credits don't penalize",
			info: &UsageInfo{
				Credits: &CreditInfo{HasCredits: false, Unlimited: true},
			},
			expected: 100,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := tc.info.AvailabilityScore()
			if score != tc.expected {
				t.Errorf("AvailabilityScore() = %d, expected %d", score, tc.expected)
			}
		})
	}
}

func TestUsageInfo_IsNearLimit(t *testing.T) {
	threshold := 0.8

	tests := []struct {
		name     string
		info     *UsageInfo
		expected bool
	}{
		{
			name:     "nil returns false",
			info:     nil,
			expected: false,
		},
		{
			name:     "empty info returns false",
			info:     &UsageInfo{},
			expected: false,
		},
		{
			name: "primary below threshold",
			info: &UsageInfo{
				PrimaryWindow: &UsageWindow{Utilization: 0.7},
			},
			expected: false,
		},
		{
			name: "primary at threshold",
			info: &UsageInfo{
				PrimaryWindow: &UsageWindow{Utilization: 0.8},
			},
			expected: true,
		},
		{
			name: "secondary above threshold",
			info: &UsageInfo{
				PrimaryWindow:   &UsageWindow{Utilization: 0.5},
				SecondaryWindow: &UsageWindow{Utilization: 0.9},
			},
			expected: true,
		},
		{
			name: "tertiary above threshold",
			info: &UsageInfo{
				PrimaryWindow:  &UsageWindow{Utilization: 0.5},
				TertiaryWindow: &UsageWindow{Utilization: 0.85},
			},
			expected: true,
		},
		{
			name: "model window above threshold",
			info: &UsageInfo{
				PrimaryWindow: &UsageWindow{Utilization: 0.5},
				ModelWindows: map[string]*UsageWindow{
					"claude-3-opus": {Utilization: 0.95},
				},
			},
			expected: true,
		},
		{
			name: "all below threshold",
			info: &UsageInfo{
				PrimaryWindow:   &UsageWindow{Utilization: 0.5},
				SecondaryWindow: &UsageWindow{Utilization: 0.6},
				TertiaryWindow:  &UsageWindow{Utilization: 0.7},
				ModelWindows: map[string]*UsageWindow{
					"claude-3-opus": {Utilization: 0.5},
				},
			},
			expected: false,
		},
		{
			name: "uses UsedPercent when Utilization is 0",
			info: &UsageInfo{
				PrimaryWindow: &UsageWindow{UsedPercent: 85},
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.info.IsNearLimit(threshold)
			if result != tc.expected {
				t.Errorf("IsNearLimit(%v) = %v, expected %v", threshold, result, tc.expected)
			}
		})
	}
}

func TestUsageInfo_TimeUntilReset(t *testing.T) {
	now := time.Now()
	future1h := now.Add(time.Hour)
	future2h := now.Add(2 * time.Hour)
	future30m := now.Add(30 * time.Minute)
	past := now.Add(-time.Hour)

	tests := []struct {
		name        string
		info        *UsageInfo
		expectZero  bool
		expectRange [2]time.Duration // min, max expected
	}{
		{
			name:       "nil returns 0",
			info:       nil,
			expectZero: true,
		},
		{
			name:       "empty info returns 0",
			info:       &UsageInfo{},
			expectZero: true,
		},
		{
			name: "primary window only",
			info: &UsageInfo{
				PrimaryWindow: &UsageWindow{ResetsAt: future1h},
			},
			expectRange: [2]time.Duration{55 * time.Minute, 65 * time.Minute},
		},
		{
			name: "picks earliest window",
			info: &UsageInfo{
				PrimaryWindow:   &UsageWindow{ResetsAt: future2h},
				SecondaryWindow: &UsageWindow{ResetsAt: future1h},
			},
			expectRange: [2]time.Duration{55 * time.Minute, 65 * time.Minute},
		},
		{
			name: "tertiary is earliest",
			info: &UsageInfo{
				PrimaryWindow:  &UsageWindow{ResetsAt: future2h},
				TertiaryWindow: &UsageWindow{ResetsAt: future30m},
			},
			expectRange: [2]time.Duration{25 * time.Minute, 35 * time.Minute},
		},
		{
			name: "model window is earliest",
			info: &UsageInfo{
				PrimaryWindow: &UsageWindow{ResetsAt: future2h},
				ModelWindows: map[string]*UsageWindow{
					"opus": {ResetsAt: future30m},
				},
			},
			expectRange: [2]time.Duration{25 * time.Minute, 35 * time.Minute},
		},
		{
			name: "past reset returns 0",
			info: &UsageInfo{
				PrimaryWindow: &UsageWindow{ResetsAt: past},
			},
			expectZero: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.info.TimeUntilReset()
			if tc.expectZero {
				if result != 0 {
					t.Errorf("TimeUntilReset() = %v, expected 0", result)
				}
			} else {
				if result < tc.expectRange[0] || result > tc.expectRange[1] {
					t.Errorf("TimeUntilReset() = %v, expected in range [%v, %v]",
						result, tc.expectRange[0], tc.expectRange[1])
				}
			}
		})
	}
}

func TestUsageInfo_MostConstrainedWindow(t *testing.T) {
	tests := []struct {
		name     string
		info     *UsageInfo
		expected float64 // expected utilization of most constrained
	}{
		{
			name:     "nil returns nil",
			info:     nil,
			expected: -1, // signal for nil
		},
		{
			name:     "empty info returns nil",
			info:     &UsageInfo{},
			expected: -1,
		},
		{
			name: "primary is most constrained",
			info: &UsageInfo{
				PrimaryWindow:   &UsageWindow{Utilization: 0.9},
				SecondaryWindow: &UsageWindow{Utilization: 0.5},
			},
			expected: 0.9,
		},
		{
			name: "tertiary is most constrained",
			info: &UsageInfo{
				PrimaryWindow:  &UsageWindow{Utilization: 0.5},
				TertiaryWindow: &UsageWindow{Utilization: 0.95},
			},
			expected: 0.95,
		},
		{
			name: "model window is most constrained",
			info: &UsageInfo{
				PrimaryWindow: &UsageWindow{Utilization: 0.5},
				ModelWindows: map[string]*UsageWindow{
					"opus": {Utilization: 1.0},
				},
			},
			expected: 1.0,
		},
		{
			name: "uses UsedPercent when Utilization is 0",
			info: &UsageInfo{
				PrimaryWindow:   &UsageWindow{UsedPercent: 80},
				SecondaryWindow: &UsageWindow{UsedPercent: 50},
			},
			expected: 0.8,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.info.MostConstrainedWindow()
			if tc.expected == -1 {
				if result != nil {
					t.Error("expected nil window")
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil window")
			}

			util := result.Utilization
			if util == 0 && result.UsedPercent > 0 {
				util = float64(result.UsedPercent) / 100.0
			}

			if util != tc.expected {
				t.Errorf("MostConstrainedWindow().Utilization = %v, expected %v", util, tc.expected)
			}
		})
	}
}

func TestUsageInfo_WindowForModel(t *testing.T) {
	tertiaryWindow := &UsageWindow{Utilization: 0.7}
	opusWindow := &UsageWindow{Utilization: 0.9}

	tests := []struct {
		name       string
		info       *UsageInfo
		model      string
		expectNil  bool
		expectUtil float64
	}{
		{
			name:      "nil returns nil",
			info:      nil,
			model:     "opus",
			expectNil: true,
		},
		{
			name:      "empty info returns nil",
			info:      &UsageInfo{},
			model:     "opus",
			expectNil: true,
		},
		{
			name: "finds model-specific window",
			info: &UsageInfo{
				TertiaryWindow: tertiaryWindow,
				ModelWindows: map[string]*UsageWindow{
					"claude-3-opus": opusWindow,
				},
			},
			model:      "claude-3-opus",
			expectUtil: 0.9,
		},
		{
			name: "falls back to tertiary",
			info: &UsageInfo{
				TertiaryWindow: tertiaryWindow,
				ModelWindows: map[string]*UsageWindow{
					"claude-3-opus": opusWindow,
				},
			},
			model:      "claude-3-sonnet",
			expectUtil: 0.7,
		},
		{
			name: "returns nil if no match and no tertiary",
			info: &UsageInfo{
				ModelWindows: map[string]*UsageWindow{
					"claude-3-opus": opusWindow,
				},
			},
			model:     "claude-3-sonnet",
			expectNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.info.WindowForModel(tc.model)
			if tc.expectNil {
				if result != nil {
					t.Error("expected nil window")
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil window")
			}

			if result.Utilization != tc.expectUtil {
				t.Errorf("WindowForModel(%s).Utilization = %v, expected %v",
					tc.model, result.Utilization, tc.expectUtil)
			}
		})
	}
}

func TestCreditInfo(t *testing.T) {
	t.Run("has credits", func(t *testing.T) {
		info := &UsageInfo{
			Credits: &CreditInfo{HasCredits: true},
		}
		// Should not penalize score
		score := info.AvailabilityScore()
		if score != 100 {
			t.Errorf("score with HasCredits=true = %d, expected 100", score)
		}
	})

	t.Run("no credits penalizes", func(t *testing.T) {
		info := &UsageInfo{
			Credits: &CreditInfo{HasCredits: false, Unlimited: false},
		}
		score := info.AvailabilityScore()
		if score != 90 {
			t.Errorf("score with no credits = %d, expected 90", score)
		}
	})
}
