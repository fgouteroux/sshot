package utils

import (
	"github.com/fgouteroux/sshot/pkg/types"
	"testing"
	"time"
)

func TestColor(t *testing.T) {
	tests := []struct {
		name     string
		noColor  bool
		input    string
		expected string
	}{
		{
			name:     "Color enabled",
			noColor:  false,
			input:    ColorRed,
			expected: ColorRed,
		},
		{
			name:     "Color disabled",
			noColor:  true,
			input:    ColorRed,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			types.ExecOptions.NoColor = tt.noColor
			result := Color(tt.input)
			if result != tt.expected {
				t.Errorf("Color() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "only seconds",
			duration: 45 * time.Second,
			expected: "45s",
		},
		{
			name:     "minutes and seconds",
			duration: 2*time.Minute + 30*time.Second,
			expected: "2m30s",
		},
		{
			name:     "hours, minutes and seconds",
			duration: 1*time.Hour + 15*time.Minute + 30*time.Second,
			expected: "1h15m30s",
		},
		{
			name:     "zero duration",
			duration: 0,
			expected: "0s",
		},
		{
			name:     "rounds to nearest second",
			duration: 45*time.Second + 500*time.Millisecond,
			expected: "46s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}
