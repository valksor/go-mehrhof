package settings

import "testing"

func TestApplyPreset_Compliance(t *testing.T) {
	s := ApplyPreset("compliance")
	if s == nil {
		t.Fatal("expected non-nil settings for compliance preset")
	}
	if !s.Storage.ActivityLog.Enabled {
		t.Error("compliance preset should enable activity log")
	}
	if s.Storage.ActivityLog.MaxFiles != 365 {
		t.Error("compliance preset should set max files to 365")
	}
	if !s.Storage.MetricsHistory.Enabled {
		t.Error("compliance preset should enable metrics history")
	}
	if s.Storage.MetricsHistory.RetentionDays != 365 {
		t.Error("compliance preset should set retention days to 365")
	}
	if !s.Workflow.Policy.RequireSecurityScan {
		t.Error("compliance preset should require security scan")
	}
	if !s.Workflow.Policy.ApprovalRequired["submit"] {
		t.Error("compliance preset should require approval for submit")
	}
	if len(s.Workflow.Policy.ReviewChecklist) != 3 {
		t.Errorf("compliance preset should have 3 review checklist items, got %d", len(s.Workflow.Policy.ReviewChecklist))
	}
}

func TestApplyPreset_Unknown(t *testing.T) {
	if s := ApplyPreset("unknown"); s != nil {
		t.Error("expected nil for unknown preset")
	}
}
