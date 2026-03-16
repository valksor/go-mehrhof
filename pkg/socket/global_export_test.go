package socket

import (
	"testing"
	"time"
)

func TestParseSinceDuration_GoDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"1h", time.Hour},
		{"30m", 30 * time.Minute},
		{"2h30m", 2*time.Hour + 30*time.Minute},
		{"500ms", 500 * time.Millisecond},
	}

	for _, tt := range tests {
		got, err := parseSinceDuration(tt.input)
		if err != nil {
			t.Errorf("parseSinceDuration(%q) error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("parseSinceDuration(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseSinceDuration_DayShorthand(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"7d", 7 * 24 * time.Hour},
		{"30d", 30 * 24 * time.Hour},
		{"1d", 24 * time.Hour},
		{"90d", 90 * 24 * time.Hour},
	}

	for _, tt := range tests {
		got, err := parseSinceDuration(tt.input)
		if err != nil {
			t.Errorf("parseSinceDuration(%q) error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("parseSinceDuration(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseSinceDuration_Invalid(t *testing.T) {
	invalid := []string{
		"abc",
		"",
		"d",
		"7x",
		"not-a-duration",
	}

	for _, input := range invalid {
		_, err := parseSinceDuration(input)
		if err == nil {
			t.Errorf("parseSinceDuration(%q) expected error, got nil", input)
		}
	}
}
