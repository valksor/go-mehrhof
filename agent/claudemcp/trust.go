package claudemcp

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ensureFolderTrusted marks dir as a trusted project in claude's config
// (~/.claude.json) so the interactive TUI does not block on the workspace
// "trust this folder?" dialog — there is no human in an orchestration session
// to confirm it, and kvelmo created the worktree, so trust is implicit.
//
// Best-effort: any failure is ignored (claude would then show the dialog, which
// is the prior behaviour). The write is atomic (temp + rename) and only happens
// when the entry is missing, so it rarely races claude's own writes to the file.
// dir is resolved through symlinks to match the absolute path claude keys on
// (e.g. /tmp -> /private/tmp on macOS).
func ensureFolderTrusted(dir string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	cfgPath := filepath.Join(home, ".claude.json")

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		// No claude config yet — claude will create it on first run; nothing to do.
		return
	}
	var cfg map[string]any
	if json.Unmarshal(data, &cfg) != nil {
		return
	}

	abs := dir
	if a, err := filepath.Abs(dir); err == nil {
		abs = a
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}

	projects, ok := cfg["projects"].(map[string]any)
	if !ok || projects == nil {
		projects = map[string]any{}
		cfg["projects"] = projects
	}
	proj, ok := projects[abs].(map[string]any)
	if !ok || proj == nil {
		proj = map[string]any{}
		projects[abs] = proj
	}
	if proj["hasTrustDialogAccepted"] == true {
		return // already trusted — skip the write to avoid racing claude
	}
	proj["hasTrustDialogAccepted"] = true
	proj["hasCompletedProjectOnboarding"] = true

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	tmp := cfgPath + ".kvelmo.tmp"
	if os.WriteFile(tmp, out, 0o600) != nil {
		return
	}
	_ = os.Rename(tmp, cfgPath)
}
