package config

// Config holds all application configuration
type Config struct {
	Agent     AgentConfig     `mapstructure:"agent"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Git       GitConfig       `mapstructure:"git"`
	Providers ProvidersConfig `mapstructure:"providers"`
	UI        UIConfig        `mapstructure:"ui"`
}

// AgentConfig holds AI agent settings
type AgentConfig struct {
	Default    string       `mapstructure:"default"`
	Claude     ClaudeConfig `mapstructure:"claude"`
	Timeout    int          `mapstructure:"timeout"`
	MaxRetries int          `mapstructure:"maxretries"`
}

// ClaudeConfig holds Claude agent settings
type ClaudeConfig struct {
	Model       string  `mapstructure:"model"`
	MaxTokens   int     `mapstructure:"maxtokens"`
	Temperature float64 `mapstructure:"temperature"`
}

// StorageConfig holds storage settings
type StorageConfig struct {
	Root                 string `mapstructure:"root"`
	MaxBlueprints        int    `mapstructure:"maxblueprints"`
	SessionRetentionDays int    `mapstructure:"sessionretentiondays"`
}

// GitConfig holds git settings
type GitConfig struct {
	AutoCommit    bool   `mapstructure:"autocommit"`
	CommitPrefix  string `mapstructure:"commitprefix"`
	BranchPattern string `mapstructure:"branchpattern"`
	SignCommits   bool   `mapstructure:"signcommits"`
}

// ProvidersConfig holds provider settings
type ProvidersConfig struct {
	Default   string                  `mapstructure:"default"` // Default provider for bare references (e.g., "file")
	File      FileProviderConfig      `mapstructure:"file"`
	Directory DirectoryProviderConfig `mapstructure:"directory"`
}

// FileProviderConfig holds file provider settings
type FileProviderConfig struct {
	BasePath string `mapstructure:"basepath"`
}

// DirectoryProviderConfig holds directory provider settings
type DirectoryProviderConfig struct {
	BasePath string `mapstructure:"basepath"`
}

// UIConfig holds UI settings
type UIConfig struct {
	Color    bool   `mapstructure:"color"`
	Format   string `mapstructure:"format"`
	Verbose  bool   `mapstructure:"verbose"`
	Progress string `mapstructure:"progress"`
}

// NewDefault creates a Config with default values
func NewDefault() *Config {
	return &Config{
		Agent: AgentConfig{
			Default:    "claude",
			Timeout:    300,
			MaxRetries: 3,
			Claude: ClaudeConfig{
				Model:       "claude-sonnet-4-20250514",
				MaxTokens:   8192,
				Temperature: 0.7,
			},
		},
		Storage: StorageConfig{
			Root:                 ".mehrhof",
			MaxBlueprints:        100,
			SessionRetentionDays: 30,
		},
		Git: GitConfig{
			AutoCommit:    true,
			BranchPattern: "task/{task_id}",
		},
		Providers: ProvidersConfig{
			File: FileProviderConfig{
				BasePath: ".",
			},
			Directory: DirectoryProviderConfig{
				BasePath: ".",
			},
		},
		UI: UIConfig{
			Color:    true,
			Format:   "text",
			Progress: "spinner",
		},
	}
}
