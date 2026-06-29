package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Response struct {
	Stdout      string          `json:"stdout"`
	Stderr      string          `json:"stderr"`
	Rc          int             `json:"rc"`
	File        *FileResp       `json:"file,omitempty"`
	Config      *ConfigAction   `json:"config,omitempty"`
	RetryParams *RetryParams    `json:"retryParams,omitempty"`
	Edit        *EditAction     `json:"edit,omitempty"`
	ReadFile    *ReadFileAction `json:"readFile,omitempty"`
}

// ReadFileAction (kc apply -f): the server parsed `-f <path>` and orders us to read that file and
// re-POST the same args with its contents in fileContent. The binary parses no flags itself — it
// only obeys this order, mirroring how Edit/RetryParams drive their round-trips.
type ReadFileAction struct {
	Path string `json:"path"`
}

// EditAction (kc edit): open Content in $EDITOR and re-POST the edited YAML. A non-empty Error
// means the previous save failed to apply — re-open showing the error so the user can fix it.
type EditAction struct {
	Content    string `json:"content"`
	Type       string `json:"type"`
	ResourceId string `json:"resourceId"`
	Error      string `json:"error"`
}

// EditApply is what we send back after the user edits in $EDITOR.
type EditApply struct {
	Type       string `json:"type"`
	ResourceId string `json:"resourceId"`
	Content    string `json:"content"`
}

// RetryParams is returned for long-running operations (e.g. delete --wait). While it is
// non-nil the binary re-sends the same request with this echoed back, polling until the
// server returns it nil. It carries the in-flight change request so the server can report
// its status without re-running the command.
type RetryParams struct {
	Type            string `json:"type"`
	ChangeRequestId string `json:"changeRequestId"`
	ResourceId      string `json:"resourceId"`
	Operation       string `json:"operation,omitempty"`
}

type FileResp struct {
	Content     string `json:"content"`
	Filename    string `json:"filename,omitempty"`
	Destination string `json:"destination,omitempty"`
}

type ConfigAction struct {
	Action  string  `json:"action"`
	Profile string  `json:"profile,omitempty"`
	Server  *string `json:"server,omitempty"`
	Token   *string `json:"token,omitempty"`
}

func handleResponse(r Response, profileName string) {
	if r.File != nil {
		handleFile(r.File)
	}
	if r.Config != nil {
		handleConfig(r.Config, profileName)
	}
	if r.Stdout != "" {
		fmt.Fprint(os.Stdout, r.Stdout)
		if !strings.HasSuffix(r.Stdout, "\n") {
			fmt.Fprintln(os.Stdout)
		}
	}
	if r.Stderr != "" {
		fmt.Fprint(os.Stderr, r.Stderr)
		if !strings.HasSuffix(r.Stderr, "\n") {
			fmt.Fprintln(os.Stderr)
		}
	}
	os.Exit(r.Rc)
}

func handleFile(f *FileResp) {
	data, err := base64.StdEncoding.DecodeString(f.Content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error decoding file: %v\n", err)
		return
	}
	dest := expandHome(f.Destination)
	if dest == "" {
		name := f.Filename
		if name == "" {
			name = "download"
		}
		dest = name
	}
	if dir := filepath.Dir(dest); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating directory: %v\n", err)
			return
		}
	}
	if err := os.WriteFile(dest, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "error writing file: %v\n", err)
	}
}

func handleConfig(c *ConfigAction, profileName string) {
	target := profileName
	if c.Profile != "" {
		target = c.Profile
	}
	switch c.Action {
	case "write":
		p := loadProfile(target)
		if c.Server != nil {
			p.Server = *c.Server
		}
		if c.Token != nil {
			p.Token = *c.Token
		}
		if err := saveProfile(target, p); err != nil {
			fmt.Fprintf(os.Stderr, "error saving profile: %v\n", err)
		}
	case "use":
		if err := setActiveProfile(target); err != nil {
			fmt.Fprintf(os.Stderr, "error setting active profile: %v\n", err)
		}
	case "read":
		data, err := os.ReadFile(filepath.Join(configDir(), target+".yaml"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading profile: %v\n", err)
			return
		}
		fmt.Print(string(data))
	case "list":
		entries, err := os.ReadDir(configDir())
		if err != nil {
			fmt.Fprintf(os.Stderr, "error listing profiles: %v\n", err)
			return
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
				fmt.Println(strings.TrimSuffix(e.Name(), ".yaml"))
			}
		}
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
