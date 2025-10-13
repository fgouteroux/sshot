package main

import (
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
			name:     "color enabled",
			noColor:  false,
			input:    ColorRed,
			expected: ColorRed,
		},
		{
			name:     "color disabled",
			noColor:  true,
			input:    ColorRed,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execOptions.NoColor = tt.noColor
			result := color(tt.input)
			if result != tt.expected {
				t.Errorf("color() = %q, want %q", result, tt.expected)
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
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}
