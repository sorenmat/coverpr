package main

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"

	"os"

	"io/ioutil"

	"github.com/google/go-github/github"
	"github.com/waigani/diffparser"
	"golang.org/x/oauth2"
	"golang.org/x/tools/cover"
)

func add(a, b int) int {
	return a + b
}

func sub(a, b int) int {
	return a - b
}

type Line struct {
	number int
	line   string
	coverd bool
}
type ChangeSet struct {
	filename string
	lines    []Line
}

var basedir = "test"

func includeFileInCoverage(filename string) bool {
	return strings.HasSuffix(filename, "_test.go") || strings.HasSuffix(filename, ".go")
}

func generateDiff(diffData string) []ChangeSet {
	diff, _ := diffparser.Parse(diffData)

	var changeSet []ChangeSet
	for _, f := range diff.Files {
		c := ChangeSet{filename: fmt.Sprintf("%v%c%v", getPackage(), os.PathSeparator, f.NewName)}
		// fmt.Println("Processing", f.NewName)
		if strings.HasSuffix(f.NewName, ".go") && !strings.HasSuffix(f.NewName, "_test.go") {
			for _, l := range f.Hunks[0].NewRange.Lines {
				if l.Mode == diffparser.ADDED {
					// fmt.Printf("DIFF: %v %v %v %v\n", l.Mode, l.Number, l.Position, l.Content)
					c.lines = append(c.lines, Line{number: l.Number, line: l.Content})
				}
			}
			changeSet = append(changeSet, c)
		}
	}
	// fmt.Println("ChangeSet", changeSet)
	return changeSet
}

func generateCoverageData() []byte {
	err := os.Chdir("test")
	if err != nil {
		fmt.Println("unable to change directory")
	}
	// go test -coverprofile=coverage.out .
	_, err = exec.Command("go", "test", "-coverprofile=/tmp/cover.out", ".").Output()
	if err != nil {
		log.Fatal(err)
	}

	coverageData, err := ioutil.ReadFile("/tmp/cover.out")
	return coverageData
}

func main() {

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: "f3b2eb2a97a547f62249e9dc94b1c2d73a0766a8"},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)
	owner := "sorenmat"
	repo := "go_pr_testing"
	prNumber := 1
	pr, _, err := client.PullRequests.Get(owner, repo, prNumber)
	if err != nil {
		fmt.Println(err)
	}

	diffURL := pr.DiffURL
	resp, err := http.Get(*diffURL)
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("unable to get diff")
	}

	diffText := string(b)
	fmt.Println("Diff: ", diffText)
	changeSet := generateDiff(diffText)
	coverageData := generateCoverageData()
	if coverageData == nil {
		log.Fatal("unable to open coverage report")
	}

	c := parseCoverfile("/tmp/cover.out", changeSet)
	result := "Please note the following code is not covered by tests.\n"
	for _, v := range c {
		result += "in " + v.filename + "\n\n```go\n"
		fmt.Printf("%v contains uncovered code\n", v.filename)
		for _, resLine := range v.lines {

			if !resLine.coverd {
				// body := fmt.Sprintf("%v %v\n", resLine.number, resLine.line)
				// position := 1
				// inreplyto := 1
				// comment := &github.PullRequestComment{
				// 	CommitID: pr.Head.SHA,
				// 	// Path:
				// 	Body: &body,
				// 	// Position:  &position,
				// 	InReplyTo: &inreplyto,
				// }
				// resline coverd
				fmt.Printf("%v %v\n", resLine.number, resLine.line)
				result += fmt.Sprintf("%v %v\n", resLine.number, resLine.line)
			}
		}
		result += "```"
	}
	comment := &github.IssueComment{
		Body: &result,
	}
	_, _, err = client.Issues.CreateComment(owner, repo, prNumber, comment)
	// _, _, err := client.PullRequests.CreateComment(owner, repo, prNumber, comment)
	if err != nil {
		log.Fatal("couldn't create comment", err)
	}
}

func getPackage() string {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal("unable to locate working directory")
	}
	path := os.Getenv("GOPATH")
	return strings.Replace(wd, path+"/src/", "", -1) + "/" + basedir
}

func parseCoverfile(file string, changeSet []ChangeSet) []ChangeSet {
	// name.go:line.column,line.column numberOfStatements count
	// see https://github.com/golang/go/blob/0104a31b8fbcbe52728a08867b26415d282c35d2/src/cmd/cover/profile.go#L56
	fmt.Println("Reading cover file", file)
	p, err := cover.ParseProfiles(file)
	if err != nil {
		log.Fatal("unable to parse coverage profile", err)
	}
	// fmt.Println("--------------------")
	// fmt.Println("Using package", getPackage())
	result := changeSet
	for _, f := range p {
		for rk, v := range changeSet {
			for clk, changeline := range v.lines {
				// fmt.Printf("%v == %v ? %v\n", v.filename, f.FileName, (v.filename == f.FileName))
				if v.filename == f.FileName {
					for _, b := range f.Blocks {
						// fmt.Printf("%v:%v included in %v %v\n", b.StartLine, b.EndLine, changeline.number, (changeline.number >= b.StartLine && changeline.number <= b.EndLine))
						if changeline.number >= b.StartLine && changeline.number <= b.EndLine && b.Count == 1 {
							result[rk].lines[clk].coverd = true
						}
						if changeline.line == "}" || changeline.line == "" {
							result[rk].lines[clk].coverd = true
						}
					}
				}
			}
		}
	}
	return changeSet
}
