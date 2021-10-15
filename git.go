package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

type GitDiffResult struct {
	commitId     string
	lastCommitId string
	diff         []*gitdiff.File
	err          error
}

func getFileAtCommit(repoPath string, commitId string, fileName string) (string, error) {
	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", commitId, fileName))
	cmd.Dir = repoPath
	var cmdOut, cmdErr bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdErr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error getting the file at commit %s: %s", commitId, cmdErr.String())
	}
	return cmdOut.String(), nil
}

func getCommitBefore(path string, commitId string, date time.Time) (string, error) {
	cmd := exec.Command("git", "rev-parse", fmt.Sprintf("%s^", commitId))
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

func getDiffsForCommits(repoPath string, commitIds []string) []chan *GitDiffResult {
	lastCommitId := commitIds[0]
	results := make([]chan *GitDiffResult, len(commitIds)-1)
	for index, commitId := range commitIds[1:] {
		resultChannel := make(chan *GitDiffResult, 1)
		results[index] = resultChannel
		go getDiffForCommitAsync(repoPath, lastCommitId, commitId, resultChannel, index)
		lastCommitId = commitId
	}
	return results
}

func getDiffForCommitAsync(repoPath string, previousCommitId string, commitId string, resultChannel chan *GitDiffResult, i int) {
	diff, err := getCommitContents(repoPath, previousCommitId, commitId)
	result := new(GitDiffResult)
	result.commitId = commitId
	result.lastCommitId = previousCommitId
	result.diff = diff
	result.err = err
	resultChannel <- result
	close(resultChannel)
}

func getCommitContents(repoPath string, previousCommitId string, commitId string) ([]*gitdiff.File, error) {
	cmd := exec.Command("git", "diff", previousCommitId, commitId)
	cmd.Dir = arguments.Path
	var cmdOut, cmdErr bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdErr

	err := cmd.Run()
	if err != nil {

		return nil, fmt.Errorf("error getting git diff: %s", cmdErr.String())
	}

	files, _, err := gitdiff.Parse(&cmdOut)
	if err != nil {
		return nil, err
	}
	return files, nil
}
