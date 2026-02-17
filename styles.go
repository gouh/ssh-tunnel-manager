package main

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#61AFEF"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E06C75")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98C379")).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5C07B")).
			Bold(true)

	subtleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370"))

	activeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98C379"))

	inactiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E06C75"))

	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D19A66"))

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#5C6370")).
			Padding(1, 2)

	selectedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#C678DD")).
				Padding(1, 2)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ABB2BF")).
			Background(lipgloss.Color("#282C34")).
			Padding(0, 1)

	helpOverlayStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#1E2127")).
				Foreground(lipgloss.Color("#ABB2BF")).
				Padding(1, 2)

	helpTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#61AFEF")).
			Bold(true).
			Padding(0, 0, 1, 0)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D19A66"))

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370"))

	toastStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#E06C75")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Margin(1)

	toastSuccessStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#98C379")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1).
				Margin(1)

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5C07B")).
			Bold(true)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C678DD"))

	logTimeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370"))
)
