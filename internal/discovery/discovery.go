// Package discovery scans a project directory to find available commands and scripts.
// The discovered list is injected into agent prompts so agents know what tools they can run.
package discovery

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var makeTargetRe = regexp.MustCompile(`^([a-zA-Z0-9][a-zA-Z0-9_\-]*):(?:[^=]|$)`)

// DiscoverTools scans workDir and returns a list of runnable commands found in
// Makefile targets, package.json scripts, Taskfile.yml tasks, and bin/ executables.
// Results are formatted as ready-to-run commands (e.g. "make test", "bun run dev").
func DiscoverTools(workDir string) []string {
	tools := make([]string, 0, 32)
	tools = append(tools, makeTargets(workDir)...)
	tools = append(tools, packageJSONScripts(workDir)...)
	tools = append(tools, taskfileTargets(workDir)...)
	tools = append(tools, binExecutables(workDir)...)

	return tools
}

func makeTargets(workDir string) []string {
	f, err := os.Open(filepath.Join(workDir, "Makefile"))
	if err != nil {
		return nil
	}
	defer f.Close() //nolint:errcheck // Read-only file handle; close error is not actionable here

	var targets []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if m := makeTargetRe.FindStringSubmatch(line); m != nil {
			name := m[1]
			// Skip internal/phony conventions starting with dot or underscore
			if !strings.HasPrefix(name, ".") && !strings.HasPrefix(name, "_") {
				targets = append(targets, "make "+name)
			}
		}
	}
	if scanner.Err() != nil {
		return nil
	}

	return targets
}

func packageJSONScripts(workDir string) []string {
	data, err := os.ReadFile(filepath.Join(workDir, "package.json"))
	if err != nil {
		return nil
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	// Detect package manager: prefer bun if bun.lockb exists, else npm
	runner := "npm run"
	if _, err := os.Stat(filepath.Join(workDir, "bun.lockb")); err == nil {
		runner = "bun run"
	}

	var scripts []string
	for name := range pkg.Scripts {
		scripts = append(scripts, runner+" "+name)
	}

	return scripts
}

type taskfileYAML struct {
	Tasks map[string]any `yaml:"tasks"`
}

func taskfileTargets(workDir string) []string {
	var data []byte
	var err error
	for _, name := range []string{"Taskfile.yml", "Taskfile.yaml"} {
		data, err = os.ReadFile(filepath.Join(workDir, name))
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil
	}

	var tf taskfileYAML
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil
	}

	var tasks []string
	for name := range tf.Tasks {
		tasks = append(tasks, "task "+name)
	}

	return tasks
}

func binExecutables(workDir string) []string {
	entries, err := os.ReadDir(filepath.Join(workDir, "bin"))
	if err != nil {
		return nil
	}

	var execs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Mode()&0o111 != 0 {
			execs = append(execs, "./bin/"+e.Name())
		}
	}

	return execs
}
