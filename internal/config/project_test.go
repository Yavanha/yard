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
ports:
  app: 3000
  api: 8080
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
	assertEqual(t, project.Ports[0].Name, "app")
	assertEqual(t, project.Ports[0].Port, 3000)
	assertEqual(t, project.Ports[1].Name, "api")
	assertEqual(t, project.Ports[1].Port, 8080)
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
