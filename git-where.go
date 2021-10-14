package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/jessevdk/go-flags"
	"github.com/tsuyoshiwada/go-gitlog"
)

type Options struct {
	Url        string `short:"u" long:"url" description:"the url to be updated"`
	FileName   string `short:"f" long:"file" description:"the relative path from the repo base of the file to track down"`
	LineNumber int64  `short:"l" long:"line" description:"the line number of the line in that file"`
	Date       string `short:"d" long:"date" description:"the date the file/line were referenced in the format of MM/DD/YYYY"`
	Path       string `short:"p" long:"path" description:"path to the repo"`
	Quiet      bool   `short:"q" long:"quiet" description:"silences all output except for the new file/line number or error message"`
}

type Result struct {
	FileName   string `json:"file"`
	LineNumber int64  `json:"line"`
}

var arguments Options = Options{
	Path:  ".",
	Quiet: false,
}

var errorLog = log.New(os.Stderr, "", 0)

func main() {
	parser := flags.NewParser(&arguments, flags.Default)
	_, err := parser.Parse()
	if err != nil {
		errorLog.Println("Error: ", err)
		parser.WriteHelp(os.Stdin)
		os.Exit(1)
	}

	file := arguments.FileName
	lineNumber := arguments.LineNumber
	date, err := time.Parse("01/02/2006", strings.TrimSpace(arguments.Date))
	if err != nil {
		errorLog.Println("Error: Issue parsing the date. Please check that it is well formatted and valid")
		parser.WriteHelp(os.Stdin)
		os.Exit(1)
	}

	// TODO: make sure we are on the right branch, and up to date
	git := gitlog.New(&gitlog.Config{
		Path: arguments.Path,
	})

	commits, err := git.Log(&gitlog.RevTime{Since: date}, &gitlog.Params{Reverse: true})
	if err != nil {
		errorLog.Println("Error: ", err)
		os.Exit(1)
	}

	if len(commits) == 0 {
		errorLog.Println("No commits since the date provided")
		os.Exit(1)
	}

	// TODO: get commit before first commit to create permalink
	baseCommit, err := getCommitBefore(arguments.Path, commits[0].Hash.Long, date)
	if err != nil {
		errorLog.Println("Error: ", err)
		os.Exit(1)
	}

	commitIds := make([]string, len(commits)+1)
	commitIds[0] = baseCommit
	for index, commit := range commits {
		commitIds[index+1] = commit.Hash.Long
	}

	if !arguments.Quiet {
		fmt.Printf("Working off of base commit %s\n\n", baseCommit)
		fileString, err := getFileAtCommit(arguments.Path, "HEAD", file)
		if err != nil {
			errorLog.Println("Error: ", err)
			os.Exit(1)
		}
		err = showFileAtLineNumber(file, fileString, lineNumber)
		if err != nil {
			errorLog.Println("Error: ", err)
			os.Exit(1)
		}
	}

	filesChannels := getDiffsForCommits(arguments.Path, commitIds)
	for _, filesChannel := range filesChannels {
		select {
		case result := <-filesChannel:
			if result.err != nil {
				errorLog.Println("Error: ", err)
				os.Exit(1)
			}
			files, _, _ := gitdiff.Parse(strings.NewReader(result.diff))
			for _, commitFile := range files {

				// TODO: we really should fork here instead of just choosing the original
				if commitFile.OldName == file && !commitFile.IsCopy {

					originalLineNumber := lineNumber
					for _, fragment := range commitFile.TextFragments {
						amountToMove, err := processFragment(fragment, originalLineNumber)
						lineNumber += amountToMove

						// TODO: Follow through edits to the line in question
						if err != nil {
							errorLog.Println(err)
							errorLog.Printf("trail went cold at commit %s", result.commitId)
							errorLog.Printf("%s, %d", file, originalLineNumber)
							os.Exit(1)
						}
					}
					//fmt.Printf("%s changed in commit %s, new line number %d\n", file, commit.Hash.Long, lineNumber)
					if commitFile.OldName != commitFile.NewName {
						file = commitFile.NewName
					}
				}
			}
		case <-time.After(30 * time.Second):
			errorLog.Println("Timeout when processing diffs")
			os.Exit(1)
		}

	}

	if arguments.Quiet {
		result := Result{
			FileName:   file,
			LineNumber: lineNumber,
		}

		output, err := json.Marshal(result)
		if err != nil {
			errorLog.Println("Error: ", err)
			os.Exit(1)
		}

		fmt.Println(string(output))
	} else {
		fmt.Printf("\n\nCurrent reference is %s line %d\n\n", file, lineNumber)
		fileString, err := getFileAtCommit(arguments.Path, "HEAD", file)
		if err != nil {
			errorLog.Println("Error: ", err)
			os.Exit(1)
		}

		err = showFileAtLineNumber(file, fileString, lineNumber)
		if err != nil {
			errorLog.Println("Error: ", err)
			os.Exit(1)
		}
	}
}

// return the amount that this fragment moves the original line
func processFragment(fragment *gitdiff.TextFragment, originalLineNumber int64) (int64, error) {

	if fragment.OldPosition <= int64(originalLineNumber) {
		if fragment.OldPosition+fragment.LinesDeleted >= int64(originalLineNumber) {
			// TODO: track down if the line is recreated elsewhere
			errorLog.Printf("fragment %s", fragment.Header())
			return 0, fmt.Errorf("line has been deleted")
		}

		return fragment.NewLines - fragment.OldLines, nil
	}
	return 0, nil
}

func min(x, y int64) int64 {
	if x < y {
		return x
	}
	return y
}

func max(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}
