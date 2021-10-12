package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/jessevdk/go-flags"
	"github.com/tsuyoshiwada/go-gitlog"
)

type Options struct {
	FileName   string `short:"f" long:"file" description:"the relative path from the repo base of the file to track down"`
	LineNumber int64  `short:"l" long:"line" description:"the line number of the line in that file"`
	Date       string `short:"d" long:"date" description:"the date the file/line were referenced in the format of MM/DD/YYYY"`
	Path       string `short:"p" long:"path" description:"path to the repo"`
}

var arguments Options = Options{
	Path: ".",
}

func main() {
	parser := flags.NewParser(&arguments, flags.Default)
	_, err := parser.Parse()
	if err != nil {
		fmt.Println("Error: ", err)
		parser.WriteHelp(os.Stdin)
		os.Exit(1)
	}

	git := gitlog.New(&gitlog.Config{
		Path: arguments.Path,
	})

	file := arguments.FileName
	lineNumber := arguments.LineNumber
	date, err := time.Parse("01/02/2006", strings.TrimSpace(arguments.Date))
	if err != nil {
		fmt.Println("Error: Issue parsing the date. Please check that it is well formatted and valid")
		parser.WriteHelp(os.Stdin)
		os.Exit(1)
	}

	commits, err := git.Log(&gitlog.RevTime{Since: date}, &gitlog.Params{Reverse: true})

	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}

	// TODO: get commit before first commit to create permalink
	baseCommit, err := getCommitBefore(arguments.Path, commits[0].Hash.Long, date)
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}

	fmt.Printf("Working off of base commit %s\n\n", baseCommit)

	showFileAtCommit(arguments.Path, baseCommit, file, lineNumber)

	for _, commit := range commits {
		cmd := exec.Command("git", "diff", fmt.Sprintf("%s~", commit.Hash.Long), commit.Hash.Long)
		cmd.Dir = arguments.Path
		var cmdOut, cmdErr bytes.Buffer
		cmd.Stdout = &cmdOut
		cmd.Stderr = &cmdErr

		err := cmd.Run()
		if err != nil {
			fmt.Println("Error: ", err)
			fmt.Println(cmdErr.String())
			os.Exit(1)
		}

		files, _, err := gitdiff.Parse(&cmdOut)
		for _, commitFile := range files {

			// TODO: we really should fork here instead of just choosing the original
			if commitFile.OldName == file && !commitFile.IsCopy {

				originalLineNumber := lineNumber
				for _, fragment := range commitFile.TextFragments {
					amountToMove, err := processFragment(fragment, originalLineNumber)
					lineNumber += amountToMove

					// TODO: Follow through edits to the line in question
					if err != nil {
						fmt.Printf("trail went cold at commit %s", commit.Hash.Long)
						fmt.Printf("%s, %d", file, originalLineNumber)
					}
				}
				//fmt.Printf("%s changed in commit %s, new line number %d\n", file, commit.Hash.Long, lineNumber)
				if commitFile.OldName != commitFile.NewName {
					file = commitFile.NewName
				}
			}
		}
	}
	fmt.Printf("\n\nCurrent reference is %s line %d\n\n", file, lineNumber)
	showFileAtCommit(arguments.Path, "HEAD", file, lineNumber)
}

// return the amount that this fragment moves the original line
func processFragment(fragment *gitdiff.TextFragment, originalLineNumber int64) (int64, error) {

	if fragment.OldPosition <= int64(originalLineNumber) {
		if fragment.OldPosition+fragment.LinesDeleted >= int64(originalLineNumber) {
			return 0, fmt.Errorf("line has been deleted")
		}
		//fmt.Print(fragment.Lines)
		return fragment.NewLines - fragment.OldLines, nil
	}
	return 0, nil
}

func showFileAtCommit(path string, commitHash string, fileName string, lineNumber int64) {
	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", commitHash, fileName))
	cmd.Dir = path
	var cmdOut, cmdErr bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdErr

	err := cmd.Run()
	if err != nil {
		fmt.Println("Error: ", err)
		fmt.Println(cmdErr.String())
		os.Exit(1)
	}

	lexer := lexers.Match(fileName)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	if strings.Count(cmdOut.String(), "\n") < int(lineNumber) {
		fmt.Printf("Line number exceeds file line count at commit %s", commitHash)
		os.Exit(1)
	}

	iteratator, err := lexer.Tokenise(nil, cmdOut.String())

	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}

	style := styles.Get("dracula")
	if style == nil {
		style = styles.Fallback
	}

	var buffer bytes.Buffer
	formatters.TTY.Format(bufio.NewWriter(&buffer), style, iteratator)

	lines := strings.Split(buffer.String(), "\n")

	for i, line := range lines[max(0, lineNumber-6):min(lineNumber+5, int64(len(lines)-1))] {
		currentLine := int64(i) + max(0, lineNumber-6) + 1
		lineNumberString := fmt.Sprintf("%5d", currentLine)
		fmt.Print(lineNumberString)
		fmt.Println(line)
	}
}

func getCommitBefore(path string, commitId string, date time.Time) (string, error) {
	cmd := exec.Command("git", "rev-list", "-1", fmt.Sprintf("--before=\"%s\"", date.Format("Jan 02 2006")), commitId)
	cmd.Dir = path
	var cmdOut, cmdErr bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdErr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("problem getting base commit: %s", cmdErr.String())
	}

	return strings.TrimSpace(cmdOut.String()), nil
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
