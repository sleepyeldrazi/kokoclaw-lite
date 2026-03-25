package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveWorkspaceRoot(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat workspace: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace must be a directory")
	}
	return abs, nil
}

func resolvePathInside(root, target string) (string, error) {
	root, err := resolveWorkspaceRoot(root)
	if err != nil {
		return "", err
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("target path is required")
	}

	candidate := target
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	absTarget, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve target path: %w", err)
	}
	rel, err := filepath.Rel(root, absTarget)
	if err != nil {
		return "", fmt.Errorf("check target path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	return absTarget, nil
}
