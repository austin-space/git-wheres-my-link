package main

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/fatih/color"
)

func showFileAtLineNumber(fileName string, fileString string, lineNumber int64) error {
	lineCount := strings.Count(fileString, "\n")

	if lineCount < int(lineNumber) {
		return fmt.Errorf("line number %d exceeds file line count %d", lineNumber, lineCount)
	}

	lexer := lexers.Match(fileName)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	style := styles.Get("dracula")
	if style == nil {
		style = styles.Fallback
	}

	iteratator, err := lexer.Tokenise(nil, fileString)
	if err != nil {
		return err
	}

	var buffer bytes.Buffer
	formatters.TTY.Format(bufio.NewWriter(&buffer), style, iteratator)

	lines := strings.Split(buffer.String(), "\n")
	//startingLine := max(0, lineNumber-6)
	//endingLine := min(lineNumber+5, int64(len(lines)-1))

	// TODO: sometimes(for example comments) the style isn't reset after a new line.
	// We need to get compute what should be placed at the beginning of each line so we can handle the first line
	// and the focused line
	for i, line := range lines { //[startingLine:endingLine] {
		currentLine := int64(i) + 1
		lineNumberString := fmt.Sprintf("%5d", currentLine)
		if currentLine == lineNumber {
			color.New(color.BgGreen).Add(color.FgBlack).Print(lineNumberString)
			color.New(color.Reset).Print("")
		} else {
			fmt.Print(lineNumberString)
		}
		fmt.Println(line)
	}
	color.New(color.Reset).Print("")
	return nil
}
