package setup

import (
	"bytes"
	"os"
	"os/exec"
	"strings"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/desktop"
)

// DialogPrompter shows desktop dialogs by running zenity or kdialog. Neither is linked into
// KVM Switcher; they are separate programs most GNOME and KDE desktops already have.
type DialogPrompter struct {
	programPath string
	isKDialog   bool
}

// FindDialogPrompter returns a prompter for the desktop's dialog tool, preferring kdialog
// on KDE Plasma and zenity elsewhere. It returns false when there is no display or neither
// tool is installed.
func FindDialogPrompter() (*DialogPrompter, bool) {
	if !desktop.HasDisplay() {
		return nil, false
	}
	toolNames := []string{"zenity", "kdialog"}
	if strings.Contains(strings.ToUpper(os.Getenv("XDG_CURRENT_DESKTOP")), "KDE") {
		toolNames = []string{"kdialog", "zenity"}
	}
	for _, toolName := range toolNames {
		if programPath, err := exec.LookPath(toolName); err == nil {
			return &DialogPrompter{programPath: programPath, isKDialog: toolName == "kdialog"}, true
		}
	}
	return nil, false
}

// ToolName is "zenity" or "kdialog".
func (prompter *DialogPrompter) ToolName() string {
	if prompter.isKDialog {
		return "kdialog"
	}
	return "zenity"
}

// run returns the tool's output without the trailing newline, and false when the user
// cancelled (exit status 1) or the tool failed.
func (prompter *DialogPrompter) run(arguments ...string) (string, bool) {
	command := exec.Command(prompter.programPath, arguments...)
	var standardOutput bytes.Buffer
	command.Stdout = &standardOutput
	if err := command.Run(); err != nil {
		return "", false
	}
	return strings.TrimRight(standardOutput.String(), "\r\n"), true
}

// escapeMarkup protects zenity's Pango markup in --text.
func escapeMarkup(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
}

// Menu shows the settings as a list; the chosen row's key is returned.
func (prompter *DialogPrompter) Menu(title, text string, rows []MenuRow) (string, bool) {
	if prompter.isKDialog {
		arguments := []string{"--title", title, "--menu", text}
		for _, row := range rows {
			rowText := row.Label
			if row.Value != "" {
				rowText += ":  " + row.Value
			}
			arguments = append(arguments, row.Key, rowText)
		}
		return prompter.run(arguments...)
	}
	arguments := []string{
		"--list", "--title", title, "--text", escapeMarkup(text),
		"--column", "key", "--column", "Setting", "--column", "Value",
		"--hide-column", "1", "--print-column", "1",
		"--ok-label", "Change…", "--cancel-label", "Close",
		"--width", "620", "--height", "420",
	}
	for _, row := range rows {
		arguments = append(arguments, row.Key, row.Label, row.Value)
	}
	chosenKey, chosen := prompter.run(arguments...)
	// Some zenity versions print "key|key" after a double-click.
	chosenKey, _, _ = strings.Cut(chosenKey, "|")
	return chosenKey, chosen && chosenKey != ""
}

// Text asks for a line of text, pre-filled with the current value.
func (prompter *DialogPrompter) Text(title, prompt, current string) (string, bool) {
	if prompter.isKDialog {
		return prompter.run("--title", title, "--inputbox", prompt, current)
	}
	return prompter.run("--entry", "--title", title, "--text", escapeMarkup(prompt),
		"--entry-text", current, "--width", "460")
}

// Password asks for a password. The tool prints it to a pipe, never to the command line.
func (prompter *DialogPrompter) Password(title, prompt string) (string, bool) {
	if prompter.isKDialog {
		return prompter.run("--title", title, "--password", prompt)
	}
	return prompter.run("--entry", "--hide-text", "--title", title, "--text", escapeMarkup(prompt),
		"--width", "460")
}

// Choice shows radio buttons.
func (prompter *DialogPrompter) Choice(title, prompt string, options []Option, selectedKey string) (string, bool) {
	if prompter.isKDialog {
		arguments := []string{"--title", title, "--radiolist", prompt}
		for _, option := range options {
			state := "off"
			if option.Key == selectedKey {
				state = "on"
			}
			arguments = append(arguments, option.Key, option.Label, state)
		}
		return prompter.run(arguments...)
	}
	arguments := []string{
		"--list", "--radiolist", "--title", title, "--text", escapeMarkup(prompt),
		"--column", "", "--column", "key", "--column", "Option",
		"--hide-column", "2", "--print-column", "2", "--hide-header",
		"--width", "380", "--height", "260",
	}
	for _, option := range options {
		selected := "FALSE"
		if option.Key == selectedKey {
			selected = "TRUE"
		}
		arguments = append(arguments, selected, option.Key, option.Label)
	}
	chosenKey, chosen := prompter.run(arguments...)
	return chosenKey, chosen && chosenKey != ""
}

// YesNo asks a question.
func (prompter *DialogPrompter) YesNo(title, question string) bool {
	if prompter.isKDialog {
		_, answeredYes := prompter.run("--title", title, "--yesno", question)
		return answeredYes
	}
	_, answeredYes := prompter.run("--question", "--no-markup", "--title", title, "--text", question,
		"--width", "460")
	return answeredYes
}

// Info shows a message.
func (prompter *DialogPrompter) Info(title, message string) {
	if prompter.isKDialog {
		prompter.run("--title", title, "--msgbox", message)
		return
	}
	prompter.run("--info", "--no-markup", "--title", title, "--text", message, "--width", "460")
}

// Error shows an error message.
func (prompter *DialogPrompter) Error(title, message string) {
	if prompter.isKDialog {
		prompter.run("--title", title, "--error", message)
		return
	}
	prompter.run("--error", "--no-markup", "--title", title, "--text", message, "--width", "460")
}

// Busy shows a pulsing progress window until the returned function is called.
func (prompter *DialogPrompter) Busy(title, message string) func() {
	if prompter.isKDialog {
		prompter.run("--title", title, "--passivepopup", message, "3")
		return func() {}
	}
	command := exec.Command(prompter.programPath, "--progress", "--pulsate", "--no-cancel",
		"--auto-close", "--title", title, "--text", escapeMarkup(message), "--width", "360")
	progressInput, err := command.StdinPipe()
	if err != nil || command.Start() != nil {
		return func() {}
	}
	return func() {
		_ = progressInput.Close()
		_ = command.Process.Kill()
		_ = command.Wait()
	}
}
