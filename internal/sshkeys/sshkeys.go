package sshkeys

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Runner interface {
	Output(command string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

type Detector struct {
	Runner Runner
	SSHDir string
}

type KeyCandidate struct {
	Path        string
	Comment     string
	Fingerprint string
	InAgent     bool
}

type agentKey struct {
	Fingerprint string
	Comment     string
}

func NewDetector(runner Runner, sshDir string) Detector {
	if runner == nil {
		runner = ExecRunner{}
	}
	return Detector{
		Runner: runner,
		SSHDir: sshDir,
	}
}

func (r ExecRunner) Output(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

func (d Detector) List() ([]KeyCandidate, error) {
	agentKeys, err := d.listAgentKeys()
	if err != nil {
		agentKeys = nil
	}

	candidates := map[string]KeyCandidate{}
	for _, publicKeyPath := range publicKeyPaths(d.SSHDir) {
		fingerprint, fallbackComment, err := d.fingerprint(publicKeyPath)
		if err != nil {
			return nil, err
		}
		comment := publicKeyComment(publicKeyPath)
		if comment == "" {
			comment = fallbackComment
		}

		candidates[fingerprint] = KeyCandidate{
			Path:        strings.TrimSuffix(publicKeyPath, ".pub"),
			Comment:     comment,
			Fingerprint: fingerprint,
		}
	}

	for _, agentKey := range agentKeys {
		candidate, ok := candidates[agentKey.Fingerprint]
		if ok {
			candidate.InAgent = true
			if candidate.Comment == "" {
				candidate.Comment = agentKey.Comment
			}
			candidates[agentKey.Fingerprint] = candidate
			continue
		}

		candidates[agentKey.Fingerprint] = KeyCandidate{
			Comment:     agentKey.Comment,
			Fingerprint: agentKey.Fingerprint,
			InAgent:     true,
		}
	}

	rows := make([]KeyCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		rows = append(rows, candidate)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Path == "" && rows[j].Path != "" {
			return false
		}
		if rows[i].Path != "" && rows[j].Path == "" {
			return true
		}
		if rows[i].Path != rows[j].Path {
			return rows[i].Path < rows[j].Path
		}
		return rows[i].Fingerprint < rows[j].Fingerprint
	})
	return rows, nil
}

func (d Detector) FingerprintForIdentity(identityFile string) (string, error) {
	fingerprint, _, err := d.fingerprint(identityFile + ".pub")
	if err != nil {
		return "", err
	}
	return fingerprint, nil
}

func (d Detector) listAgentKeys() ([]agentKey, error) {
	output, err := d.Runner.Output("ssh-add", "-l")
	if err != nil && strings.TrimSpace(string(output)) == "" {
		return nil, err
	}
	return parseIdentityList(output), nil
}

func (d Detector) fingerprint(publicKeyPath string) (string, string, error) {
	output, err := d.Runner.Output("ssh-keygen", "-lf", publicKeyPath)
	if err != nil {
		return "", "", fmt.Errorf("read SSH key fingerprint for %s: %w", publicKeyPath, err)
	}

	keys := parseIdentityList(output)
	if len(keys) == 0 {
		return "", "", fmt.Errorf("read SSH key fingerprint for %s: empty ssh-keygen output", publicKeyPath)
	}
	return keys[0].Fingerprint, keys[0].Comment, nil
}

func publicKeyPaths(sshDir string) []string {
	matches, err := filepath.Glob(filepath.Join(sshDir, "*.pub"))
	if err != nil {
		return nil
	}
	paths := make([]string, 0, len(matches))
	for _, path := range matches {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

func publicKeyComment(publicKeyPath string) string {
	content, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 3 {
			return strings.Join(fields[2:], " ")
		}
	}
	return ""
}

func parseIdentityList(content []byte) []agentKey {
	lines := strings.Split(string(content), "\n")
	keys := make([]agentKey, 0, len(lines))
	for _, line := range lines {
		key, ok := parseIdentityLine(line)
		if ok {
			keys = append(keys, key)
		}
	}
	return keys
}

func parseIdentityLine(line string) (agentKey, bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 2 {
		return agentKey{}, false
	}
	if !strings.Contains(fields[1], ":") {
		return agentKey{}, false
	}

	comment := strings.Join(fields[2:], " ")
	if index := strings.LastIndex(comment, " ("); index >= 0 && strings.HasSuffix(comment, ")") {
		comment = comment[:index]
	}

	return agentKey{
		Fingerprint: fields[1],
		Comment:     strings.TrimSpace(comment),
	}, true
}
