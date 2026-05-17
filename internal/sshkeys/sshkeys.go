package sshkeys

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Runner interface {
	Output(command string, args ...string) ([]byte, error)
	Run(command string, args ...string) error
}

type ExecRunner struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

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

func (r ExecRunner) Run(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdin = r.stdin()
	cmd.Stdout = r.stdout()
	cmd.Stderr = r.stderr()
	return cmd.Run()
}

func (r ExecRunner) stdin() io.Reader {
	if r.Stdin != nil {
		return r.Stdin
	}
	return os.Stdin
}

func (r ExecRunner) stdout() io.Writer {
	if r.Stdout != nil {
		return r.Stdout
	}
	return os.Stdout
}

func (r ExecRunner) stderr() io.Writer {
	if r.Stderr != nil {
		return r.Stderr
	}
	return os.Stderr
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

func (d Detector) Create(identityFile string, comment string) (KeyCandidate, error) {
	if identityFile == "" {
		return KeyCandidate{}, fmt.Errorf("SSH identity file is required")
	}
	if err := ensureKeyDoesNotExist(identityFile); err != nil {
		return KeyCandidate{}, err
	}
	if err := os.MkdirAll(filepath.Dir(identityFile), 0o700); err != nil {
		return KeyCandidate{}, err
	}
	if err := d.Runner.Run("ssh-keygen", "-t", "ed25519", "-f", identityFile, "-C", comment); err != nil {
		return KeyCandidate{}, fmt.Errorf("create SSH key: %w", err)
	}
	fingerprint, err := d.FingerprintForIdentity(identityFile)
	if err != nil {
		return KeyCandidate{}, err
	}
	return KeyCandidate{
		Path:        identityFile,
		Comment:     comment,
		Fingerprint: fingerprint,
	}, nil
}

func (d Detector) PublicKey(identityFile string) (string, error) {
	content, err := os.ReadFile(identityFile + ".pub")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func (d Detector) FingerprintForIdentity(identityFile string) (string, error) {
	fingerprint, _, err := d.fingerprint(identityFile + ".pub")
	if err != nil {
		return "", err
	}
	return fingerprint, nil
}

func DefaultIdentityPath(sshDir string, repoURL string) string {
	org, repo := repoParts(repoURL)
	return filepath.Join(sshDir, "yard_"+safeKeyName(org)+"_"+safeKeyName(repo)+"_ed25519")
}

func ensureKeyDoesNotExist(identityFile string) error {
	for _, path := range []string{identityFile, identityFile + ".pub"} {
		_, err := os.Stat(path)
		switch {
		case err == nil:
			return fmt.Errorf("SSH key already exists: %s", path)
		case os.IsNotExist(err):
			continue
		default:
			return err
		}
	}
	return nil
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

func repoParts(repoURL string) (string, string) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(repoURL), "/")
	trimmed = strings.TrimSuffix(trimmed, ".git")

	pathPart := trimmed
	if _, after, ok := strings.Cut(trimmed, ":"); ok && strings.Contains(trimmed, "@") {
		pathPart = after
	} else if strings.Contains(trimmed, "://") {
		if _, after, ok := strings.Cut(trimmed, "://"); ok {
			pathPart = after
		}
		parts := strings.Split(pathPart, "/")
		if len(parts) > 1 {
			pathPart = strings.Join(parts[1:], "/")
		}
	}

	parts := strings.Split(pathPart, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2], parts[len(parts)-1]
	}
	if len(parts) == 1 && parts[0] != "" {
		return "repo", parts[0]
	}
	return "repo", "project"
}

var unsafeKeyNameChar = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func safeKeyName(value string) string {
	normalized := strings.ToLower(strings.Trim(value, " ._-"))
	normalized = unsafeKeyNameChar.ReplaceAllString(normalized, "_")
	normalized = strings.Trim(normalized, "_")
	if normalized == "" {
		return "project"
	}
	return normalized
}
