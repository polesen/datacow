package views

import tea "github.com/charmbracelet/bubbletea"

// Test exports for package-private functions.

var FormatBytesExported = formatBytes
var FormatEstimateExported = formatEstimate

// FilterFlashExpiredMsgForTest returns the unexported filterFlashExpiredMsg for use in external test packages.
func FilterFlashExpiredMsgForTest() tea.Msg {
	return filterFlashExpiredMsg{}
}
