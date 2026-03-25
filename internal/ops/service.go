package ops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const runTimeout = 2 * time.Minute

type Service struct {
	workspaceRoot string
	statePath     string

	mu      sync.Mutex
	actions map[string]*Action
}

func NewService(workspaceRoot string) (*Service, error) {
	root, err := resolveWorkspaceRoot(workspaceRoot)
	if err != nil {
		return nil, err
	}
	stateDir := filepath.Join(root, ".kokoclaw-lite")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}

	svc := &Service{
		workspaceRoot: root,
		statePath:     filepath.Join(stateDir, "approvals.json"),
		actions:       map[string]*Action{},
	}
	if err := svc.loadLocked(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *Service) QueueRun(requestedBy, command string) (Action, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return Action{}, fmt.Errorf("command is required")
	}
	decision := EvaluateRunPolicy(command)
	return s.createAction(Action{
		RequestedBy:    strings.TrimSpace(requestedBy),
		Kind:           ActionRun,
		Command:        command,
		Status:         StatusPending,
		PolicyDecision: decision.Decision,
		PolicyReason:   decision.Reason,
	})
}

func (s *Service) QueueWrite(requestedBy, path, content string) (Action, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Action{}, fmt.Errorf("path is required")
	}
	decision := EvaluateWritePolicy(path, content)
	return s.createAction(Action{
		RequestedBy:    strings.TrimSpace(requestedBy),
		Kind:           ActionWrite,
		Path:           path,
		Content:        content,
		Status:         StatusPending,
		PolicyDecision: decision.Decision,
		PolicyReason:   decision.Reason,
	})
}

func (s *Service) List() []Action {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]Action, 0, len(s.actions))
	for _, action := range s.actions {
		items = append(items, *action)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items
}

func (s *Service) Approve(id string) (Action, error) {
	s.mu.Lock()
	action, ok := s.actions[strings.TrimSpace(id)]
	if !ok {
		s.mu.Unlock()
		return Action{}, fmt.Errorf("approval %s not found", id)
	}
	if action.Status != StatusPending {
		copy := *action
		s.mu.Unlock()
		return copy, fmt.Errorf("approval %s is not pending", id)
	}
	if action.PolicyDecision == "deny" {
		copy := *action
		s.mu.Unlock()
		return copy, fmt.Errorf("approval %s is policy-denied; use override instead", id)
	}
	action.Status = StatusApproved
	action.UpdatedAt = time.Now().UTC()
	kind := action.Kind
	command := action.Command
	path := action.Path
	content := action.Content
	s.mu.Unlock()

	result, err := s.execute(kind, command, path, content)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		action.Status = StatusFailed
		action.Error = err.Error()
		action.UpdatedAt = time.Now().UTC()
		if saveErr := s.saveLocked(); saveErr != nil {
			err = fmt.Errorf("%w (plus save error: %v)", err, saveErr)
		}
		return *action, err
	}
	action.Result = result
	action.UpdatedAt = time.Now().UTC()
	if err := s.saveLocked(); err != nil {
		return *action, err
	}
	return *action, nil
}

func (s *Service) Override(id, reason string) (Action, error) {
	s.mu.Lock()
	action, ok := s.actions[strings.TrimSpace(id)]
	if !ok {
		s.mu.Unlock()
		return Action{}, fmt.Errorf("approval %s not found", id)
	}
	if action.Status != StatusPending {
		copy := *action
		s.mu.Unlock()
		return copy, fmt.Errorf("approval %s is not pending", id)
	}
	action.Override = true
	action.OverrideReason = strings.TrimSpace(reason)
	if action.OverrideReason == "" {
		action.OverrideReason = "operator override"
	}
	action.Status = StatusApproved
	action.UpdatedAt = time.Now().UTC()
	kind := action.Kind
	command := action.Command
	path := action.Path
	content := action.Content
	s.mu.Unlock()

	result, err := s.execute(kind, command, path, content)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		action.Status = StatusFailed
		action.Error = err.Error()
		action.UpdatedAt = time.Now().UTC()
		if saveErr := s.saveLocked(); saveErr != nil {
			err = fmt.Errorf("%w (plus save error: %v)", err, saveErr)
		}
		return *action, err
	}
	action.Result = result
	action.UpdatedAt = time.Now().UTC()
	if err := s.saveLocked(); err != nil {
		return *action, err
	}
	return *action, nil
}

func (s *Service) Deny(id string) (Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	action, ok := s.actions[strings.TrimSpace(id)]
	if !ok {
		return Action{}, fmt.Errorf("approval %s not found", id)
	}
	if action.Status != StatusPending {
		return *action, fmt.Errorf("approval %s is not pending", id)
	}
	action.Status = StatusDenied
	action.UpdatedAt = time.Now().UTC()
	if err := s.saveLocked(); err != nil {
		return *action, err
	}
	return *action, nil
}

func (s *Service) createAction(action Action) (Action, error) {
	id, err := randomID()
	if err != nil {
		return Action{}, err
	}
	now := time.Now().UTC()
	action.ID = id
	action.CreatedAt = now
	action.UpdatedAt = now
	if strings.TrimSpace(action.RequestedBy) == "" {
		action.RequestedBy = "operator"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.actions[action.ID] = &action
	if err := s.saveLocked(); err != nil {
		return Action{}, err
	}
	return action, nil
}

func (s *Service) execute(kind ActionKind, command, path, content string) (string, error) {
	switch kind {
	case ActionRun:
		ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "bash", "-lc", command)
		cmd.Dir = s.workspaceRoot
		out, err := cmd.CombinedOutput()
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out after %s", runTimeout)
		}
		if err != nil {
			if len(out) == 0 {
				return "", fmt.Errorf("command failed: %w", err)
			}
			return "", fmt.Errorf("command failed: %w\n%s", err, strings.TrimSpace(string(out)))
		}
		return strings.TrimSpace(string(out)), nil
	case ActionWrite:
		resolved, err := resolvePathInside(s.workspaceRoot, path)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
			return "", fmt.Errorf("create parent directory: %w", err)
		}
		if err := os.WriteFile(resolved, []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("write file: %w", err)
		}
		rel, err := filepath.Rel(s.workspaceRoot, resolved)
		if err != nil {
			rel = resolved
		}
		return fmt.Sprintf("wrote %s", rel), nil
	default:
		return "", fmt.Errorf("unsupported action kind: %s", kind)
	}
}

func (s *Service) loadLocked() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read approvals: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	var items []Action
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("decode approvals: %w", err)
	}
	for i := range items {
		item := items[i]
		copy := item
		s.actions[item.ID] = &copy
	}
	return nil
}

func (s *Service) saveLocked() error {
	items := make([]Action, 0, len(s.actions))
	for _, action := range s.actions {
		items = append(items, *action)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("encode approvals: %w", err)
	}
	if err := os.WriteFile(s.statePath, data, 0o600); err != nil {
		return fmt.Errorf("write approvals: %w", err)
	}
	return nil
}

func randomID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
