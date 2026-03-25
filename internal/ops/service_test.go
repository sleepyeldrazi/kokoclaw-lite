package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueueWriteMarksEnvAsPolicyDenied(t *testing.T) {
	svc, err := NewService(t.TempDir())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	action, err := svc.QueueWrite("alice", ".env", "OPENAI_API_KEY=secret")
	if err != nil {
		t.Fatalf("queue write: %v", err)
	}
	if got := action.PolicyDecision; got != "deny" {
		t.Fatalf("policy decision = %q want deny", got)
	}
	if _, err := svc.Approve(action.ID); err == nil {
		t.Fatal("expected approve to reject policy-denied action")
	}
}

func TestApproveRunExecutesInWorkspace(t *testing.T) {
	root := t.TempDir()
	svc, err := NewService(root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	action, err := svc.QueueRun("alice", "printf 'hello' > note.txt")
	if err != nil {
		t.Fatalf("queue run: %v", err)
	}
	if _, err := svc.Approve(action.ID); err != nil {
		t.Fatalf("approve run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "note.txt"))
	if err != nil {
		t.Fatalf("read note.txt: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("note.txt = %q want hello", string(data))
	}
}

func TestWriteCannotEscapeWorkspace(t *testing.T) {
	root := t.TempDir()
	svc, err := NewService(root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	action, err := svc.QueueWrite("alice", "../escape.txt", "nope")
	if err != nil {
		t.Fatalf("queue write: %v", err)
	}
	if action.PolicyDecision != "allow" {
		t.Fatalf("unexpected policy decision = %q", action.PolicyDecision)
	}

	_, err = svc.Approve(action.ID)
	if err == nil {
		t.Fatal("expected approve to fail for escaping path")
	}
	if !strings.Contains(err.Error(), "escapes workspace") {
		t.Fatalf("unexpected error: %v", err)
	}
}
