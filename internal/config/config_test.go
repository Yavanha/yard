package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSimpleYAML(t *testing.T) {
	t.Parallel()

	parsed, err := ParseSimpleYAML(`org: lmdlp
project: lmdlp-client
enabled: true
resources:
  cpus: 4
  memory: 6G
  quoted: "hello"
`)
	if err != nil {
		t.Fatalf("ParseSimpleYAML returned error: %v", err)
	}

	assertEqual(t, parsed["org"], "lmdlp")
	assertEqual(t, parsed["enabled"], true)

	resources := parsed["resources"].(map[string]any)
	assertEqual(t, resources["cpus"], 4)
	assertEqual(t, resources["memory"], "6G")
	assertEqual(t, resources["quoted"], "hello")
}

func TestParseSimpleYAMLRejectsUnsupportedLines(t *testing.T) {
	t.Parallel()

	_, err := ParseSimpleYAML(`items:
  - one
`)
	if err == nil {
		t.Fatal("expected unsupported list syntax to fail")
	}
}

func TestFindPathWalksUpward(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, FileName)
	if err := os.WriteFile(configPath, []byte("project: example\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	found, ok := FindPath(nested)
	if !ok {
		t.Fatal("expected config path to be found")
	}
	if found != configPath {
		t.Fatalf("expected %q, got %q", configPath, found)
	}
}

func TestResolveProjectPathUsesDirectoryConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	configPath := filepath.Join(root, FileName)
	if err := os.WriteFile(configPath, []byte("project: example\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	resolved, ok := ResolveProjectPath(root)
	if !ok {
		t.Fatal("expected project directory config path to resolve")
	}
	if resolved != configPath {
		t.Fatalf("expected %q, got %q", configPath, resolved)
	}
}

func assertEqual(t *testing.T, got any, want any) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}
