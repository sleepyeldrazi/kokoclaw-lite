package ops

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

func EvaluateRunPolicy(command string) PolicyDecision {
	command = strings.TrimSpace(command)
	if command == "" {
		return deny("empty command is not allowed")
	}

	rules := []struct {
		re     *regexp.Regexp
		reason string
	}{
		{regexp.MustCompile(`(?i)(^|[;&|])\s*rm\s+-rf\s+/(?:\s|$|[;&|])`), "destructive root delete pattern detected"},
		{regexp.MustCompile(`(?i)\bmkfs(\.[a-z0-9]+)?\b`), "filesystem formatting command detected"},
		{regexp.MustCompile(`(?i)\bdd\s+if=.*\bof=/dev/`), "raw disk overwrite pattern detected"},
		{regexp.MustCompile(`(?i)\bshutdown\b|\breboot\b|\bpoweroff\b`), "host shutdown or reboot command detected"},
		{regexp.MustCompile(`(?i):\(\)\s*\{\s*:\|:&\s*\};:`), "fork bomb pattern detected"},
		{regexp.MustCompile(`(?i)\bcurl\b[^\n|]*\|\s*(bash|sh)\b`), "remote script piping pattern detected"},
		{regexp.MustCompile(`(?i)\bwget\b[^\n|]*\|\s*(bash|sh)\b`), "remote script piping pattern detected"},
		{regexp.MustCompile(`(?i)\bsudo\b`), "sudo is blocked in the public demo"},
		{regexp.MustCompile(`(?i)\bgit\s+reset\s+--hard\b`), "hard reset is blocked"},
		{regexp.MustCompile(`(?i)\bgit\s+clean\s+-fd`), "git clean is blocked"},
	}
	for _, rule := range rules {
		if rule.re.MatchString(command) {
			return deny(rule.reason)
		}
	}
	return allow("command passed the default runtime policy")
}

func EvaluateWritePolicy(path, content string) PolicyDecision {
	path = strings.TrimSpace(path)
	if path == "" {
		return deny("write path is required")
	}

	lowerPath := strings.ToLower(filepath.ToSlash(path))
	base := strings.ToLower(filepath.Base(lowerPath))
	for _, sensitive := range []string{
		".env", ".env.local", ".env.production", ".env.development",
		"id_rsa", "id_ed25519", "authorized_keys", "known_hosts",
	} {
		if base == sensitive {
			return deny("writes to sensitive credential files are blocked")
		}
	}
	if strings.Contains(lowerPath, "/.git/") || lowerPath == ".git" {
		return deny("writes inside .git internals are blocked")
	}
	if strings.Contains(lowerPath, "/.kokoclaw-lite/") || lowerPath == ".kokoclaw-lite" {
		return deny("writes inside internal state directories are blocked")
	}

	lowerContent := strings.ToLower(content)
	for _, marker := range []string{
		"openai_api_key=",
		"openrouter_api_key=",
		"aws_secret_access_key=",
		"-----begin private key-----",
		"ghp_",
		"xoxb-",
	} {
		if strings.Contains(lowerContent, marker) {
			return deny(fmt.Sprintf("content appears to include a secret marker: %q", marker))
		}
	}

	return allow("write payload passed the default policy")
}

func allow(reason string) PolicyDecision {
	return PolicyDecision{
		Allowed:  true,
		Decision: "allow",
		Reason:   strings.TrimSpace(reason),
	}
}

func deny(reason string) PolicyDecision {
	return PolicyDecision{
		Allowed:  false,
		Decision: "deny",
		Reason:   strings.TrimSpace(reason),
	}
}
