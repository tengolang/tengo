// tengo-man: terminal reference for the Tengo language.
//
// Install: go install github.com/tengolang/tengo/v3/cmd/tengo-man@latest
// Usage:   tengo-man [topic]   or   tengo man [topic]
package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed pages
var pages embed.FS

func main() {
	if len(os.Args) < 2 {
		listTopics(os.Stdout)
		return
	}
	topic := strings.ToLower(strings.Join(os.Args[1:], " "))
	if topic == "help" || topic == "-h" || topic == "--help" {
		fmt.Fprintln(os.Stdout, "Usage: tengo-man [topic]")
		fmt.Fprintln(os.Stdout, "       tengo-man          list all topics")
		return
	}
	showTopic(topic)
}

// showTopic finds and pages the content for the given topic name.
func showTopic(topic string) {
	content, found := findTopic(topic)
	if !found {
		fmt.Fprintf(os.Stderr, "tengo-man: no manual entry for %q\n", topic)
		fmt.Fprintf(os.Stderr, "Run 'tengo-man' with no arguments to list all topics.\n")
		os.Exit(1)
	}
	page(content)
}

// findTopic searches pages/ for a file whose base name matches topic.
func findTopic(topic string) ([]byte, bool) {
	topic = strings.ReplaceAll(topic, " ", "-")
	var match string
	_ = fs.WalkDir(pages, "pages", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if strings.EqualFold(base, topic) {
			match = path
		}
		return nil
	})
	if match == "" {
		return nil, false
	}
	data, err := pages.ReadFile(match)
	if err != nil {
		return nil, false
	}
	return data, true
}

// listTopics prints all available topics grouped by category.
func listTopics(w io.Writer) {
	type entry struct{ cat, name string }
	var entries []entry

	_ = fs.WalkDir(pages, "pages", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		parts := strings.Split(path, "/")
		if len(parts) < 3 {
			return nil
		}
		cat := parts[1]
		name := strings.TrimSuffix(parts[2], filepath.Ext(parts[2]))
		entries = append(entries, entry{cat, name})
		return nil
	})

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].cat != entries[j].cat {
			return entries[i].cat < entries[j].cat
		}
		return entries[i].name < entries[j].name
	})

	fmt.Fprintln(w, "TENGO MANUAL")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "    Usage: tengo-man <topic>")
	fmt.Fprintln(w, "           tengo man <topic>")
	fmt.Fprintln(w)

	cur := ""
	for _, e := range entries {
		if e.cat != cur {
			cur = e.cat
			fmt.Fprintf(w, "    %s\n", strings.ToUpper(cur))
		}
		fmt.Fprintf(w, "        %s\n", e.name)
	}
	fmt.Fprintln(w)
}

// page writes content through $PAGER / less / more, falling back to stdout.
func page(content []byte) {
	pager := os.Getenv("PAGER")
	if pager == "" {
		if p, err := exec.LookPath("less"); err == nil {
			pager = p
		} else if p, err := exec.LookPath("more"); err == nil {
			pager = p
		}
	}

	if pager == "" {
		os.Stdout.Write(content)
		return
	}

	cmd := exec.Command(pager)
	if filepath.Base(pager) == "less" {
		cmd = exec.Command(pager, "-R", "-F", "-X")
	}
	cmd.Stdin = strings.NewReader(string(content))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}
