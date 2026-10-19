package setup

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// TerminalPrompter asks questions on a terminal. It also works with piped input, for
// scripts, though passwords are then read as plain lines.
type TerminalPrompter struct {
	input       *bufio.Reader
	inputFile   *os.File
	output      io.Writer
	inputClosed bool
}

// NewTerminalPrompter reads from stdin and writes to stdout.
func NewTerminalPrompter() *TerminalPrompter {
	return &TerminalPrompter{input: bufio.NewReader(os.Stdin), inputFile: os.Stdin, output: os.Stdout}
}

// NewScriptedPrompter reads answers from input (used by tests).
func NewScriptedPrompter(input io.Reader, output io.Writer) *TerminalPrompter {
	return &TerminalPrompter{input: bufio.NewReader(input), output: output}
}

func (prompter *TerminalPrompter) readLine() (string, bool) {
	if prompter.inputClosed {
		return "", false
	}
	line, err := prompter.input.ReadString('\n')
	if err != nil && line == "" {
		prompter.inputClosed = true
		fmt.Fprintln(prompter.output)
		return "", false
	}
	return strings.TrimRight(line, "\r\n"), true
}

func (prompter *TerminalPrompter) printWrapped(text string) {
	fmt.Fprintln(prompter.output, wrapText(text, 78))
}

// wrapText breaks paragraphs into lines of at most width characters.
func wrapText(text string, width int) string {
	var wrappedParagraphs []string
	for _, paragraph := range strings.Split(text, "\n") {
		var lines []string
		currentLine := ""
		for _, word := range strings.Fields(paragraph) {
			if currentLine != "" && len([]rune(currentLine))+1+len([]rune(word)) > width {
				lines = append(lines, currentLine)
				currentLine = word
				continue
			}
			if currentLine == "" {
				currentLine = word
			} else {
				currentLine += " " + word
			}
		}
		lines = append(lines, currentLine)
		wrappedParagraphs = append(wrappedParagraphs, strings.Join(lines, "\n"))
	}
	return strings.Join(wrappedParagraphs, "\n")
}

// Menu prints numbered rows and reads a number. Enter alone picks the last row ("Done").
func (prompter *TerminalPrompter) Menu(title, text string, rows []MenuRow) (string, bool) {
	for {
		fmt.Fprintf(prompter.output, "\n%s\n%s\n", title, strings.Repeat("=", len(title)))
		labelWidth := 0
		for _, row := range rows {
			labelWidth = max(labelWidth, len([]rune(row.Label)))
		}
		for rowIndex, row := range rows {
			if row.Value == "" {
				fmt.Fprintf(prompter.output, "  %d. %s\n", rowIndex+1, row.Label)
				continue
			}
			fmt.Fprintf(prompter.output, "  %d. %-*s  %s\n", rowIndex+1, labelWidth, row.Label, row.Value)
		}
		fmt.Fprintln(prompter.output)
		prompter.printWrapped(text)
		fmt.Fprintf(prompter.output, "Choose 1-%d (Enter for %s): ", len(rows), rows[len(rows)-1].Label)
		answer, answered := prompter.readLine()
		if !answered {
			return "", false
		}
		answer = strings.TrimSpace(answer)
		if answer == "" {
			return rows[len(rows)-1].Key, true
		}
		if chosenNumber, err := strconv.Atoi(answer); err == nil && chosenNumber >= 1 && chosenNumber <= len(rows) {
			return rows[chosenNumber-1].Key, true
		}
		fmt.Fprintf(prompter.output, "Type a number from 1 to %d.\n", len(rows))
	}
}

// Text asks for a line of text. Enter keeps the current value.
func (prompter *TerminalPrompter) Text(_, prompt, current string) (string, bool) {
	fmt.Fprintln(prompter.output)
	prompter.printWrapped(prompt)
	if current != "" {
		fmt.Fprintf(prompter.output, "[%s]: ", current)
	} else {
		fmt.Fprint(prompter.output, ": ")
	}
	answer, answered := prompter.readLine()
	if !answered {
		return "", false
	}
	if strings.TrimSpace(answer) == "" {
		return current, true
	}
	return answer, true
}

// Password asks for a password without echoing it when stdin is a terminal.
func (prompter *TerminalPrompter) Password(_, prompt string) (string, bool) {
	fmt.Fprintln(prompter.output)
	prompter.printWrapped(prompt)
	fmt.Fprint(prompter.output, "Password: ")
	if prompter.inputFile != nil && term.IsTerminal(int(prompter.inputFile.Fd())) {
		passwordBytes, err := term.ReadPassword(int(prompter.inputFile.Fd()))
		fmt.Fprintln(prompter.output)
		if err != nil {
			return "", false
		}
		return string(passwordBytes), true
	}
	return prompter.readLine()
}

// Choice prints numbered options and reads a number. Enter keeps the current choice.
func (prompter *TerminalPrompter) Choice(_, prompt string, options []Option, selectedKey string) (string, bool) {
	for {
		fmt.Fprintln(prompter.output)
		prompter.printWrapped(prompt)
		for optionIndex, option := range options {
			marker := " "
			if option.Key == selectedKey {
				marker = "*"
			}
			fmt.Fprintf(prompter.output, " %s%d. %s\n", marker, optionIndex+1, option.Label)
		}
		fmt.Fprintf(prompter.output, "Choose 1-%d (Enter keeps *): ", len(options))
		answer, answered := prompter.readLine()
		if !answered {
			return "", false
		}
		answer = strings.TrimSpace(answer)
		if answer == "" {
			return selectedKey, selectedKey != ""
		}
		if chosenNumber, err := strconv.Atoi(answer); err == nil && chosenNumber >= 1 && chosenNumber <= len(options) {
			return options[chosenNumber-1].Key, true
		}
		fmt.Fprintf(prompter.output, "Type a number from 1 to %d.\n", len(options))
	}
}

// YesNo asks a question; anything but y/yes means no.
func (prompter *TerminalPrompter) YesNo(_, question string) bool {
	fmt.Fprintln(prompter.output)
	prompter.printWrapped(question)
	fmt.Fprint(prompter.output, "[y/N]: ")
	answer, _ := prompter.readLine()
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

// Info prints a message.
func (prompter *TerminalPrompter) Info(_, message string) {
	fmt.Fprintln(prompter.output)
	prompter.printWrapped(message)
}

// Error prints a message marked as an error.
func (prompter *TerminalPrompter) Error(_, message string) {
	fmt.Fprintln(prompter.output)
	prompter.printWrapped("Error: " + message)
}

// Busy prints the message; there is nothing to hide afterwards.
func (prompter *TerminalPrompter) Busy(_, message string) func() {
	fmt.Fprintln(prompter.output)
	fmt.Fprintln(prompter.output, message)
	return func() {}
}
