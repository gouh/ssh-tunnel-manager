package main

// SSH Tunnel Manager - Architecture Overview
//
// Main Goroutine (Navigator):
//   - Runs the Bubbletea TUI program
//   - Handles user input (keyboard, resize events)
//   - Refreshes UI every 100ms via tickMsg
//   - Navigates between different tunnel views
//   - Coordinates all tunnel goroutines
//
// Tunnel Goroutines (Workers):
//   - One goroutine per active tunnel
//   - Reads SSH stderr output independently
//   - Updates tunnel logs with mutex protection
//   - Runs until SSH connection closes
//
// Communication:
//   - Tunnels update logs directly (thread-safe with mutex)
//   - Navigator reads logs when rendering UI
//   - No blocking channels or message passing
//   - Clean separation: workers write, navigator reads

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/moby/moby/pkg/namesgenerator"
)

type view int

const (
	viewMain view = iota
	viewNewTunnel
	viewQuitConfirm
	viewDeleteConfirm
	viewHelp
	maxHostVisible = 10
)

type tunnelStep int

const (
	stepHost tunnelStep = iota
	stepHostIP
	stepManualHost
	stepRemotePort
	stepLocalPort
	stepTag
	stepVerbose
	stepConnecting
)

type tunnel struct {
	id         int
	tag        string
	host       string
	localPort  string
	remotePort string
	verbose    bool
	cmd        *exec.Cmd
	logs       []string
	active     bool
	logChan    chan string
	logMutex   sync.Mutex
}

// Implement list.Item interface for tunnel
func (t tunnel) FilterValue() string { return t.tag }
func (t tunnel) Title() string       { return t.tag }
func (t tunnel) Description() string {
	status := "●"
	if t.active {
		status = "🟢"
	} else {
		status = "🔴"
	}
	return fmt.Sprintf("%s %s  %s → %s", status, t.host, t.localPort, t.remotePort)
}

type model struct {
	view            view
	tunnels         []tunnel
	tunnelList      list.Model
	selectedPanel   int
	selectedTunnel  int
	logScroll       int
	deleteTunnelIdx int

	step         tunnelStep
	hosts        []string
	hostIPs      []string
	hostIPIndex  int
	hostIPScroll int
	cursor       int
	hostScroll   int
	input        string
	tempHost     string
	tempRemote   string
	tempLocal    string
	tempTag      string
	tempVerbose  bool
	err          error
	spinner      spinner.Model

	nextTunnelID int
	width        int
	height       int
	program      *tea.Program

	toast         string
	toastType     string
	toastTimer    time.Time
	statusMessage string
}

const banner = `
  ╭─────────────────────────────────────────────╮
  │           SSH TUNNEL MANAGER                │
  ╰─────────────────────────────────────────────╯
`

// Custom list delegate for fancy rendering
type tunnelDelegate struct{}

func (d tunnelDelegate) Height() int                             { return 3 }
func (d tunnelDelegate) Spacing() int                            { return 1 }
func (d tunnelDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d tunnelDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	t, ok := listItem.(tunnel)
	if !ok {
		return
	}

	var str string
	if index == m.Index() {
		str = selectedStyle.Render(fmt.Sprintf("▶ %s", t.Title())) + "\n"
		str += selectedStyle.Render(fmt.Sprintf("  %s", t.Description()))
	} else {
		str = subtleStyle.Render(fmt.Sprintf("  %s", t.Title())) + "\n"
		str += subtleStyle.Render(fmt.Sprintf("  %s", t.Description()))
	}

	fmt.Fprint(w, str)
}

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#C678DD"))

	// Initialize list
	delegate := tunnelDelegate{}
	tunnelList := list.New([]list.Item{}, delegate, 0, 0)
	tunnelList.Title = "ACTIVE TUNNELS"
	tunnelList.SetShowStatusBar(false)
	tunnelList.SetFilteringEnabled(false)
	tunnelList.SetShowHelp(false)
	tunnelList.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#61AFEF")).
		Bold(true).
		Padding(0, 0, 1, 0)

	return model{
		view:          viewMain,
		hosts:         getSSHHosts(),
		selectedPanel: 0,
		nextTunnelID:  1,
		spinner:       s,
		tunnelList:    tunnelList,
		statusMessage: "Ready • Press ? for help",
	}
}

type logMsg struct {
	tunnelID int
	line     string
}

type connectingMsg struct{}

type tickMsg time.Time

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*1, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func waitForConnection() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return connectingMsg{}
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tickMsg:
		// Main UI refresh tick - the navigator polls all tunnel goroutines
		// and updates the display without blocking
		return m, tickCmd()

	case connectingMsg:
		if m.view == viewNewTunnel && m.step == stepConnecting {
			return m.finalizeTunnel()
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case logMsg:
		// Legacy - logs are now updated directly by goroutines
		break

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update list size
		listWidth := 36
		listHeight := msg.Height - 12
		if listHeight < 5 {
			listHeight = 5
		}
		m.tunnelList.SetSize(listWidth, listHeight)

	case tea.MouseMsg:
		if msg.Type == tea.MouseLeft {
			y := msg.Y - 4
			x := msg.X
			panelWidth := 40

			if m.view == viewMain {
				if x < panelWidth {
					m.selectedPanel = 0
					if y >= 3 && y < len(m.tunnels)+3 {
						idx := y - 3
						if idx < len(m.tunnels) {
							m.tunnelList.Select(idx)
							m.selectedTunnel = idx
						}
					}
				} else {
					m.selectedPanel = 1
				}
			}
		}

	case tea.KeyMsg:
		// Handle text input first for forms
		if m.view == viewNewTunnel && (m.step == stepRemotePort || m.step == stepLocalPort || m.step == stepTag || m.step == stepManualHost) {
			switch msg.String() {
			case "esc":
				m.view = viewMain
				return m, nil
			case "enter":
				return m.handleEnter()
			case "backspace":
				if len(m.input) > 0 {
					m.input = m.input[:len(m.input)-1]
				}
			default:
				if m.step == stepRemotePort || m.step == stepLocalPort {
					if len(msg.String()) == 1 && msg.String()[0] >= '0' && msg.String()[0] <= '9' {
						m.input += msg.String()
					}
				} else if m.step == stepTag {
					if len(msg.String()) == 1 {
						c := msg.String()[0]
						if c == ' ' {
							m.input += "_"
						} else if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
							m.input += msg.String()
						} else if c >= 'A' && c <= 'Z' {
							m.input += strings.ToLower(msg.String())
						}
					}
				} else if m.step == stepManualHost {
					if len(msg.String()) == 1 {
						c := msg.String()[0]
						if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '@' {
							m.input += msg.String()
						}
					}
				}
			}
			return m, nil
		}

		// Handle other commands
		switch msg.String() {
		case "?":
			if m.view == viewMain {
				m.view = viewHelp
				return m, nil
			}

		case "ctrl+c", "q":
			if m.view == viewQuitConfirm {
				// Already in quit confirm, force quit
				for i := range m.tunnels {
					if m.tunnels[i].active && m.tunnels[i].cmd != nil {
						m.tunnels[i].cmd.Process.Kill()
					}
				}
				return m, tea.Quit
			}
			// If in help view, just close it
			if m.view == viewHelp {
				m.view = viewMain
				return m, nil
			}
			// Show quit confirmation
			m.view = viewQuitConfirm
			return m, nil

		case "tab":
			if m.view == viewMain {
				m.selectedPanel = (m.selectedPanel + 1) % 2 // Only 2 panels now
			}

		case "n":
			if m.view == viewMain && m.selectedPanel == 0 {
				m.view = viewNewTunnel
				m.step = stepHost
				m.cursor = 0
				m.hostScroll = 0
				m.input = ""
				m.err = nil
			} else if m.view == viewQuitConfirm {
				m.view = viewMain
			}

		case "N":
			if m.view == viewQuitConfirm {
				m.view = viewMain
			}

		case "m":
			if m.view == viewNewTunnel && m.step == stepHost {
				m.step = stepManualHost
				m.input = ""
			}

		case "esc", "escape":
			if m.view == viewNewTunnel && m.step == stepHostIP {
				m.step = stepHost
				m.hostIPs = nil
			} else if m.view == viewNewTunnel {
				m.view = viewMain
			} else if m.view == viewQuitConfirm {
				m.view = viewMain
			} else if m.view == viewDeleteConfirm {
				m.view = viewMain
			} else if m.view == viewHelp {
				m.view = viewMain
			}

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.view == viewMain && m.selectedPanel == 0 {
				// Let list handle navigation
			} else if m.view == viewMain && m.selectedPanel == 1 {
				// Scroll logs up
				if m.logScroll > 0 {
					m.logScroll--
				}
			} else if m.view == viewNewTunnel && m.step == stepHost {
				if m.cursor > 0 {
					m.cursor--
				}
				if m.cursor < m.hostScroll {
					m.hostScroll = m.cursor
				}
			} else if m.view == viewNewTunnel && m.step == stepHostIP {
				if m.hostIPIndex > 0 {
					m.hostIPIndex--
				}
				if m.hostIPIndex < m.hostIPScroll {
					m.hostIPScroll = m.hostIPIndex
				}
			}

		case "down", "j":
			if m.view == viewMain && m.selectedPanel == 0 {
				// Let list handle navigation
			} else if m.view == viewMain && m.selectedPanel == 1 {
				// Scroll logs down
				if len(m.tunnels) > 0 && m.selectedTunnel < len(m.tunnels) {
					maxScroll := len(m.tunnels[m.selectedTunnel].logs) - 1
					if m.logScroll < maxScroll {
						m.logScroll++
					}
				}
			} else if m.view == viewNewTunnel && m.step == stepHost {
				if m.cursor < len(m.hosts)-1 {
					m.cursor++
				}
				if m.cursor >= m.hostScroll+maxHostVisible {
					m.hostScroll = m.cursor - maxHostVisible + 1
				}
			} else if m.view == viewNewTunnel && m.step == stepHostIP {
				if m.hostIPIndex < len(m.hostIPs)-1 {
					m.hostIPIndex++
				}
				if m.hostIPIndex >= m.hostIPScroll+maxHostVisible {
					m.hostIPScroll = m.hostIPIndex - maxHostVisible + 1
				}
			}

		case "d":
			if m.view == viewMain && m.selectedPanel == 0 && len(m.tunnels) > 0 {
				idx := m.tunnelList.Index()
				if idx < len(m.tunnels) {
					m.view = viewDeleteConfirm
					m.deleteTunnelIdx = idx
				}
			}

		case "y", "Y":
			if m.view == viewNewTunnel && m.step == stepVerbose {
				m.tempVerbose = true
				m.step = stepConnecting
				return m, tea.Batch(m.spinner.Tick, waitForConnection())
			} else if m.view == viewQuitConfirm {
				// Confirm quit
				for i := range m.tunnels {
					if m.tunnels[i].active && m.tunnels[i].cmd != nil {
						m.tunnels[i].cmd.Process.Kill()
					}
				}
				return m, tea.Quit
			} else if m.view == viewDeleteConfirm {
				idx := m.deleteTunnelIdx
				if idx < len(m.tunnels) {
					if m.tunnels[idx].active && m.tunnels[idx].cmd != nil {
						m.tunnels[idx].cmd.Process.Kill()
						m.tunnels[idx].active = false
					}
					m.tunnels = append(m.tunnels[:idx], m.tunnels[idx+1:]...)
					m.updateTunnelList()
					if idx >= len(m.tunnels) && idx > 0 {
						m.selectedTunnel = idx - 1
					}
				}
				m.view = viewMain
			}
		}
	}

	// Update list if in main view and left panel selected
	if m.view == viewMain && m.selectedPanel == 0 {
		var cmd tea.Cmd
		m.tunnelList, cmd = m.tunnelList.Update(msg)
		m.selectedTunnel = m.tunnelList.Index()
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) updateTunnelList() {
	items := make([]list.Item, len(m.tunnels))
	for i, t := range m.tunnels {
		items[i] = t
	}
	m.tunnelList.SetItems(items)
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	if m.view == viewNewTunnel {
		switch m.step {
		case stepHost:
			selectedHost := m.hosts[m.cursor]
			m.hostIPs = extractAllHostnames(selectedHost)
			if len(m.hostIPs) > 1 {
				m.step = stepHostIP
				m.hostIPIndex = 0
				m.hostIPScroll = 0
			} else {
				m.tempHost = extractHostname(selectedHost)
				m.step = stepRemotePort
			}

		case stepHostIP:
			m.tempHost = m.hostIPs[m.hostIPIndex]
			m.hostIPs = nil
			m.step = stepRemotePort

		case stepManualHost:
			if m.input != "" {
				m.tempHost = m.input
				m.input = ""
				m.step = stepRemotePort
			}

		case stepRemotePort:
			if m.input != "" {
				m.tempRemote = m.input
				m.input = ""
				m.step = stepLocalPort
			}

		case stepLocalPort:
			if m.input != "" {
				if isPortInUse(m.input) {
					m.err = fmt.Errorf("port %s is already in use", m.input)
					m.input = ""
				} else {
					m.tempLocal = m.input
					m.input = ""
					m.err = nil
					m.step = stepTag
				}
			}

		case stepTag:
			if m.input == "" {
				m.tempTag = namesgenerator.GetRandomName(0)
			} else {
				m.tempTag = m.input
			}
			m.input = ""
			m.step = stepVerbose

		case stepVerbose:
			m.tempVerbose = false
			m.step = stepConnecting
			return m, tea.Batch(m.spinner.Tick, waitForConnection())
		}
	}
	return m, nil
}

func (m *model) finalizeTunnel() (tea.Model, tea.Cmd) {
	args := []string{"-N", "-L", fmt.Sprintf("%s:localhost:%s", m.tempLocal, m.tempRemote)}
	if m.tempVerbose {
		args = append(args, "-v")
	}
	args = append(args, m.tempHost)

	cmd := exec.Command("ssh", args...)

	// Create pipes for stderr (SSH outputs to stderr)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return m, nil
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return m, nil
	}

	tunnelID := m.nextTunnelID

	t := tunnel{
		id:         tunnelID,
		tag:        m.tempTag,
		host:       m.tempHost,
		localPort:  m.tempLocal,
		remotePort: m.tempRemote,
		verbose:    m.tempVerbose,
		cmd:        cmd,
		active:     true,
		logs:       []string{fmt.Sprintf("[%s] Tunnel started", time.Now().Format("15:04:05"))},
	}

	m.tunnels = append(m.tunnels, t)
	m.nextTunnelID++
	m.view = viewMain
	m.selectedTunnel = len(m.tunnels) - 1
	m.updateTunnelList()

	// Start dedicated goroutine for this tunnel's log stream
	// This goroutine runs independently and updates logs in background
	go m.streamTunnelLogs(&m.tunnels[len(m.tunnels)-1], stderr)

	return m, nil
}

// streamTunnelLogs runs in a separate goroutine per tunnel
// It reads from stderr and updates the tunnel's logs independently
func (m *model) streamTunnelLogs(tun *tunnel, stderr io.ReadCloser) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			tun.logMutex.Lock()
			tun.logs = append(tun.logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), line))
			if len(tun.logs) > 100 {
				tun.logs = tun.logs[1:]
			}
			tun.logMutex.Unlock()
		}
	}
}

func (m model) View() string {
	// Top bar
	topBar := m.renderTopBar()

	// Always render main view first
	mainContent := topBar + "\n" + m.renderMainView()

	// Render status bar
	statusBar := m.renderStatusBar()
	mainContent = mainContent + "\n" + statusBar

	// Overlay modals on top
	if m.view == viewQuitConfirm {
		return m.renderModalOverlay(mainContent, m.renderQuitConfirm())
	}

	if m.view == viewDeleteConfirm {
		return m.renderModalOverlay(mainContent, m.renderDeleteConfirm())
	}

	if m.view == viewNewTunnel {
		return m.renderModalOverlay(mainContent, m.renderNewTunnelForm())
	}

	if m.view == viewHelp {
		return m.renderModalOverlay(mainContent, m.renderHelp())
	}

	return mainContent
}

func (m model) renderModalOverlay(background, modalContent string) string {
	backgroundLines := strings.Split(background, "\n")
	modalLines := strings.Split(modalContent, "\n")

	modalWidth := 0
	for _, ml := range modalLines {
		if len(ml) > modalWidth {
			modalWidth = len(ml)
		}
	}

	modalHeight := len(modalLines)
	bgHeight := len(backgroundLines)

	startRow := (bgHeight - modalHeight) / 2
	if startRow < 0 {
		startRow = 0
	}
	startCol := (m.width - modalWidth) / 2
	if startCol < 0 {
		startCol = 0
	}

	var result strings.Builder

	for i := range backgroundLines {
		if i >= startRow && i < startRow+modalHeight {
			modalIdx := i - startRow
			spaces := strings.Repeat(" ", startCol)
			result.WriteString(spaces + modalLines[modalIdx])
		}
		if i < bgHeight-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func (m model) renderStatusBar() string {
	if m.width < 10 || m.statusMessage == "" {
		return ""
	}

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ABB2BF")).
		Background(lipgloss.Color("#282C34")).
		Padding(0, 1).
		Width(m.width - 2)

	return statusStyle.Render(m.statusMessage)
}

func (m model) renderHelp() string {
	var content strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#61AFEF")).
		Bold(true).
		Padding(0, 0, 1, 0)

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D19A66"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5C6370"))

	content.WriteString("  " + titleStyle.Render("Keyboard Shortcuts") + "\n\n")

	shortcuts := []struct {
		key  string
		desc string
	}{
		{"Tab", "Switch between panels"},
		{"n", "Create new tunnel"},
		{"d", "Delete selected tunnel"},
		{"↑/↓ or j/k", "Navigate tunnel list"},
		{"enter", "Select / Confirm"},
		{"esc", "Cancel / Go back"},
		{"q or ctrl+c", "Quit (with confirmation)"},
		{"?", "Show this help"},
	}

	for _, s := range shortcuts {
		content.WriteString("  " + keyStyle.Render(fmt.Sprintf("%-15s", s.key+": ")))
		content.WriteString(descStyle.Render(s.desc) + "\n")
	}

	content.WriteString("\n  " + titleStyle.Render("Tips") + "\n\n")
	content.WriteString("  " + descStyle.Render("• Click on tunnels to select them") + "\n")
	content.WriteString("  " + descStyle.Render("• Use scroll wheel to navigate") + "\n")
	content.WriteString("  " + descStyle.Render("• Press 'esc' to close this help") + "\n")

	content.WriteString("\n  " + descStyle.Render("Version: "+Version))

	helpStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#1E2127")).
		Foreground(lipgloss.Color("#ABB2BF")).
		Padding(1, 2)

	// Create panel with content
	modal := helpStyle.Render(content.String())

	// Center the panel on screen
	centered := lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, modal)

	return centered
}

func (m model) renderQuitConfirm() string {
	// Create modal content
	activeTunnels := 0
	for _, t := range m.tunnels {
		if t.active {
			activeTunnels++
		}
	}

	var content string
	content += errorStyle.Render("Quit Confirmation") + "\n\n"

	if activeTunnels > 0 {
		content += fmt.Sprintf("You have %s active tunnel(s).\n", highlightStyle.Render(fmt.Sprintf("%d", activeTunnels)))
		content += "All tunnels will be closed.\n\n"
	} else {
		content += "Are you sure you want to quit?\n\n"
	}

	content += successStyle.Render("Y") + subtleStyle.Render(" - Yes, quit   ") + errorStyle.Render("Any key") + subtleStyle.Render(" - Cancel")

	// Center content and use same style as new tunnel form
	centeredContent := lipgloss.NewStyle().Width(60).Align(lipgloss.Center).Render(content)
	modal := panelStyle.Width(60).Render(centeredContent)

	// Center vertically and horizontally
	centered := lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, modal)

	return centered
}

func (m model) renderDeleteConfirm() string {
	var content string
	content += errorStyle.Render("Delete Tunnel") + "\n\n"

	if m.selectedTunnel < len(m.tunnels) {
		t := m.tunnels[m.selectedTunnel]
		content += fmt.Sprintf("Delete tunnel %s?\n", highlightStyle.Render(t.tag))
		content += fmt.Sprintf("Host: %s → %s\n\n", t.host, t.remotePort)
	}

	content += successStyle.Render("Y") + subtleStyle.Render(" - Yes, delete   ") + errorStyle.Render("Any key") + subtleStyle.Render(" - Cancel")

	centeredContent := lipgloss.NewStyle().Width(60).Align(lipgloss.Center).Render(content)
	modal := panelStyle.Width(60).Render(centeredContent)

	centered := lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, modal)

	return centered
}

func (m model) renderTopBar() string {
	if m.width < 10 {
		return titleStyle.Render(banner)
	}

	title := "SSH TUNNEL MANAGER"
	version := "v" + Version
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370"))

	titleWithVersion := title + " " + versionStyle.Render(version)
	titleLen := len(title) + len(version) + 1
	padding := (m.width - titleLen - 4) / 2
	if padding < 0 {
		padding = 0
	}

	topBorder := "╭" + strings.Repeat("─", m.width-2) + "╮"
	titleLine := "│" + strings.Repeat(" ", padding) + titleWithVersion + strings.Repeat(" ", m.width-titleLen-padding-2) + "│"
	bottomBorder := "╰" + strings.Repeat("─", m.width-2) + "╯"

	return titleStyle.Render(topBorder + "\n" + titleLine + "\n" + bottomBorder)
}

func (m model) renderMainView() string {
	// Calculate dimensions
	if m.width < 80 || m.height < 20 {
		return subtleStyle.Render("Terminal too small. Please resize to at least 80x20")
	}

	sidebarWidth := 40
	bodyWidth := m.width - sidebarWidth - 4
	contentHeight := m.height - 8 // Reserve space for header and footer

	if bodyWidth < 10 {
		bodyWidth = 10
	}
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Sidebar: Active Tunnels
	sidebar := m.renderSidebar(sidebarWidth, contentHeight)

	// Body: Tunnel Output/Logs
	body := m.renderBody(bodyWidth, contentHeight)

	// Join sidebar and body
	content := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, body)

	// Footer: Actions/Help
	footer := m.renderFooter(m.width)

	return content + "\n" + footer
}

func (m model) renderSidebar(width, height int) string {
	style := panelStyle.Width(width).Height(height)
	if m.selectedPanel == 0 {
		style = selectedPanelStyle.Width(width).Height(height)
	}

	if len(m.tunnels) == 0 {
		content := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#61AFEF")).Render("ACTIVE TUNNELS") + "\n\n"
		content += subtleStyle.Render("No tunnels active\n\nPress 'n' to create one")
		return style.Render(content)
	}

	return style.Render(m.tunnelList.View())
}

func (m model) renderBody(width, height int) string {
	style := panelStyle.Width(width).Height(height)
	if m.selectedPanel == 1 {
		style = selectedPanelStyle.Width(width).Height(height)
	}

	if len(m.tunnels) == 0 || m.selectedTunnel >= len(m.tunnels) {
		content := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#61AFEF")).Render("TUNNEL OUTPUT") + "\n\n"
		content += subtleStyle.Render("No tunnel selected")
		return style.Render(content)
	}

	t := m.tunnels[m.selectedTunnel]

	var content strings.Builder

	// Header info (no glamour needed here)
	content.WriteString(successStyle.Render(fmt.Sprintf("▶ %s", t.tag)) + "\n\n")
	content.WriteString(fmt.Sprintf("Host: %s\n", selectedStyle.Render(t.host)))
	content.WriteString(fmt.Sprintf("Local Port: %s\n", selectedStyle.Render(t.localPort)))
	content.WriteString(fmt.Sprintf("Remote Port: %s\n", selectedStyle.Render(t.remotePort)))

	if t.active {
		content.WriteString(fmt.Sprintf("Status: %s\n\n", activeStyle.Render("🟢 ACTIVE")))
	} else {
		content.WriteString(fmt.Sprintf("Status: %s\n\n", inactiveStyle.Render("🔴 INACTIVE")))
	}

	content.WriteString(highlightStyle.Render("Logs:") + "\n")
	content.WriteString(strings.Repeat("─", width-6) + "\n")

	// Calculate available lines for logs
	availableLines := height - 12
	if availableLines < 1 {
		availableLines = 1
	}

	// Only read the last N logs we need
	t.logMutex.Lock()
	totalLogs := len(t.logs)
	start := totalLogs - availableLines
	if start < 0 {
		start = 0
	}
	visibleLogs := make([]string, totalLogs-start)
	copy(visibleLogs, t.logs[start:])
	t.logMutex.Unlock()

	if len(visibleLogs) > 0 {
		maxWidth := width - 8 // Account for padding and borders
		for i, log := range visibleLogs {
			// Truncate long lines to prevent overflow
			if len(log) > maxWidth {
				log = log[:maxWidth-3] + "..."
			}
			content.WriteString(subtleStyle.Render(log))
			if i < len(visibleLogs)-1 {
				content.WriteString("\n")
			}
		}
	} else {
		content.WriteString(subtleStyle.Render("No logs yet...") + "\n")
	}

	return style.Render(content.String())
}

func (m model) renderFooter(width int) string {
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D19A66"))

	leftHelp := keyStyle.Render("Tab") + ": switch"
	centerHelp := ""
	rightHelp := keyStyle.Render("?") + ": help"

	if m.selectedPanel == 0 {
		centerHelp = keyStyle.Render("n") + ": new  " + keyStyle.Render("d") + ": delete  " + keyStyle.Render("↑/↓") + ": nav"
	} else if m.selectedPanel == 1 {
		centerHelp = keyStyle.Render("↑/↓") + ": scroll"
	}

	leftStyle := subtleStyle.Width(width / 3).Align(lipgloss.Left)
	centerStyle := subtleStyle.Width(width / 3).Align(lipgloss.Center)
	rightStyle := subtleStyle.Width(width / 3).Align(lipgloss.Right)

	footer := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftStyle.Render(leftHelp),
		centerStyle.Render(centerHelp),
		rightStyle.Render(rightHelp),
	)

	return "\n" + footer
}

func (m model) renderNewTunnelForm() string {
	var content string

	switch m.step {
	case stepHost:
		maxVisible := maxHostVisible
		start := m.hostScroll
		end := start + maxVisible
		if end > len(m.hosts) {
			end = len(m.hosts)
		}

		content = lipgloss.NewStyle().Bold(true).Render("Select SSH Host:") + "\n\n"
		for i := start; i < end; i++ {
			if m.cursor == i {
				content += selectedStyle.Render(fmt.Sprintf("  ▶  %s", m.hosts[i]))
			} else {
				content += fmt.Sprintf("     %s", m.hosts[i])
			}
			if i < end-1 {
				content += "\n"
			}
		}

		if len(m.hosts) > maxVisible {
			content += "\n\n" + subtleStyle.Render(fmt.Sprintf("(%d/%d) ↑/↓ to scroll • Enter to select • m for manual • Esc to cancel", m.cursor+1, len(m.hosts)))
		} else {
			content += "\n\n" + subtleStyle.Render("↑/↓ to move • Enter to select • m for manual • Esc to cancel")
		}

	case stepHostIP:
		hostName := m.hosts[m.cursor]
		content = lipgloss.NewStyle().Bold(true).Render("Select IP for "+hostName+":") + "\n\n"
		start := m.hostIPScroll
		end := start + maxHostVisible
		if end > len(m.hostIPs) {
			end = len(m.hostIPs)
		}
		for i := start; i < end; i++ {
			if m.hostIPIndex == i {
				content += selectedStyle.Render(fmt.Sprintf("  ▶  %s", m.hostIPs[i]))
			} else {
				content += fmt.Sprintf("     %s", m.hostIPs[i])
			}
			if i < end-1 {
				content += "\n"
			}
		}
		content += "\n\n" + subtleStyle.Render("↑/↓ to move • Enter to select • Esc to go back")

	case stepRemotePort:
		content = "Host: " + selectedStyle.Render(m.tempHost) + "\n\n"
		content += fmt.Sprintf("Remote port: %s█", m.input)
		content += "\n\n" + subtleStyle.Render("Enter port number • Esc to cancel")

	case stepLocalPort:
		content = "Remote port: " + successStyle.Render(m.tempRemote) + "\n\n"
		content += fmt.Sprintf("Local port: %s█", m.input)
		if m.err != nil {
			content += "\n\n" + errorStyle.Render("❌ "+m.err.Error())
		}
		content += "\n\n" + subtleStyle.Render("Enter port number • Esc to cancel")

	case stepTag:
		content = "Tag for this tunnel:\n\n"
		content += fmt.Sprintf("%s█", m.input)
		content += "\n\n" + subtleStyle.Render("Enter tag or press Enter for random • Esc to cancel")

	case stepVerbose:
		content = "Show verbose SSH logs? " + subtleStyle.Render("(y/n or just Enter for no)")

	case stepManualHost:
		content = lipgloss.NewStyle().Bold(true).Render("Enter SSH host manually:") + "\n\n"
		content += fmt.Sprintf("Host: %s█", m.input)
		content += "\n\n" + subtleStyle.Render("Format: user@host or host • Esc to cancel")

	case stepConnecting:
		content = highlightStyle.Render("Connecting to tunnel...") + "\n\n"
		content += m.spinner.View() + " " + subtleStyle.Render("Please wait...") + "\n\n"
		content += subtleStyle.Render(fmt.Sprintf("Host: %s\nPorts: %s → %s", m.tempHost, m.tempLocal, m.tempRemote))
	}

	// Create panel with content (text left-aligned)
	modal := panelStyle.Width(60).Render(content)

	// Center the panel on screen
	centered := lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, modal)

	return centered
}

func getSSHHosts() []string {
	file, err := os.Open(os.Getenv("HOME") + "/.ssh/config")
	if err != nil {
		return []string{}
	}
	defer file.Close()

	var hosts []string
	scanner := bufio.NewScanner(file)
	re := regexp.MustCompile(`^Host\s+(.+)`)

	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); matches != nil {
			host := strings.TrimSpace(matches[1])
			if host != "*" {
				hosts = append(hosts, host)
			}
		}
	}

	hosts = expandSSHHosts(hosts)
	return hosts
}

func expandSSHHosts(hosts []string) []string {
	file, err := os.Open(os.Getenv("HOME") + "/.ssh/config")
	if err != nil {
		return hosts
	}
	defer file.Close()

	var expanded []string
	currentHost := ""
	reHost := regexp.MustCompile(`^Host\s+(.+)`)
	reHostname := regexp.MustCompile(`(?i)^Hostname\s+(.+)`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if matches := reHost.FindStringSubmatch(line); matches != nil {
			currentHost = strings.TrimSpace(matches[1])
			if currentHost != "*" && currentHost != "" {
				expanded = append(expanded, currentHost)
			}
		} else if matches := reHostname.FindStringSubmatch(line); matches != nil && currentHost != "" {
			hostname := strings.TrimSpace(matches[1])
			if hostname != "" && !strings.HasPrefix(hostname, "*") {
				expanded = append(expanded, currentHost+" "+hostname)
			}
		}
	}

	if len(expanded) == 0 {
		return hosts
	}
	return expanded
}

func isPortInUse(port string) bool {
	cmd := exec.Command("ss", "-tuln")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), ":"+port+" ")
}

func extractHostname(hostWithIP string) string {
	parts := strings.Fields(hostWithIP)
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return hostWithIP
}

func extractAllHostnames(hostWithIP string) []string {
	parts := strings.Fields(hostWithIP)
	if len(parts) <= 1 {
		return []string{}
	}
	result := []string{parts[0]}
	result = append(result, parts[1:]...)
	return result
}

func main() {
	// Create the main TUI program (navigator)
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())

	// Run the navigator in the main goroutine
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
