package views

import tea "github.com/charmbracelet/bubbletea"

// Test exports for package-private functions.

var FormatBytesExported = formatBytes
var FormatEstimateExported = formatEstimate

// FilterFlashExpiredMsgForTest returns the unexported filterFlashExpiredMsg for use in external test packages.
func FilterFlashExpiredMsgForTest() tea.Msg {
	return filterFlashExpiredMsg{}
}

// LocalSearchFlashExpiredMsgForTest returns the unexported localSearchFlashExpiredMsg for use in external test packages.
func LocalSearchFlashExpiredMsgForTest() tea.Msg {
	return localSearchFlashExpiredMsg{}
}

// IsLocalSearchFlashing exposes the localSearchFlashing field for tests.
func (m RowBrowserModel) IsLocalSearchFlashing() bool { return m.localSearchFlashing }
