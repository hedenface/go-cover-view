package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/cover"
)

var (
	modFile     string
	report      string
	covered     string
	uncovered   string
	gitDiffBase string
	writer io.Writer = os.Stdout
)

type renderer interface {
	Render(w io.Writer, profiles []*cover.Profile, path string) error
}

type _modfile interface {
	Path() string
}

type modfileFromJSON struct {
	Module struct {
		Path string
	}
}

func (m *modfileFromJSON) Path() string {
	return m.Module.Path
}

type xmodfile struct {
	*modfile.File
}

func (m *xmodfile) Path() string {
	return m.Module.Mod.Path
}

func init() {
	flag.StringVar(&modFile, "modfile", "", "go.mod path")
	flag.StringVar(&report, "report", "coverage.txt", "coverage report path")
	flag.StringVar(&covered, "covered", "O", "prefix for covered line")
	flag.StringVar(&uncovered, "uncovered", "X", "prefix for uncovered line")
	flag.StringVar(&gitDiffBase, "git-diff-base", "origin/master", "git diff base")
}

func main() {
	prNumber := os.Getenv("PR_NUMBER")
	if prNumber == "" {
		fmt.Println("No PR_NUMBER environment variable, skipping")
		return
	}

	flag.Parse()
	if err := _main(); err != nil {
		log.Fatal(err)
	}
}

func _main() error {
	m, err := parseModfile()
	if err != nil {
		return err
	}

	profiles, err := cover.ParseProfiles(report)
	if err != nil {
		return err
	}

	submitCoverageData(report)
	return upsertGitHubPullRequestComment(profiles, m.Path())
}

func parseModfile() (_modfile, error) {
	if modFile == "" {
		output, err := exec.Command("go", "mod", "edit", "-json").Output()
		if err != nil {
			return nil, fmt.Errorf("go mod edit -json: %w", err)
		}
		var m modfileFromJSON
		if err := json.Unmarshal(output, &m); err != nil {
			return nil, err
		}
		return &m, nil
	}

	data, err := ioutil.ReadFile(modFile)
	if err != nil {
		return nil, err
	}

	f, err := modfile.Parse(modFile, data, nil)
	if err != nil {
		return nil, err
	}
	return &xmodfile{File: f}, nil
}

func getLines(profile *cover.Profile, module string) ([]string, error) {
	// github.com/johejo/go-cover-view/main.go -> ./main.go
	p := strings.ReplaceAll(profile.FileName, module, ".")
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lines := make([]string, 0, 1000)
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
	}

	width := int(math.Log10(float64(len(lines)))) + 1
	if len(covered) > len(uncovered) {
		width += len(covered) + 1
	} else {
		width += len(uncovered) + 1
	}
	w := strconv.Itoa(width)
	for i, line := range lines {
		var newLine string
		if len(line) == 0 {
			format := "%" + w + "d:"
			newLine = fmt.Sprintf(format, i+1)
		} else {
			format := "%" + w + "d: %s"
			newLine = fmt.Sprintf(format, i+1, line)
		}
		lines[i] = newLine
	}

	for _, block := range profile.Blocks {
		var prefix string
		if block.Count > 0 {
			prefix = covered
		} else {
			prefix = uncovered
		}
		for i := block.StartLine - 1; i <= block.EndLine-1; i++ {
			if i >= len(lines) {
				return nil, fmt.Errorf("invalid line length: index=%d, len(lines)=%d", i, len(lines))
			}
			line := lines[i]
			newLine := prefix + line[len(prefix):]
			lines[i] = newLine
		}
	}

	return lines, nil
}

func getDiffs() ([]string, error) {
	args := []string{"diff", "--name-only"}

	if gitDiffBase != "" {
		args = append(args, gitDiffBase)
	}

	_out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, err
	}

	out := strings.TrimSpace(string(_out))
	diffs := strings.Split(out, "\n")
	return diffs, nil
}

func containsDiff(filename, path string, diffs []string) bool {
	for _, diff := range diffs {
		name := fmt.Sprintf("%s/%s", path, diff)
		if filename == name {
			return true
		}
	}
	return false
}

func submitCoverageData(report string) {
	out, err := exec.Command("go", "tool", "cover", "-func", report).Output()
	if err != nil {
		return
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		// we're looking for a line that says:
		// total: (statements) 0.0%
		if strings.HasPrefix(line, "total:") {
			keyValPair := strings.Split(line, ":")
			//key := strings.TrimSpace(keyValPair[0])
			_val := strings.ReplaceAll(keyValPair[1], "(statements)", "")
			_val = strings.ReplaceAll(_val, "%", "")
			val, err := strconv.ParseFloat(strings.TrimSpace(_val), 64)
			if err == nil {
				submitDataDog(val)
			} else {
				fmt.Printf("Error: %v\n", err)
			}
		}
	}
}
