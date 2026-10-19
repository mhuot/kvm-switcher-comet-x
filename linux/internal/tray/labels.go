package tray

import (
	"strconv"
	"strings"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/kvm"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/settings"
)

// MaximumLabelLength is the longest port name shown beside the icon, in characters.
const MaximumLabelLength = 24

// PanelLabel is the text beside the panel icon for the configured label style.
func PanelLabel(switchState *kvm.SwitchState, current settings.Settings) string {
	if switchState == nil || current.LabelStyle == settings.LabelStyleIconOnly {
		return ""
	}
	portNumber := strconv.Itoa(switchState.ActivePortIndex + 1)
	if current.LabelStyle == settings.LabelStylePortNumber {
		return portNumber
	}
	activePort, found := switchState.ActivePort()
	if !found {
		return portNumber
	}
	nameCharacters := []rune(activePort.DisplayName(current))
	if len(nameCharacters) > MaximumLabelLength {
		return string(nameCharacters[:MaximumLabelLength-1]) + "…"
	}
	return string(nameCharacters)
}

// escapeMnemonics doubles underscores so dbusmenu shows them instead of underlining.
func escapeMnemonics(text string) string {
	return strings.ReplaceAll(text, "_", "__")
}

// MenuItemLabel is a port's menu text, with a "_1" mnemonic so typing 1-9 picks it.
func MenuItemLabel(port kvm.Port, current settings.Settings) string {
	mnemonic := ""
	if port.Index < 9 {
		mnemonic = "_" + strconv.Itoa(port.Index+1) + "   "
	}
	videoNote := ""
	if !port.HasVideoLink {
		videoNote = "  (no video)"
	}
	return mnemonic + escapeMnemonics(port.DisplayName(current)) + videoNote
}
