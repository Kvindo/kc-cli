package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// version is the semantic client version, baked in at build time via
// -ldflags "-X main.version=vX.Y.Z" (see the Makefile). "dev" for un-tagged local builds.
var version = "dev"

func main() {
	profileName := os.Getenv("KC_PROFILE")
	if profileName == "" {
		profileName = activeProfile()
	}

	p := loadProfile(profileName)

	if v := os.Getenv("KC_API_URL"); v != "" {
		p.Server = v
	}
	if v := os.Getenv("KC_TOKEN"); v != "" {
		p.Token = v
	}

	args := os.Args[1:]

	// `kc version` is handled locally so it always reports the baked-in semantic client version and
	// works offline. We also show the server version, but never fail the command if it's unreachable.
	if len(args) >= 1 && (args[0] == "version" || args[0] == "--version" || args[0] == "-v") {
		fmt.Printf("kc %s\n", version)
		if _, body, err := post(p.Server, p.Token, []string{"version"}, nil, nil, ""); err == nil {
			var sr Response
			if json.Unmarshal(body, &sr) == nil && sr.Stdout != "" {
				fmt.Print(sr.Stdout)
				if !strings.HasSuffix(sr.Stdout, "\n") {
					fmt.Println()
				}
			}
		}
		return
	}

	status, body, err := postRetrying(p.Server, p.Token, args, nil, nil, "")
	if err != nil {
		printContext(profileName, p.Server, p.Token, 0, nil)
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if status == 401 {
		token := promptToken(profileName)
		p.Token = token
		_ = saveProfile(profileName, p)

		status, body, err = post(p.Server, token, args, nil, nil, "")
		if err != nil {
			printContext(profileName, p.Server, token, 0, nil)
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if status == 401 {
			printContext(profileName, p.Server, p.Token, status, body)
			fmt.Fprintln(os.Stderr, "error: authentication failed")
			os.Exit(1)
		}
	}

	if status == 205 {
		if confirm(fmt.Sprintf("Delete saved token for profile %q? [y/N]", profileName)) {
			_ = clearToken(profileName)
		}
		os.Exit(0)
	}

	if len(body) == 0 {
		printContext(profileName, p.Server, p.Token, status, nil)
		fmt.Fprintln(os.Stderr, "error: server returned empty response")
		os.Exit(1)
	}

	var r Response
	if err := json.Unmarshal(body, &r); err != nil {
		printContext(profileName, p.Server, p.Token, status, body)
		fmt.Fprintln(os.Stderr, "error: response is not valid JSON")
		os.Exit(1)
	}

	// Long-running operations (e.g. delete --wait) return a non-nil RetryParams: re-send the
	// same args plus the echoed RetryParams, polling until the server returns it nil. A non-zero
	// rc at any point stops the loop and is honored. Unbounded by design (some deletes take an
	// hour); each request still has the 60s client timeout.
	for r.RetryParams != nil && r.Rc == 0 {
		time.Sleep(2 * time.Second)
		status, body, err = postRetrying(p.Server, p.Token, args, r.RetryParams, nil, "")
		if err != nil {
			printContext(profileName, p.Server, p.Token, 0, nil)
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if len(body) == 0 {
			printContext(profileName, p.Server, p.Token, status, nil)
			fmt.Fprintln(os.Stderr, "error: server returned empty response")
			os.Exit(1)
		}
		r = Response{}
		if err := json.Unmarshal(body, &r); err != nil {
			printContext(profileName, p.Server, p.Token, status, body)
			fmt.Fprintln(os.Stderr, "error: response is not valid JSON")
			os.Exit(1)
		}
	}

	// kc apply -f <file>: the server parsed `-f` and ordered us to read a file. Read it and re-POST
	// the same args with its contents in fileContent. We parse nothing — just obey the order, the
	// same way the Edit/RetryParams loops below/above do. Looping handles a chained order if any.
	for r.ReadFile != nil {
		var content []byte
		var readErr error
		// "-" means stdin: cat <<EOF | kc apply -f -
		if r.ReadFile.Path == "-" {
			content, readErr = io.ReadAll(os.Stdin)
		} else {
			content, readErr = os.ReadFile(expandHome(r.ReadFile.Path))
		}
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "error: cannot read %s: %v\n", r.ReadFile.Path, readErr)
			os.Exit(1)
		}
		status, body, err = postRetrying(p.Server, p.Token, args, nil, nil, string(content))
		if err != nil {
			printContext(profileName, p.Server, p.Token, 0, nil)
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if len(body) == 0 {
			printContext(profileName, p.Server, p.Token, status, nil)
			fmt.Fprintln(os.Stderr, "error: server returned empty response")
			os.Exit(1)
		}
		r = Response{}
		if err := json.Unmarshal(body, &r); err != nil {
			printContext(profileName, p.Server, p.Token, status, body)
			fmt.Fprintln(os.Stderr, "error: response is not valid JSON")
			os.Exit(1)
		}
	}

	// kc edit: the server returns an Edit action; open it in $EDITOR and re-POST the result. If the
	// save fails to parse/validate the server returns Edit again (with Error set) and we re-open —
	// looping until it applies or the user makes no change.
	for r.Edit != nil {
		edited, changed := runEditor(r.Edit)
		if !changed {
			fmt.Fprintln(os.Stderr, "Edit cancelled, no changes made.")
			if r.Edit.Error != "" {
				os.Exit(1) // leaving an unresolved error
			}
			os.Exit(0)
		}
		apply := &EditApply{Type: r.Edit.Type, ResourceId: r.Edit.ResourceId, Content: edited}
		status, body, err = postRetrying(p.Server, p.Token, args, nil, apply, "")
		if err != nil {
			printContext(profileName, p.Server, p.Token, 0, nil)
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if len(body) == 0 {
			printContext(profileName, p.Server, p.Token, status, nil)
			fmt.Fprintln(os.Stderr, "error: server returned empty response")
			os.Exit(1)
		}
		r = Response{}
		if err := json.Unmarshal(body, &r); err != nil {
			printContext(profileName, p.Server, p.Token, status, body)
			fmt.Fprintln(os.Stderr, "error: response is not valid JSON")
			os.Exit(1)
		}
	}

	handleResponse(r, profileName)
}

func printContext(profile, server, token string, status int, body []byte) {
	fmt.Fprintln(os.Stderr, "---")
	fmt.Fprintf(os.Stderr, "  profile : %s\n", profile)
	fmt.Fprintf(os.Stderr, "  server  : %s\n", server)
	if token == "" {
		fmt.Fprintf(os.Stderr, "  token   : (none)\n")
	} else {
		sum := sha256.Sum256([]byte(token))
		fmt.Fprintf(os.Stderr, "  token   : sha256:%x\n", sum[:8])
	}
	if status != 0 {
		fmt.Fprintf(os.Stderr, "  http    : %d\n", status)
	}
	if len(body) > 0 {
		fmt.Fprintf(os.Stderr, "  body    : %s\n", body)
	}
	fmt.Fprintln(os.Stderr, "---")
}

func promptToken(profileName string) string {
	fmt.Fprintf(os.Stderr, "Enter token for profile %q: ", profileName)
	token, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line)
	}
	return strings.TrimSpace(string(token))
}

func confirm(prompt string) bool {
	fmt.Fprintf(os.Stderr, "%s: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}
