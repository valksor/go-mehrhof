package settings

import (
	"testing"
)

func TestMerge_AllProviderFields(t *testing.T) {
	dst := DefaultSettings()
	src := &Settings{
		Providers: ProviderSettings{
			GitHub: GitHubConfig{
				Token:              "ghtoken",
				Owner:              "myorg",
				AllowTicketComment: true,
				StatusSync:         true,
				StatusMapping:      map[string]string{"implementing": "in-progress"},
			},
			GitLab: GitLabConfig{
				Token:              "gltoken",
				BaseURL:            "https://gl.example.com",
				AllowTicketComment: true,
				StatusSync:         true,
				StatusMapping:      map[string]string{"reviewing": "review"},
			},
			Wrike: WrikeConfig{
				Token:                 "wktoken",
				IncludeParentContext:  true,
				IncludeSiblingContext: true,
				AllowTicketComment:    true,
			},
			Linear: LinearConfig{
				Token:                 "lntoken",
				Team:                  "engineering",
				IncludeParentContext:  true,
				IncludeSiblingContext: true,
				AllowTicketComment:    true,
				StatusSync:            true,
				StatusMapping:         map[string]string{"submitted": "in-review"},
			},
			Jira: JiraConfig{
				Token:              "jratoken",
				Email:              "user@example.com",
				BaseURL:            "https://example.atlassian.net",
				AllowTicketComment: true,
				StatusSync:         true,
				StatusMapping:      map[string]string{"implementing": "In Progress"},
			},
			AzureDevOps: AzureDevOpsConfig{
				Token:              "aztoken",
				BaseURL:            "https://dev.azure.com",
				Organization:       "myorg",
				Project:            "myproj",
				Repository:         "myrepo",
				AllowTicketComment: true,
				StatusSync:         true,
				StatusMapping:      map[string]string{"submitted": "Active"},
			},
		},
	}

	Merge(dst, src)

	// GitHub
	if dst.Providers.GitHub.Token != "ghtoken" || !dst.Providers.GitHub.StatusSync ||
		dst.Providers.GitHub.StatusMapping["implementing"] != "in-progress" {
		t.Error("GitHub config not fully merged")
	}
	// GitLab
	if dst.Providers.GitLab.BaseURL != "https://gl.example.com" || !dst.Providers.GitLab.StatusSync ||
		dst.Providers.GitLab.StatusMapping["reviewing"] != "review" {
		t.Error("GitLab config not fully merged")
	}
	// Wrike
	if !dst.Providers.Wrike.IncludeParentContext || !dst.Providers.Wrike.IncludeSiblingContext ||
		!dst.Providers.Wrike.AllowTicketComment || dst.Providers.Wrike.Token != "wktoken" {
		t.Error("Wrike config not fully merged")
	}
	// Linear
	if dst.Providers.Linear.Team != "engineering" || !dst.Providers.Linear.StatusSync ||
		dst.Providers.Linear.StatusMapping["submitted"] != "in-review" {
		t.Error("Linear config not fully merged")
	}
	// Jira
	if dst.Providers.Jira.Email != "user@example.com" || dst.Providers.Jira.BaseURL == "" ||
		!dst.Providers.Jira.StatusSync || dst.Providers.Jira.StatusMapping["implementing"] != "In Progress" {
		t.Error("Jira config not fully merged")
	}
	// AzureDevOps
	if dst.Providers.AzureDevOps.Organization != "myorg" || dst.Providers.AzureDevOps.Project != "myproj" ||
		dst.Providers.AzureDevOps.Repository != "myrepo" || !dst.Providers.AzureDevOps.StatusSync ||
		dst.Providers.AzureDevOps.StatusMapping["submitted"] != "Active" {
		t.Error("AzureDevOps config not fully merged")
	}
}

func TestMerge_StatusMappingPreservesExisting(t *testing.T) {
	// Existing dst entries should survive, new src entries should be added.
	dst := DefaultSettings()
	dst.Providers.GitHub.StatusMapping = map[string]string{"loaded": "todo"}

	src := &Settings{
		Providers: ProviderSettings{
			GitHub: GitHubConfig{
				StatusMapping: map[string]string{"reviewing": "review"},
			},
		},
	}

	Merge(dst, src)

	if dst.Providers.GitHub.StatusMapping["loaded"] != "todo" {
		t.Error("existing status mapping entry was lost")
	}
	if dst.Providers.GitHub.StatusMapping["reviewing"] != "review" {
		t.Error("new status mapping entry was not merged")
	}
}

func TestMerge_WorkflowExternalReview(t *testing.T) {
	dst := DefaultSettings()
	src := &Settings{
		Workflow: WorkflowSettings{
			ExternalReview: ExternalReviewConfig{
				Mode:    ExternalReviewNever,
				Command: "/usr/bin/review",
			},
		},
	}
	Merge(dst, src)

	if dst.Workflow.ExternalReview.Mode != ExternalReviewNever {
		t.Errorf("ExternalReview.Mode = %q, want %q", dst.Workflow.ExternalReview.Mode, ExternalReviewNever)
	}
	if dst.Workflow.ExternalReview.Command != "/usr/bin/review" {
		t.Errorf("ExternalReview.Command = %q, want /usr/bin/review", dst.Workflow.ExternalReview.Command)
	}
}

func TestMerge_PolicyFields(t *testing.T) {
	dst := DefaultSettings()
	src := &Settings{
		Workflow: WorkflowSettings{
			Policy: PolicySettings{
				RequiredPhases:   []string{"plan", "implement"},
				SensitivePaths:   []string{"infra/**", "secrets/**"},
				MinSpecSections:  3,
				ApprovalRequired: map[string]bool{"submit": true},
				DocRequirements: []DocRequirement{
					{Trigger: "internal/**.go", Requires: "docs/**.md"},
				},
			},
		},
	}
	Merge(dst, src)

	if len(dst.Workflow.Policy.RequiredPhases) != 2 {
		t.Errorf("RequiredPhases not merged: %v", dst.Workflow.Policy.RequiredPhases)
	}
	if len(dst.Workflow.Policy.SensitivePaths) != 2 {
		t.Errorf("SensitivePaths not merged: %v", dst.Workflow.Policy.SensitivePaths)
	}
	if dst.Workflow.Policy.MinSpecSections != 3 {
		t.Errorf("MinSpecSections = %d, want 3", dst.Workflow.Policy.MinSpecSections)
	}
	if !dst.Workflow.Policy.ApprovalRequired["submit"] {
		t.Error("ApprovalRequired[submit] should be true")
	}
	if len(dst.Workflow.Policy.DocRequirements) != 1 {
		t.Errorf("DocRequirements not merged: %v", dst.Workflow.Policy.DocRequirements)
	}
}

func TestMerge_PhaseGuardrails(t *testing.T) {
	dst := DefaultSettings()
	src := &Settings{
		Workflow: WorkflowSettings{
			PhaseGuardrails: map[string]GuardrailConfig{
				"plan": {Pre: []string{"echo before"}, Post: []string{"echo after"}},
			},
		},
	}
	Merge(dst, src)

	if dst.Workflow.PhaseGuardrails == nil {
		t.Fatal("PhaseGuardrails map is nil after merge")
	}
	if _, ok := dst.Workflow.PhaseGuardrails["plan"]; !ok {
		t.Error("plan guardrail not merged")
	}
}

func TestMerge_UIOnboardingDismissed(t *testing.T) {
	dst := DefaultSettings()
	if dst.UI.OnboardingDismissed {
		t.Fatal("UI.OnboardingDismissed should default to false")
	}

	src := &Settings{UI: UISettings{OnboardingDismissed: true}}
	Merge(dst, src)

	if !dst.UI.OnboardingDismissed {
		t.Error("UI.OnboardingDismissed should be true after merge")
	}
}

func TestMerge_StorageFields(t *testing.T) {
	dst := DefaultSettings()
	commitSpecs := true
	saveInProject := true

	src := &Settings{
		Storage: StorageSettings{
			SaveInProject:  &saveInProject,
			SpecOutputPath: "specs/{key}.md",
			CommitSpecs:    &commitSpecs,
			ChangelogPath:  "CHANGES.md",
		},
	}
	Merge(dst, src)

	if dst.Storage.SaveInProject == nil || !*dst.Storage.SaveInProject {
		t.Error("SaveInProject not merged")
	}
	if dst.Storage.SpecOutputPath != "specs/{key}.md" {
		t.Errorf("SpecOutputPath = %q", dst.Storage.SpecOutputPath)
	}
	if dst.Storage.CommitSpecs == nil || !*dst.Storage.CommitSpecs {
		t.Error("CommitSpecs not merged")
	}
	if dst.Storage.ChangelogPath != "CHANGES.md" {
		t.Errorf("ChangelogPath = %q", dst.Storage.ChangelogPath)
	}
}

func TestMerge_GitExtras(t *testing.T) {
	dst := DefaultSettings()
	src := &Settings{
		Git: GitSettings{
			BaseBranch:              "develop",
			CommitPattern:           `^(feat|fix):`,
			PRTitlePattern:          "[{key}] {title}",
			BranchValidationPattern: `^feature/.*`,
		},
	}
	Merge(dst, src)

	if dst.Git.BaseBranch != "develop" {
		t.Errorf("BaseBranch = %q, want develop", dst.Git.BaseBranch)
	}
	if dst.Git.CommitPattern == "" {
		t.Error("CommitPattern not merged")
	}
	if dst.Git.PRTitlePattern == "" {
		t.Error("PRTitlePattern not merged")
	}
	if dst.Git.BranchValidationPattern == "" {
		t.Error("BranchValidationPattern not merged")
	}
}

func TestMerge_WorkflowHoldTheLine(t *testing.T) {
	dst := DefaultSettings()
	holdTrue := true
	src := &Settings{
		Workflow: WorkflowSettings{HoldTheLine: &holdTrue},
	}
	Merge(dst, src)

	if dst.Workflow.HoldTheLine == nil || !*dst.Workflow.HoldTheLine {
		t.Error("HoldTheLine not merged")
	}
}

// ─── setStatusMapping ───────────────────────────────────────────────────────

func TestSetStatusMapping_StringMap(t *testing.T) {
	s := DefaultSettings()
	input := map[string]string{"implementing": "in-progress", "submitted": "review"}
	if err := SetValue(s, "providers.github.status_mapping", input); err != nil {
		t.Fatalf("SetValue: %v", err)
	}
	if s.Providers.GitHub.StatusMapping["implementing"] != "in-progress" {
		t.Errorf("status_mapping not set")
	}
}

func TestSetStatusMapping_AnyMap(t *testing.T) {
	// JSON/YAML unmarshaling often produces map[string]any.
	s := DefaultSettings()
	input := map[string]any{"implementing": "in-progress", "submitted": "review"}
	if err := SetValue(s, "providers.gitlab.status_mapping", input); err != nil {
		t.Fatalf("SetValue: %v", err)
	}
	if s.Providers.GitLab.StatusMapping["submitted"] != "review" {
		t.Errorf("status_mapping[submitted] = %q", s.Providers.GitLab.StatusMapping["submitted"])
	}
}

func TestSetStatusMapping_NonStringValueReturnsError(t *testing.T) {
	s := DefaultSettings()
	input := map[string]any{"implementing": 123}
	err := SetValue(s, "providers.linear.status_mapping", input)
	if err == nil {
		t.Error("expected error when value is not a string")
	}
}

func TestSetStatusMapping_WrongTypeReturnsError(t *testing.T) {
	s := DefaultSettings()
	err := SetValue(s, "providers.jira.status_mapping", "not-a-map")
	if err == nil {
		t.Error("expected error when value is not a map")
	}
}

func TestSetStatusMapping_AzureDevOps(t *testing.T) {
	s := DefaultSettings()
	input := map[string]string{"submitted": "Active"}
	if err := SetValue(s, "providers.azuredevops.status_mapping", input); err != nil {
		t.Fatalf("SetValue: %v", err)
	}
	if s.Providers.AzureDevOps.StatusMapping["submitted"] != "Active" {
		t.Error("azuredevops status_mapping not set")
	}
}
