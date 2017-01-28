package main

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"

	"os"

	"io/ioutil"

	"github.com/alecthomas/kingpin"
	"github.com/google/go-github/github"
	"github.com/waigani/diffparser"
	"golang.org/x/oauth2"
	"golang.org/x/tools/cover"
)

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
	return changeSet
}

func githubClient(githubToken string) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	return github.NewClient(tc)

}
func getDiff(githubToken string, owner string, repo string, prNumber int) string {
	if githubToken != "" {
		client := githubClient(githubToken)
		// owner := "sorenmat"
		// repo := "go_pr_testing"
		// prNumber := 1
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
		return string(b)
	}
	cmd := exec.Command("git", "diff")
	// err := cmd.Run()
	// if err != nil {
	// fmt.Println("unable to generate diff from git", err)
	// os.Exit(1)
	// }
	data, err := cmd.Output()
	if err != nil {
		fmt.Println("unable to get output from git diff", err)
		os.Exit(1)
	}
	return string(data)
}
func main() {
	// verbose := kingpin.Flag("verbose", "Verbose mode.").Short('v').Bool()
	githubToken := kingpin.Flag("github_token", "Github OAuth2 access token").OverrideDefaultFromEnvar("GITHUB_TOKEN").String()
	repoName := kingpin.Flag("repo", "Github repository name").Default("").String()
	repoOwner := kingpin.Flag("repoOwner", "Github repository owner").Default("").String()
	prNumber := kingpin.Flag("pr", "Github pull-request number").Int()

	coverFile := kingpin.Flag("coverFile", "Coverfile generated with go test -cover").OverrideDefaultFromEnvar("COVER_FILE").Required().String()
	kingpin.Parse()

	if *githubToken != "" {
		if *repoOwner == "" || *repoName == "" || *prNumber == 0 {
			fmt.Println("Please specify all github information repo, repoOwner and pr")
			os.Exit(1)
		}
	}
	diffText := getDiff(*githubToken, *repoOwner, *repoName, *prNumber)
	// fmt.Println("Diff: ", diffText)
	changeSet := generateDiff(diffText)

	c := parseCoverfile(*coverFile, changeSet)
	markdown := *githubToken != ""
	// output coverage data to either github or console
	result := generateResult(c, markdown)

	if *githubToken != "" {
		comment := &github.IssueComment{
			Body: &result,
		}
		comment, _, err := githubClient(*githubToken).Issues.CreateComment(*repoOwner, *repoName, *prNumber, comment)
		if err != nil {
			log.Fatal("couldn't create comment", err)
		}
		fmt.Println("Created a comment at ", *comment.HTMLURL)
	} else {
		fmt.Println(result)
	}
	if result != "" {
		os.Exit(1)
	}
	os.Exit(0)
}

func generateResult(c []ChangeSet, markdown bool) string {
	result := ""
	for _, v := range c {
		result += v.filename + "\n\n"
		if markdown {
			result += "```go\n"
		}
		// result += fmt.Sprintf("%v contains uncovered code\n", v.filename)
		for _, resLine := range v.lines {

			if !resLine.coverd {
				// resline coverd
				// fmt.Printf("%v %v\n", resLine.number, resLine.line)
				result += fmt.Sprintf("%v %v\n", resLine.number, resLine.line)
			}
		}
		if markdown {
			result += "```"
		}
	}
	if result != "" {
		// If we have a result prepend a header
		result = "Please note the following code is not covered by tests.\n" + result
	}
	return result
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
