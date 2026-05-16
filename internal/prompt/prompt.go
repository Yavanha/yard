package prompt

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type Prompter struct {
	reader *bufio.Reader
	out    io.Writer
}

func New(in io.Reader, out io.Writer) Prompter {
	return Prompter{
		reader: bufio.NewReader(in),
		out:    out,
	}
}

func (p Prompter) Writer() io.Writer {
	return p.out
}

func (p Prompter) Ask(label string, defaultValue string, required bool) (string, error) {
	for {
		if defaultValue == "" {
			fmt.Fprintf(p.out, "%s: ", label)
		} else {
			fmt.Fprintf(p.out, "%s [%s]: ", label, defaultValue)
		}

		answer, err := p.readLine()
		if err != nil {
			return "", err
		}
		if answer == "" {
			answer = defaultValue
		}
		if answer != "" || !required {
			return answer, nil
		}

		fmt.Fprintln(p.out, "Value required.")
	}
}

func (p Prompter) Confirm(label string, defaultValue bool) (bool, error) {
	defaultLabel := "y/N"
	if defaultValue {
		defaultLabel = "Y/n"
	}

	for {
		fmt.Fprintf(p.out, "%s [%s]: ", label, defaultLabel)
		answer, err := p.readLine()
		if err != nil {
			return false, err
		}
		if answer == "" {
			return defaultValue, nil
		}

		switch strings.ToLower(answer) {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintln(p.out, "Answer yes or no.")
		}
	}
}

func (p Prompter) readLine() (string, error) {
	line, err := p.reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	if err == io.EOF && line == "" {
		return "", io.ErrUnexpectedEOF
	}
	return strings.TrimSpace(line), nil
}
