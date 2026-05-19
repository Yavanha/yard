package runtime

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"yard/internal/registry"
)

type RemoteSSH struct {
	runner Runner
	remote registry.RemoteServer
}

func NewRemoteSSH(runner Runner, remote registry.RemoteServer) RemoteSSH {
	if runner == nil {
		runner = ExecRunner{}
	}
	return RemoteSSH{
		runner: runner,
		remote: remote,
	}
}

func (target RemoteSSH) Exec(command []string) error {
	if len(command) == 0 {
		return errors.New("exec requires a command")
	}
	if err := validateRemote(target.remote); err != nil {
		return err
	}
	if err := target.CheckHostKey(); err != nil {
		return err
	}
	return target.runner.Run("ssh", RemoteSSHArgs(target.remote, command)...)
}

func (target RemoteSSH) ExecOutput(command []string) ([]byte, error) {
	if len(command) == 0 {
		return nil, errors.New("exec requires a command")
	}
	if err := validateRemote(target.remote); err != nil {
		return nil, err
	}
	if err := target.CheckHostKey(); err != nil {
		return nil, err
	}
	return target.runner.Output("ssh", RemoteSSHArgs(target.remote, command)...)
}

func (target RemoteSSH) CheckReachable() error {
	return target.Exec([]string{"true"})
}

func (target RemoteSSH) CheckHostKey() error {
	if target.remote.HostKeyFingerprint == "" {
		return nil
	}
	if err := validateRemote(target.remote); err != nil {
		return err
	}
	output, err := target.runner.Output("ssh-keyscan", RemoteHostKeyScanArgs(target.remote)...)
	if err != nil {
		return fmt.Errorf("scan remote host key: %w", err)
	}
	fingerprints, err := RemoteHostKeyFingerprints(output)
	if err != nil {
		return err
	}
	for _, fingerprint := range fingerprints {
		if fingerprint == target.remote.HostKeyFingerprint {
			return nil
		}
	}
	return fmt.Errorf("remote host key fingerprint mismatch: expected %s, got %s", target.remote.HostKeyFingerprint, strings.Join(fingerprints, ", "))
}

func RemoteSSHArgs(remote registry.RemoteServer, command []string) []string {
	port := remote.Port
	if port == 0 {
		port = registry.DefaultRemotePort
	}
	args := []string{
		"-p",
		strconv.Itoa(port),
		"-o",
		"BatchMode=yes",
		"-o",
		"ForwardAgent=yes",
		"-o",
		"ControlMaster=no",
		"-o",
		"StrictHostKeyChecking=accept-new",
		"-o",
		"ServerAliveInterval=30",
		"-o",
		"ConnectTimeout=5",
	}
	if remote.IdentityFile != "" {
		args = append(args, "-i", remote.IdentityFile)
	}
	args = append(args, remote.User+"@"+remote.Host, "--")
	return append(args, command...)
}

func RemoteHostKeyScanArgs(remote registry.RemoteServer) []string {
	port := remote.Port
	if port == 0 {
		port = registry.DefaultRemotePort
	}
	return []string{
		"-p",
		strconv.Itoa(port),
		"-T",
		"5",
		remote.Host,
	}
}

func RemoteHostKeyFingerprints(scanOutput []byte) ([]string, error) {
	fingerprintSet := map[string]struct{}{}
	scanner := bufio.NewScanner(bytes.NewReader(scanOutput))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, fmt.Errorf("invalid remote host key line: %s", line)
		}
		keyBlob, err := base64.StdEncoding.DecodeString(fields[2])
		if err != nil {
			return nil, fmt.Errorf("invalid remote host key: %w", err)
		}
		hash := sha256.Sum256(keyBlob)
		fingerprintSet["SHA256:"+base64.RawStdEncoding.EncodeToString(hash[:])] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	fingerprints := make([]string, 0, len(fingerprintSet))
	for fingerprint := range fingerprintSet {
		fingerprints = append(fingerprints, fingerprint)
	}
	sort.Strings(fingerprints)
	if len(fingerprints) == 0 {
		return nil, errors.New("no remote host keys found")
	}
	return fingerprints, nil
}

func validateRemote(remote registry.RemoteServer) error {
	if remote.Host == "" {
		return errors.New("remote.host is required")
	}
	if remote.User == "" {
		return errors.New("remote.user is required")
	}
	if remote.Port < 0 || remote.Port > 65535 {
		return fmt.Errorf("unsupported remote.port: %d", remote.Port)
	}
	return nil
}
