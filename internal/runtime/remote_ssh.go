package runtime

import (
	"errors"
	"fmt"
	"strconv"

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
	return target.runner.Run("ssh", RemoteSSHArgs(target.remote, command)...)
}

func (target RemoteSSH) ExecOutput(command []string) ([]byte, error) {
	if len(command) == 0 {
		return nil, errors.New("exec requires a command")
	}
	if err := validateRemote(target.remote); err != nil {
		return nil, err
	}
	return target.runner.Output("ssh", RemoteSSHArgs(target.remote, command)...)
}

func (target RemoteSSH) CheckReachable() error {
	return target.Exec([]string{"true"})
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
