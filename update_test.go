package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdaterReplacesOnlyVerifiedBuilds(t *testing.T) {
	for _, scenario := range []string{"success", "unchanged", "offline", "build failure", "verification failure"} {
		t.Run(scenario, func(t *testing.T) {
			source := fixture(t)
			writeFile(t, source.home, "go.mod", "module example.invalid/update-fixture\n\ngo 1.24.0\n")
			program := "package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"new version\") }\n"
			if scenario == "verification failure" {
				program = "package main\nimport \"os\"\nfunc main() { os.Exit(1) }\n"
			}
			if scenario == "build failure" {
				program = "invalid Go program"
			}
			writeFile(t, source.home, "main.go", program)
			for _, path := range []string{"go.mod", "main.go"} {
				if _, err := source.add(path); err != nil {
					t.Fatal(err)
				}
			}
			mustGit(t, source, "commit", "-m", "new version")
			previous := buildRevision
			t.Cleanup(func() { buildRevision = previous })
			buildRevision = ""
			if scenario == "unchanged" {
				buildRevision = strings.TrimSpace(mustGit(t, source, "rev-parse", "HEAD"))
			}
			dir := t.TempDir()
			binary := filepath.Join(dir, "mydotty")
			original := "#!/bin/sh\nprintf 'old version\\n'\n"
			writeFile(t, dir, "mydotty", original)
			if err := os.Chmod(binary, 0755); err != nil {
				t.Fatal(err)
			}
			url := source.dir
			if scenario == "offline" {
				url = filepath.Join(dir, "missing.git")
			}
			var out bytes.Buffer
			err := updateBinary(binary, url, &out)
			wantErr := scenario == "offline" || scenario == "build failure" || scenario == "verification failure"
			if (err != nil) != wantErr {
				t.Fatalf("unexpected update result: %v", err)
			}
			if scenario == "success" {
				output, err := exec.Command(binary, "--help").CombinedOutput()
				if err != nil || string(output) != "new version\n" {
					t.Fatalf("installed binary: %q, %v", output, err)
				}
			} else {
				data, err := os.ReadFile(binary)
				if err != nil || string(data) != original {
					t.Fatalf("existing executable changed: %q, %v", data, err)
				}
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".mydotty-update-") {
					t.Fatalf("temporary build left behind: %s", entry.Name())
				}
			}
		})
	}
}
