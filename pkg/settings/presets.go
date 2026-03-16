package settings

// PresetNames lists all available preset names.
var PresetNames = []string{"compliance", "fast", "solo"}

// ApplyPreset returns a Settings struct with preset defaults applied.
// Only non-zero values from the preset are set; the caller should merge them
// with existing settings (preset values act as defaults, not overrides).
func ApplyPreset(name string) *Settings {
	switch name {
	case "compliance":
		return compliancePreset()
	case "fast":
		return fastPreset()
	case "solo":
		return soloPreset()
	default:
		return nil
	}
}

func compliancePreset() *Settings {
	return &Settings{
		Storage: StorageSettings{
			ActivityLog: ActivityLogSettings{
				Enabled:  true,
				MaxFiles: 365,
			},
			MetricsHistory: MetricsHistorySettings{
				Enabled:       true,
				RetentionDays: 365,
			},
		},
		Workflow: WorkflowSettings{
			Policy: PolicySettings{
				RequireSecurityScan: true,
				ApprovalRequired:    map[string]bool{"submit": true},
				ReviewChecklist:     []string{"security review", "tests passing", "docs updated"},
			},
		},
	}
}

func fastPreset() *Settings {
	return &Settings{
		Workers: WorkerSettings{
			Max: 5,
		},
		Workflow: WorkflowSettings{
			AutoAdvance: true,
		},
	}
}

func soloPreset() *Settings {
	return &Settings{
		Workers: WorkerSettings{
			Max: 1,
		},
	}
}
