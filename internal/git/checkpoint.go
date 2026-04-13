package git

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type CheckpointManager struct {
	repo        *Repository
	checkpoints []Checkpoint
	current     int
}

type Checkpoint struct {
	ID        string
	CommitSHA string
	Branch    string
	Message   string
	State     string
	CreatedAt time.Time
}

func NewCheckpointManager(repo *Repository) *CheckpointManager {
	return &CheckpointManager{
		repo:        repo,
		checkpoints: make([]Checkpoint, 0),
		current:     -1,
	}
}

func (m *CheckpointManager) Create(ctx context.Context, state, message, commitPrefix string) (*Checkpoint, error) {
	// Stage and commit
	if err := m.repo.StageAll(ctx); err != nil {
		return nil, fmt.Errorf("stage: %w", err)
	}

	has, err := m.repo.HasUncommittedChanges(ctx)
	if err != nil {
		return nil, fmt.Errorf("check changes: %w", err)
	}

	prefix := commitPrefix
	if prefix == "" {
		prefix = "[kvelmo]"
	}

	var sha string
	if has {
		sha, err = m.repo.Commit(ctx, fmt.Sprintf("%s %s: %s", prefix, state, message))
		if err != nil {
			return nil, fmt.Errorf("commit: %w", err)
		}
	} else {
		sha, err = m.repo.CurrentCommit(ctx)
		if err != nil {
			return nil, fmt.Errorf("get commit: %w", err)
		}
	}

	branch, err := m.repo.CurrentBranch(ctx)
	if err != nil {
		return nil, fmt.Errorf("get branch: %w", err)
	}

	cp := Checkpoint{
		ID:        fmt.Sprintf("cp-%d", len(m.checkpoints)+1),
		CommitSHA: sha,
		Branch:    branch,
		Message:   message,
		State:     state,
		CreatedAt: time.Now(),
	}

	// Truncate history if we're not at the end
	if m.current < len(m.checkpoints)-1 {
		m.checkpoints = m.checkpoints[:m.current+1]
	}

	m.checkpoints = append(m.checkpoints, cp)
	m.current = len(m.checkpoints) - 1

	return &cp, nil
}

func (m *CheckpointManager) CanUndo() bool {
	return m.current > 0
}

func (m *CheckpointManager) CanRedo() bool {
	return m.current < len(m.checkpoints)-1
}

func (m *CheckpointManager) Undo(ctx context.Context) (*Checkpoint, error) {
	if !m.CanUndo() {
		return nil, errors.New("nothing to undo")
	}

	m.current--
	cp := m.checkpoints[m.current]

	if err := m.repo.Reset(ctx, cp.CommitSHA, true); err != nil {
		m.current++ // Revert

		return nil, fmt.Errorf("reset: %w", err)
	}

	return &cp, nil
}

func (m *CheckpointManager) Redo(ctx context.Context) (*Checkpoint, error) {
	if !m.CanRedo() {
		return nil, errors.New("nothing to redo")
	}

	m.current++
	cp := m.checkpoints[m.current]

	if err := m.repo.Reset(ctx, cp.CommitSHA, true); err != nil {
		m.current-- // Revert

		return nil, fmt.Errorf("reset: %w", err)
	}

	return &cp, nil
}

func (m *CheckpointManager) List() []Checkpoint {
	return m.checkpoints
}

func (m *CheckpointManager) Current() *Checkpoint {
	if m.current < 0 || m.current >= len(m.checkpoints) {
		return nil
	}

	return &m.checkpoints[m.current]
}

func (m *CheckpointManager) GoTo(ctx context.Context, id string) (*Checkpoint, error) {
	for i, cp := range m.checkpoints {
		if cp.ID == id {
			if err := m.repo.Reset(ctx, cp.CommitSHA, true); err != nil {
				return nil, fmt.Errorf("reset: %w", err)
			}
			m.current = i

			return &cp, nil
		}
	}

	return nil, fmt.Errorf("checkpoint not found: %s", id)
}
