package config

import (
	"strings"
	"testing"
)

func TestRenderScaffold(t *testing.T) {
	t.Parallel()

	rendered, err := RenderScaffold(ScaffoldOptions{
		Project:        "api",
		Repo:           "git@github.com:acme/api.git",
		VMName:         "api-dev",
		VMUser:         "ubuntu",
		RepoDir:        "/home/ubuntu/workspaces/api",
		VMProvider:     "auto",
		VMType:         "vz",
		CPUs:           4,
		Memory:         "6G",
		Disk:           "50G",
		ServiceName:    "web",
		ServiceCommand: "pnpm dev --host 0.0.0.0",
		ServiceWorkdir: ".",
		ServicePort:    3000,
	})
	if err != nil {
		t.Fatalf("RenderScaffold returned error: %v", err)
	}

	content := string(rendered)
	for _, expected := range []string{
		`project: "api"`,
		`repo: "git@github.com:acme/api.git"`,
		`vm_name: "api-dev"`,
		`resources:`,
		`services:`,
		`  web:`,
		`    command: "pnpm dev --host 0.0.0.0"`,
		`    port: 3000`,
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected scaffold to contain %q:\n%s", expected, content)
		}
	}
}

func TestRenderScaffoldUsesDefaults(t *testing.T) {
	t.Parallel()

	rendered, err := RenderScaffold(ScaffoldOptions{Project: "api"})
	if err != nil {
		t.Fatalf("RenderScaffold returned error: %v", err)
	}

	content := string(rendered)
	for _, expected := range []string{
		`vm_name: "api-dev"`,
		`repo_dir: "/home/ubuntu/workspaces/api"`,
		`  cpus: 4`,
		`  app: 3000`,
		`    command: "make dev"`,
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected scaffold to contain %q:\n%s", expected, content)
		}
	}

	parsed, err := ParseSimpleYAML(content)
	if err != nil {
		t.Fatalf("ParseSimpleYAML returned error: %v", err)
	}
	project, err := ProjectConfigFromMap(parsed)
	if err != nil {
		t.Fatalf("ProjectConfigFromMap returned error: %v", err)
	}
	assertEqual(t, project.Services[0].Command, "make dev")
}

func TestRenderScaffoldRejectsUnsafeServiceName(t *testing.T) {
	t.Parallel()

	_, err := RenderScaffold(ScaffoldOptions{
		Project:        "api",
		ServiceName:    "../web",
		ServiceCommand: "pnpm dev",
	})
	if err == nil {
		t.Fatal("expected unsafe service name to fail")
	}
}
