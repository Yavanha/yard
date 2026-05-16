package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"devctl/internal/config"
)

const version = "0.2.0-dev"

type args struct {
	command     string
	projectPath string
	help        bool
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(argv []string) error {
	parsed, err := parseArgs(argv)
	if err != nil {
		return err
	}

	if parsed.help || parsed.command == "" {
		printHelp()
		return nil
	}

	switch parsed.command {
	case "config":
		return runConfig(parsed)
	default:
		return fmt.Errorf("unknown command: %s", parsed.command)
	}
}

func parseArgs(argv []string) (args, error) {
	parsed := args{}

	for index := 0; index < len(argv); index++ {
		value := argv[index]

		switch value {
		case "--help", "-h":
			parsed.help = true
		case "--project":
			if index+1 >= len(argv) {
				return args{}, errors.New("--project requires a path")
			}
			parsed.projectPath = argv[index+1]
			index++
		default:
			if len(value) > 0 && value[0] == '-' {
				return args{}, fmt.Errorf("unknown flag: %s", value)
			}
			if parsed.command != "" {
				return args{}, fmt.Errorf("unexpected argument: %s", value)
			}
			parsed.command = value
		}
	}

	return parsed, nil
}

func runConfig(parsed args) error {
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}

	loaded, err := config.Load(parsed.projectPath, workDir)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(loaded)
}

func printHelp() {
	fmt.Printf(`devctl-go %s

Usage:
  go run ./cmd/devctl --help
  go run ./cmd/devctl config [--project <path>]

Commands:
  config  Print resolved project config as JSON.
`, version)
}
