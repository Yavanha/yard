package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"
	"text/tabwriter"

	yarddocs "yard/docs"
)

type guideEntry struct {
	Slug   string
	Title  string
	Status string
}

func runGuide(parsed args, output io.Writer) error {
	switch parsed.subcommand {
	case "list":
		if len(parsed.positionals) != 0 {
			return errors.New("usage: guide list|<slug>")
		}
		return runGuideList(output)
	case "":
		return errors.New("usage: guide list|<slug>")
	default:
		if len(parsed.positionals) != 0 {
			return errors.New("usage: guide list|<slug>")
		}
		return runGuideShow(parsed.subcommand, output)
	}
}

func runGuideList(output io.Writer) error {
	entries, err := guideEntries()
	if err != nil {
		return err
	}

	writer := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "SLUG\tSTATUS\tTITLE")
	for _, entry := range entries {
		fmt.Fprintf(writer, "%s\t%s\t%s\n", entry.Slug, entry.Status, entry.Title)
	}
	return writer.Flush()
}

func runGuideShow(slug string, output io.Writer) error {
	if !validGuideSlug(slug) {
		return fmt.Errorf("unknown guide: %s. Run \"yard guide list\".", slug)
	}
	content, err := fs.ReadFile(yarddocs.CLI, "cli/"+slug+".md")
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("unknown guide: %s. Run \"yard guide list\".", slug)
	}
	if err != nil {
		return err
	}
	_, err = output.Write(content)
	return err
}

func guideEntries() ([]guideEntry, error) {
	files, err := fs.ReadDir(yarddocs.CLI, "cli")
	if err != nil {
		return nil, err
	}

	entries := []guideEntry{}
	for _, file := range files {
		if file.IsDir() || file.Name() == "README.md" || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}
		content, err := fs.ReadFile(yarddocs.CLI, "cli/"+file.Name())
		if err != nil {
			return nil, err
		}
		entries = append(entries, guideEntry{
			Slug:   strings.TrimSuffix(file.Name(), ".md"),
			Title:  markdownTitle(string(content)),
			Status: markdownStatus(string(content)),
		})
	}
	sort.Slice(entries, func(left int, right int) bool {
		return entries[left].Slug < entries[right].Slug
	})
	return entries, nil
}

func markdownTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if title, ok := strings.CutPrefix(line, "# "); ok {
			return strings.TrimSpace(title)
		}
	}
	return "-"
}

func markdownStatus(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if status, ok := strings.CutPrefix(line, "Status:"); ok {
			return strings.TrimSpace(status)
		}
	}
	return "-"
}

func validGuideSlug(slug string) bool {
	if slug == "" || strings.Contains(slug, "..") {
		return false
	}
	for _, char := range slug {
		if !(char == '-' || char >= '0' && char <= '9' || char >= 'a' && char <= 'z') {
			return false
		}
	}
	return true
}
