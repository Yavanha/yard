package gitrepo

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Runner interface {
	Output(env []string, command string, args ...string) ([]byte, error)
	Run(env []string, command string, args ...string) error
}

type ExecRunner struct{}

type Client struct {
	Runner Runner
}

func NewClient(runner Runner) Client {
	if runner == nil {
		runner = ExecRunner{}
	}
	return Client{Runner: runner}
}

func (r ExecRunner) Output(env []string, command string, args ...string) ([]byte, error) {
	cmd := exec.Command(command, args...)
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}

func (r ExecRunner) Run(env []string, command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c Client) TestAccess(repoURL string, identityFile string) error {
	if repoURL == "" {
		return fmt.Errorf("repo URL is required")
	}
	if identityFile == "" {
		return fmt.Errorf("SSH identity file is required")
	}

	output, err := c.Runner.Output(gitEnv(identityFile), "git", "ls-remote", repoURL)
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return fmt.Errorf("test repository access: %w: %s", err, message)
		}
		return fmt.Errorf("test repository access: %w", err)
	}
	return nil
}

func (c Client) Clone(repoURL string, identityFile string, destination string) error {
	if repoURL == "" {
		return fmt.Errorf("repo URL is required")
	}
	if identityFile == "" {
		return fmt.Errorf("SSH identity file is required")
	}
	if destination == "" {
		return fmt.Errorf("destination path is required")
	}

	if err := c.Runner.Run(gitEnv(identityFile), "git", "clone", repoURL, destination); err != nil {
		return fmt.Errorf("clone repository: %w", err)
	}
	return nil
}

func gitEnv(identityFile string) []string {
	return []string{
		"GIT_SSH_COMMAND=ssh -i " + shellQuote(identityFile) + " -o IdentitiesOnly=yes -o BatchMode=yes -o StrictHostKeyChecking=accept-new",
	}
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
