package security

import (
	"testing"
)

func TestRedactor_Redact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "AWS access key is redacted",
			input:    "aws_key = AKIAIOSFODNN7EXAMPLE",
			expected: "aws_key = [REDACTED:AWS Access Key]",
		},
		{
			name:     "AWS secret key is redacted",
			input:    `aws_secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"`,
			expected: `[REDACTED:AWS Secret Key]"`,
		},
		{
			name:     "GitHub token is redacted",
			input:    "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmn",
			expected: "token: [REDACTED:GitHub Token]",
		},
		{
			name:     "generic API key is redacted",
			input:    `api_key = "sk_live_abcdefghijklmnopqrstuv"`,
			expected: `[REDACTED:Generic API Key]`,
		},
		{
			name:     "generic secret is redacted",
			input:    `password = "super-secret-password-123"`,
			expected: `[REDACTED:Generic Secret]`,
		},
		{
			name:     "private key header is redacted",
			input:    "-----BEGIN RSA PRIVATE KEY-----\nMIIE...",
			expected: "[REDACTED:Private Key]\nMIIE...",
		},
		{
			name:     "JWT token is redacted",
			input:    "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc123def456",
			expected: "Bearer [REDACTED:JWT Token]",
		},
		{
			name:     "normal text is not modified",
			input:    "This is a normal log line with no secrets.",
			expected: "This is a normal log line with no secrets.",
		},
		{
			name:     "code without secrets is not modified",
			input:    "func main() { fmt.Println(\"hello world\") }",
			expected: "func main() { fmt.Println(\"hello world\") }",
		},
		{
			name:     "multiple secrets in same content are all redacted",
			input:    "key=AKIAIOSFODNN7EXAMPLE token=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmn",
			expected: "key=[REDACTED:AWS Access Key] token=[REDACTED:GitHub Token]",
		},
		{
			name:     "empty string returns empty",
			input:    "",
			expected: "",
		},
	}

	redactor := NewRedactor(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := redactor.Redact(tt.input)
			if got != tt.expected {
				t.Errorf("Redact() =\n  %q\nwant:\n  %q", got, tt.expected)
			}
		})
	}
}

func TestRedactor_ExtraPatterns(t *testing.T) {
	t.Parallel()

	redactor := NewRedactor([]string{`CUSTOM-[A-Z]{10}`})

	input := "token = CUSTOM-ABCDEFGHIJ"
	got := redactor.Redact(input)
	expected := "token = [REDACTED:Custom Pattern 1]"

	if got != expected {
		t.Errorf("Redact() with extra pattern =\n  %q\nwant:\n  %q", got, expected)
	}
}

func TestRedactor_InvalidExtraPattern(t *testing.T) {
	t.Parallel()

	// Invalid regex should be skipped without panic.
	redactor := NewRedactor([]string{`[invalid`})

	input := "normal text"
	got := redactor.Redact(input)

	if got != input {
		t.Errorf("Redact() with invalid pattern modified text: %q", got)
	}
}
