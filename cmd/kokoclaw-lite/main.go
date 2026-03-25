package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/sleepyeldrazi/kokoclaw-lite/internal/ops"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "request":
		runRequest(os.Args[2:])
	case "approvals":
		runApprovals(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func runRequest(args []string) {
	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "run":
		fs := flag.NewFlagSet("request run", flag.ExitOnError)
		workspace := fs.String("workspace", ".", "workspace root")
		user := fs.String("user", "operator", "requesting user")
		command := fs.String("command", "", "shell command to queue")
		_ = fs.Parse(args[1:])
		if strings.TrimSpace(*command) == "" && fs.NArg() > 0 {
			*command = strings.Join(fs.Args(), " ")
		}
		svc := mustService(*workspace)
		action, err := svc.QueueRun(*user, *command)
		if err != nil {
			log.Fatal(err)
		}
		printAction(action)
	case "write":
		fs := flag.NewFlagSet("request write", flag.ExitOnError)
		workspace := fs.String("workspace", ".", "workspace root")
		user := fs.String("user", "operator", "requesting user")
		path := fs.String("path", "", "path to write inside the workspace")
		content := fs.String("content", "", "file content")
		_ = fs.Parse(args[1:])
		svc := mustService(*workspace)
		action, err := svc.QueueWrite(*user, *path, *content)
		if err != nil {
			log.Fatal(err)
		}
		printAction(action)
	default:
		printUsage()
		os.Exit(1)
	}
}

func runApprovals(args []string) {
	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("approvals list", flag.ExitOnError)
		workspace := fs.String("workspace", ".", "workspace root")
		_ = fs.Parse(args[1:])
		svc := mustService(*workspace)
		printList(svc.List())
	case "approve":
		fs := flag.NewFlagSet("approvals approve", flag.ExitOnError)
		workspace := fs.String("workspace", ".", "workspace root")
		_ = fs.Parse(args[1:])
		if fs.NArg() != 1 {
			log.Fatal("approvals approve requires an action id")
		}
		svc := mustService(*workspace)
		action, err := svc.Approve(fs.Arg(0))
		if err != nil {
			log.Fatal(err)
		}
		printAction(action)
	case "override":
		fs := flag.NewFlagSet("approvals override", flag.ExitOnError)
		workspace := fs.String("workspace", ".", "workspace root")
		reason := fs.String("reason", "operator override", "override reason")
		_ = fs.Parse(args[1:])
		if fs.NArg() != 1 {
			log.Fatal("approvals override requires an action id")
		}
		svc := mustService(*workspace)
		action, err := svc.Override(fs.Arg(0), *reason)
		if err != nil {
			log.Fatal(err)
		}
		printAction(action)
	case "deny":
		fs := flag.NewFlagSet("approvals deny", flag.ExitOnError)
		workspace := fs.String("workspace", ".", "workspace root")
		_ = fs.Parse(args[1:])
		if fs.NArg() != 1 {
			log.Fatal("approvals deny requires an action id")
		}
		svc := mustService(*workspace)
		action, err := svc.Deny(fs.Arg(0))
		if err != nil {
			log.Fatal(err)
		}
		printAction(action)
	default:
		printUsage()
		os.Exit(1)
	}
}

func mustService(workspace string) *ops.Service {
	svc, err := ops.NewService(workspace)
	if err != nil {
		log.Fatal(err)
	}
	return svc
}

func printList(actions []ops.Action) {
	if len(actions) == 0 {
		fmt.Println("No approvals queued.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tKIND\tREQUESTED BY\tPOLICY\tUPDATED")
	for _, action := range actions {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			action.ID,
			action.Status,
			action.Kind,
			action.RequestedBy,
			action.PolicyDecision,
			action.UpdatedAt.Format("2006-01-02 15:04:05"),
		)
	}
	_ = w.Flush()
}

func printAction(action ops.Action) {
	fmt.Printf("ID: %s\n", action.ID)
	fmt.Printf("Status: %s\n", action.Status)
	fmt.Printf("Kind: %s\n", action.Kind)
	fmt.Printf("Requested By: %s\n", action.RequestedBy)
	fmt.Printf("Policy: %s\n", action.PolicyDecision)
	if strings.TrimSpace(action.PolicyReason) != "" {
		fmt.Printf("Policy Reason: %s\n", action.PolicyReason)
	}
	if strings.TrimSpace(action.Path) != "" {
		fmt.Printf("Path: %s\n", action.Path)
	}
	if strings.TrimSpace(action.Command) != "" {
		fmt.Printf("Command: %s\n", action.Command)
	}
	if strings.TrimSpace(action.Result) != "" {
		fmt.Printf("Result:\n%s\n", action.Result)
	}
	if strings.TrimSpace(action.Error) != "" {
		fmt.Printf("Error:\n%s\n", action.Error)
	}
}

func printUsage() {
	fmt.Println(`kokoclaw-lite

Public demo of an approval-gated local operator loop.

Usage:
  kokoclaw-lite request run --workspace . --user alice --command "git status --short"
  kokoclaw-lite request write --workspace . --user alice --path notes.txt --content "hello"
  kokoclaw-lite approvals list --workspace .
  kokoclaw-lite approvals approve --workspace . <id>
  kokoclaw-lite approvals override --workspace . --reason "local demo only" <id>
  kokoclaw-lite approvals deny --workspace . <id>`)
}
