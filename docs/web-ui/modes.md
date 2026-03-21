# Simple vs Developer Modes

kvelmo's web UI supports two modes to match different user needs.

## Simple Mode

A streamlined, single-column interface for users who want to load tasks and let the AI handle the details.

**Visible features:**
- Project list with search
- Add Project and Settings buttons
- Step-by-step task workflow (Start, Continue, Build, Review, Submit, Finish)
- Activity timeline from checkpoints
- Simplified chat (no @file, no /slash commands)
- File change summary with review actions

**Hidden features:**
Batch actions, Diagnose, Memory, Recordings, Backup, Activity log, Security scan, Catalog, Access tokens, Report, Export, MetricsWidget, StatsWidget, agent warnings, tabbed layout, keyboard workflow chords.

## Developer Mode

The full dashboard with tabs, panels, widgets, and all advanced tools. This is the default for existing users.

## First Visit

On first visit (no persisted preference), a **ModePickerModal** appears with two cards:
- **Simple**: "Load tasks and let the AI handle it"
- **Developer**: "Full control over the workflow"

Users must pick one to proceed. The choice is persisted to localStorage.

## Switching Modes

- **Toggle button**: appears in the header of both GlobalView and ProjectView
- **Keyboard shortcut**: `Ctrl+Shift+V` (`Cmd+Shift+V` on macOS) (works even when focused in an input field)
- **URL parameter**: `?mode=simple` or `?mode=developer` — persists the choice to localStorage

## URL Demo Mode

Append `?demo&mode=simple` to test simple mode without a backend connection.
