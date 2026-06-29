package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// clientCommentPrefix marks lines this binary adds to the edit buffer (instructions / error
// banner). They are stripped each round so they don't accumulate, and the server ignores them
// as YAML comments anyway.
const clientCommentPrefix = "#| "

// runEditor opens the resource YAML in $EDITOR and returns the edited content plus whether the
// user actually changed anything (an unchanged or empty buffer means "abort").
func runEditor(edit *EditAction) (string, bool) {
	// Drop our own previous comment lines; keep the YAML body and any user '#' comments.
	body := stripClientLines(edit.Content)

	var b strings.Builder
	b.WriteString(clientCommentPrefix + "Edit the resource below, then save and close the editor to apply.\n")
	b.WriteString(clientCommentPrefix + "Lines starting with '#' are ignored. An empty file or no change aborts.\n")
	if edit.Error != "" {
		for _, line := range strings.Split(strings.TrimRight(edit.Error, "\n"), "\n") {
			b.WriteString(clientCommentPrefix + "ERROR: " + line + "\n")
		}
	}
	b.WriteString(body)
	initial := b.String()

	f, err := os.CreateTemp("", "kc-edit-*.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot create temp file: %v\n", err)
		os.Exit(1)
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.WriteString(initial); err != nil {
		_ = f.Close()
		fmt.Fprintf(os.Stderr, "error: cannot write temp file: %v\n", err)
		os.Exit(1)
	}
	_ = f.Close()

	if err := launchEditor(tmp); err != nil {
		fmt.Fprintf(os.Stderr, "error: editor failed: %v\n", err)
		os.Exit(1)
	}

	out, err := os.ReadFile(tmp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot read temp file: %v\n", err)
		os.Exit(1)
	}
	edited := string(out)
	changed := edited != initial && strings.TrimSpace(stripAllComments(edited)) != ""
	return edited, changed
}

// launchEditor runs $KC_EDITOR || $EDITOR || vi on the file, inheriting the terminal.
func launchEditor(path string) error {
	editor := os.Getenv("KC_EDITOR")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	parts = append(parts, path)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func stripClientLines(s string) string {
	var kept []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, clientCommentPrefix) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func stripAllComments(s string) string {
	var kept []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
