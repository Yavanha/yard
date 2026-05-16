package config

import (
	"fmt"
	"runtime"
	"sort"
)

type ProjectConfig struct {
	VMName    string
	VMUser    string
	VM        VMConfig
	Resources ResourceConfig
	Ports     []PortMapping
}

type VMConfig struct {
	Provider string
	Type     string
}

type ResourceConfig struct {
	CPUs   int
	Memory string
	Disk   string
}

type PortMapping struct {
	Name string
	Port int
}

func ProjectConfigFromMap(values map[string]any) (ProjectConfig, error) {
	vmName, err := requiredString(values, "vm_name")
	if err != nil {
		return ProjectConfig{}, err
	}

	cpus, err := requiredNestedInt(values, "resources", "cpus")
	if err != nil {
		return ProjectConfig{}, err
	}
	memory, err := requiredNestedString(values, "resources", "memory")
	if err != nil {
		return ProjectConfig{}, err
	}
	disk, err := requiredNestedString(values, "resources", "disk")
	if err != nil {
		return ProjectConfig{}, err
	}

	ports, err := portMappings(values)
	if err != nil {
		return ProjectConfig{}, err
	}

	return ProjectConfig{
		VMName: vmName,
		VMUser: optionalString(values, "vm_user", "ubuntu"),
		VM: VMConfig{
			Provider: nestedString(values, "vm", "provider", "auto"),
			Type:     nestedString(values, "vm", "type", defaultVMType()),
		},
		Resources: ResourceConfig{
			CPUs:   cpus,
			Memory: memory,
			Disk:   disk,
		},
		Ports: ports,
	}, nil
}

func requiredString(values map[string]any, key string) (string, error) {
	value, ok := values[key]
	if !ok || value == nil || fmt.Sprint(value) == "" {
		return "", fmt.Errorf("missing required config key: %s", key)
	}
	return fmt.Sprint(value), nil
}

func optionalString(values map[string]any, key string, fallback string) string {
	value, ok := values[key]
	if !ok || value == nil || fmt.Sprint(value) == "" {
		return fallback
	}
	return fmt.Sprint(value)
}

func requiredNestedString(values map[string]any, section string, key string) (string, error) {
	value, ok := nestedValue(values, section, key)
	if !ok || value == nil || fmt.Sprint(value) == "" {
		return "", fmt.Errorf("missing required config key: %s.%s", section, key)
	}
	return fmt.Sprint(value), nil
}

func nestedString(values map[string]any, section string, key string, fallback string) string {
	value, ok := nestedValue(values, section, key)
	if !ok || value == nil || fmt.Sprint(value) == "" {
		return fallback
	}
	return fmt.Sprint(value)
}

func requiredNestedInt(values map[string]any, section string, key string) (int, error) {
	value, ok := nestedValue(values, section, key)
	if !ok || value == nil {
		return 0, fmt.Errorf("missing required config key: %s.%s", section, key)
	}
	parsed, ok := value.(int)
	if !ok {
		return 0, fmt.Errorf("config key %s.%s must be an integer", section, key)
	}
	return parsed, nil
}

func nestedValue(values map[string]any, section string, key string) (any, bool) {
	rawSection, ok := values[section]
	if !ok {
		return nil, false
	}
	sectionValues, ok := rawSection.(map[string]any)
	if !ok {
		return nil, false
	}
	value, ok := sectionValues[key]
	return value, ok
}

func portMappings(values map[string]any) ([]PortMapping, error) {
	rawPorts, ok := values["ports"]
	if !ok {
		return nil, nil
	}
	portsMap, ok := rawPorts.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("config key ports must be a map")
	}

	ports := make([]PortMapping, 0, len(portsMap))
	for name, rawPort := range portsMap {
		port, ok := rawPort.(int)
		if !ok {
			return nil, fmt.Errorf("config key ports.%s must be an integer", name)
		}
		if port <= 0 || port > 65535 {
			return nil, fmt.Errorf("config key ports.%s is out of range", name)
		}
		ports = append(ports, PortMapping{Name: name, Port: port})
	}

	sort.Slice(ports, func(left int, right int) bool {
		if ports[left].Port == ports[right].Port {
			return ports[left].Name < ports[right].Name
		}
		return ports[left].Port < ports[right].Port
	})
	return ports, nil
}

func defaultVMType() string {
	if runtime.GOOS == "darwin" {
		return "vz"
	}
	return "qemu"
}
