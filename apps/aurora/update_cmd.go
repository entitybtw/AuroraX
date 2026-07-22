package main

import (
	"flag"
	"fmt"
	"io"
	"os/exec"
)

func runUpdateSubcommand(args []string, stdout, stderr io.Writer) (handled bool, exitCode int) {
	if len(args) < 1 {
		return false, 0
	}

	switch args[0] {
	case "update":
		return true, updateCmd(args[1:], stdout, stderr)
	case "uninstall":
		return true, uninstallCmd(args[1:], stdout, stderr)
	default:
		return false, 0
	}
}

func updateCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	npmPath, err := exec.LookPath("npm")
	if err != nil {
		fmt.Fprintln(stderr, "error: npm not found in PATH")
		fmt.Fprintln(stderr, "  To update manually: npm install -g iaurora@latest")
		return 1
	}

	fmt.Fprintf(stdout, "  Updating iaurora to latest version...\n")
	cmd := exec.Command(npmPath, "install", "-g", "iaurora@latest")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "error: update failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "\n  iaurora updated to latest version.\n")
	return 0
}

func uninstallCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	npmPath, err := exec.LookPath("npm")
	if err != nil {
		fmt.Fprintln(stderr, "error: npm not found in PATH")
		fmt.Fprintln(stderr, "  To uninstall manually: npm uninstall -g iaurora")
		return 1
	}

	fmt.Fprintf(stdout, "  Removing iaurora...\n")
	cmd := exec.Command(npmPath, "uninstall", "-g", "iaurora")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "error: uninstall failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "\n  iaurora uninstalled.\n")
	return 0
}
