package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"strconv"
	"text/template"

	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
	"golang.org/x/tools/cover"
)

var reportTmp = template.Must(template.New("report").Parse(`
# go-cover-view

{{range .}}
<details> <summary> {{.FileName}} </summary>
{{.Report}}
</details>
{{end}}
`))

type markdownRenderer struct{}

var _ renderer = (*markdownRenderer)(nil)

func (r *markdownRenderer) Render(w io.Writer, profiles []*cover.Profile, path string) error {
	results, err := getMarkdownReports(profiles, path)
	if err != nil {
		return err
	}
	return reportTmp.ExecuteTemplate(w, "report", results)
}

type markdownReport struct {
	FileName string
	Report   string
}

func newMarkdownReport(fileName string, lines []string) *markdownReport {
	return &markdownReport{
		FileName: fileName,
		Report:   buildReport(lines),
	}
}

func getMarkdownReports(profiles []*cover.Profile, path string) ([]*markdownReport, error) {
	diffs, err := getDiffs()
	if err != nil {
		return nil, err
	}
	reports := make([]*markdownReport, 0, len(profiles))
	for _, profile := range profiles {
		lines, err := getLines(profile, path)
		if err != nil {
			return nil, err
		}
		if containsDiff(profile.FileName, path, diffs) {
			reports = append(reports, newMarkdownReport(profile.FileName, lines))
		}
	}
	return reports, nil
}

func buildReport(lines []string) string {
	var b strings.Builder
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```")
	for _, line := range lines {
		fmt.Fprintln(&b, line)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```")
	return b.String()
}

func upsertGitHubPullRequestComment(profiles []*cover.Profile, path string) error {
	prNumber, err := strconv.Atoi(os.Getenv("PR_NUMBER"))
	if err != nil {
		return err
	}
	if prNumber <= 0 {
		return errors.New("env: missing PR_NUMBER")
	}

	ctx := context.Background()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return errors.New("env: GITHUB_TOKEN is missing")
	}

	var buf bytes.Buffer
	r := &markdownRenderer{}
	if err := r.Render(&buf, profiles, path); err != nil {
		return err
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(ctx, ts)
	gc := github.NewClient(httpClient)

	_repo := strings.Split(os.Getenv("GITHUB_REPOSITORY"), "/")
	if len(_repo) != 2 {
		return fmt.Errorf("invalid env: GITHUB_REPOSITORY=%v", _repo)
	}

	owner := _repo[0]
	repo := _repo[1]
	comments, _, err := gc.Issues.ListComments(ctx, owner, repo, prNumber, nil)
	if err != nil {
		return err
	}

	var commentID int64
	for _, c := range comments {
		u := c.GetUser()
		fmt.Printf("Got comment user login: [%s], type: [%s]\n", u.GetLogin(), u.GetType())
		login := u.GetLogin()
		if (login == "rStheBot" || login == "github-actions[bot]") && strings.Contains(c.GetBody(), "# go-cover-view") {
			commentID = c.GetID()
			break
		}
	}
	fmt.Printf("Got PR Number: %d, Comment ID: %d\n", prNumber, commentID)

	// The Issues API is what manages comments for PRs
	// The PR API only handles commenting on the code diff
	body := buf.String()
	if commentID == 0 {
		_, _, err := gc.Issues.CreateComment(ctx, owner, repo, prNumber, &github.IssueComment{
			Body: &body,
		})
		if err != nil {
			fmt.Println("Returning err from CreateComment")
			return err
		}
	} else {
		_, _, err := gc.Issues.EditComment(ctx, owner, repo, commentID, &github.IssueComment{
			Body: &body,
		})
		if err != nil {
			fmt.Println("Returning err from EditComment")
			return err
		}
	}
	return nil
}
