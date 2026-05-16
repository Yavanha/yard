package config

import "testing"

func TestProjectConfigFromMap(t *testing.T) {
	t.Parallel()

	parsed, err := ParseSimpleYAML(`vm_name: example-dev
vm_user: ubuntu
vm:
  provider: auto
  type: vz
resources:
  cpus: 4
  memory: 6G
  disk: 50G
supabase:
  enabled: true
ports:
  app: 3000
  api: 8080
services:
  api:
    command: go run ./cmd/api
    workdir: backend
  web:
    command: pnpm dev --host 0.0.0.0
    port: 3000
`)
	if err != nil {
		t.Fatal(err)
	}

	project, err := ProjectConfigFromMap(parsed)
	if err != nil {
		t.Fatalf("ProjectConfigFromMap returned error: %v", err)
	}

	assertEqual(t, project.VMName, "example-dev")
	assertEqual(t, project.VMUser, "ubuntu")
	assertEqual(t, project.VM.Provider, "auto")
	assertEqual(t, project.VM.Type, "vz")
	assertEqual(t, project.Resources.CPUs, 4)
	assertEqual(t, project.Resources.Memory, "6G")
	assertEqual(t, project.Resources.Disk, "50G")
	assertEqual(t, project.SupabaseEnabled, true)
	assertEqual(t, project.Ports[0].Name, "app")
	assertEqual(t, project.Ports[0].Port, 3000)
	assertEqual(t, project.Ports[1].Name, "api")
	assertEqual(t, project.Ports[1].Port, 8080)
	assertEqual(t, project.Services[0].Name, "api")
	assertEqual(t, project.Services[0].Command, "go run ./cmd/api")
	assertEqual(t, project.Services[0].Workdir, "backend")
	assertEqual(t, project.Services[0].Port, 8080)
	assertEqual(t, project.Services[1].Name, "web")
	assertEqual(t, project.Services[1].Port, 3000)
}

func TestProjectConfigFromMapKeepsLegacyAppService(t *testing.T) {
	t.Parallel()

	parsed, err := ParseSimpleYAML(`vm_name: example-dev
resources:
  cpus: 4
  memory: 6G
  disk: 50G
app:
  dev_command: pnpm dev --host 0.0.0.0
ports:
  app: 3000
`)
	if err != nil {
		t.Fatal(err)
	}

	project, err := ProjectConfigFromMap(parsed)
	if err != nil {
		t.Fatalf("ProjectConfigFromMap returned error: %v", err)
	}

	if len(project.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(project.Services))
	}
	assertEqual(t, project.Services[0].Name, "app")
	assertEqual(t, project.Services[0].Command, "pnpm dev --host 0.0.0.0")
	assertEqual(t, project.Services[0].Port, 3000)
}

func TestProjectConfigFromMapRequiresResources(t *testing.T) {
	t.Parallel()

	parsed, err := ParseSimpleYAML(`vm_name: example-dev
resources:
  cpus: 4
`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ProjectConfigFromMap(parsed)
	if err == nil {
		t.Fatal("expected missing resources to fail")
	}
}
