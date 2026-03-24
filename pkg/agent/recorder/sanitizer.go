package recorder

import (
	"os"
	"regexp"
	"strings"

	"github.com/valksor/kvelmo/pkg/settings"
)

// namedPattern pairs a compiled regexp with a human-readable name used in
// redaction placeholders (e.g. [REDACTED:AWS Access Key]).
type namedPattern struct {
	name    string
	pattern *regexp.Regexp
}

// Sanitizer masks secrets in recording data before it is written to disk.
// It applies two layers of protection:
//  1. Known token values (from settings / environment) are replaced with a
//     masked form that preserves the first 4 and last 4 characters.
//  2. Regex patterns detect common secret formats and replace them with a
//     tagged [REDACTED:<name>] placeholder.
type Sanitizer struct {
	knownTokens []string
	patterns    []namedPattern
}

// NewSanitizer creates a sanitizer that masks the given token values and
// matches common secret patterns.  Empty token strings are silently ignored.
func NewSanitizer(tokens []string) *Sanitizer {
	s := &Sanitizer{}

	for _, t := range tokens {
		if t != "" {
			s.knownTokens = append(s.knownTokens, t)
		}
	}

	s.patterns = []namedPattern{
		{
			name:    "AWS Access Key",
			pattern: regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		},
		{
			name:    "AWS Secret Key",
			pattern: regexp.MustCompile(`(?i)aws[_\-]?secret[_\-]?(?:access)?[_\-]?key['\"]?\s*[:=]\s*['\"]?([A-Za-z0-9/+=]{40})`),
		},
		{
			name:    "GitHub Token",
			pattern: regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{36,255}`),
		},
		{
			name:    "Generic API Key",
			pattern: regexp.MustCompile(`(?i)(?:api[_\-]?key|apikey)['\"]?\s*[:=]\s*['\"]?([A-Za-z0-9_\-]{20,})['\"]?`),
		},
		{
			name:    "Private Key",
			pattern: regexp.MustCompile(`(?s)-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----(?:.*?-----END (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----)?`),
		},
		{
			name:    "JWT Token",
			pattern: regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`),
		},
		{
			name:    "Anthropic API Key",
			pattern: regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`),
		},
		{
			name:    "OpenAI API Key",
			pattern: regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`),
		},
		{
			name:    "Generic Secret Assignment",
			pattern: regexp.MustCompile(`(?i)(?:secret|password|token|credential)['\"]?\s*[:=]\s*['\"]?([A-Za-z0-9_\-/.+=]{16,})['\"]?`),
		},
	}

	return s
}

// Sanitize replaces secrets in data and returns sanitized bytes.
// Known token values are masked first, then regex patterns are applied.
func (s *Sanitizer) Sanitize(data []byte) []byte {
	if s == nil {
		return data
	}

	text := string(data)

	// First pass: replace known token values with masked form.
	for _, token := range s.knownTokens {
		masked := settings.MaskToken(token)
		text = strings.ReplaceAll(text, token, masked)
	}

	// Second pass: apply regex patterns.
	for _, p := range s.patterns {
		text = p.pattern.ReplaceAllString(text, "[REDACTED:"+p.name+"]")
	}

	return []byte(text)
}

// SanitizeString is a convenience wrapper around Sanitize for string data.
func (s *Sanitizer) SanitizeString(str string) string {
	return string(s.Sanitize([]byte(str)))
}

// CollectSensitiveValues extracts non-empty token values from settings and
// well-known environment variables.  The returned slice is suitable for passing
// to NewSanitizer.
func CollectSensitiveValues(s *settings.Settings) []string {
	var values []string

	add := func(v string) {
		if v != "" {
			values = append(values, v)
		}
	}

	// Tokens from settings.
	add(s.Providers.GitHub.Token)
	add(s.Providers.GitLab.Token)
	add(s.Providers.Wrike.Token)
	add(s.Providers.Linear.Token)

	// API agent keys from settings.
	add(s.Agent.OpenAI.APIKey)
	add(s.Agent.Anthropic.APIKey)

	// Tokens from environment variables (may differ from settings).
	envVars := []string{
		"GITHUB_TOKEN",
		"GITLAB_TOKEN",
		"WRIKE_TOKEN",
		"LINEAR_TOKEN",
		"OPENAI_API_KEY",
		"ANTHROPIC_API_KEY",
	}
	for _, env := range envVars {
		add(os.Getenv(env))
	}

	return values
}
