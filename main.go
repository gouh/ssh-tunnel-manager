package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/moby/moby/pkg/namesgenerator"
)

type view int

const (
	viewMain view = iota
	viewNewTunnel
	viewQuitConfirm
)

type tunnelStep int

const (
	stepHost tunnelStep = iota
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
}

type model struct {
	view          view
	tunnels       []tunnel
	selectedPanel int // 0=tunnels list, 1=logs
	selectedTunnel int
	logScroll     int // Scroll position for logs
	
	// New tunnel form
	step         tunnelStep
	hosts        []string
	cursor       int
	input        string
	tempHost     string
	tempRemote   string
	tempLocal    string
	tempTag      string
	tempVerbose  bool
	err          error
	progress     progress.Model
	
	nextTunnelID int
	width        int
	height       int
}

var (
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF79C6"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Bold(true)
	successStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Bold(true)
	selectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD")).Bold(true)
	subtleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
	activeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))
	inactiveStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))
	highlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB86C"))
	
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#6272A4")).
			Padding(1, 2)
	
	selectedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#BD93F9")).
				Padding(1, 2)
)

const banner = `
  ╭─────────────────────────────────────────────╮
  │           SSH TUNNEL MANAGER                │
  ╰─────────────────────────────────────────────╯
`

func initialModel() model {
	prog := progress.New(progress.WithDefaultGradient())
	prog.Width = 40
	
	return model{
		view:          viewMain,
		hosts:         getSSHHosts(),
		selectedPanel: 0,
		nextTunnelID:  1,
		progress:      prog,
	}
}

type logMsg struct {
	tunnelID int
	line     string
}

type progressMsg float64

func (m model) Init() tea.Cmd {
	return nil
}

func tickProgress() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return progressMsg(0.1)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case progressMsg:
		if m.view == viewNewTunnel && m.step == stepConnecting {
			cmd := m.progress.IncrPercent(float64(msg))
			if m.progress.Percent() >= 1.0 {
				// Connection complete
				return m.finalizeTunnel()
			}
			return m, tea.Batch(cmd, tickProgress())
		}
		
	case logMsg:
		// Update logs for the specific tunnel
		for i := range m.tunnels {
			if m.tunnels[i].id == msg.tunnelID {
				if msg.line != "" {
					m.tunnels[i].logs = append(m.tunnels[i].logs, msg.line)
					// Keep only last 100 lines
					if len(m.tunnels[i].logs) > 100 {
						m.tunnels[i].logs = m.tunnels[i].logs[1:]
					}
				}
				return m, m.waitForLog(msg.tunnelID)
			}
		}
		return m, nil
		
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
	case tea.KeyMsg:
		switch msg.String() {
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

		case "esc":
			if m.view == viewNewTunnel {
				m.view = viewMain
			} else if m.view == viewQuitConfirm {
				m.view = viewMain
			}

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.view == viewMain && m.selectedPanel == 0 {
				if m.selectedTunnel > 0 {
					m.selectedTunnel--
					m.logScroll = 0 // Reset scroll when changing tunnel
				}
			} else if m.view == viewMain && m.selectedPanel == 1 {
				// Scroll logs up
				if m.logScroll > 0 {
					m.logScroll--
				}
			} else if m.view == viewNewTunnel && m.step == stepHost {
				if m.cursor > 0 {
					m.cursor--
				}
			}

		case "down", "j":
			if m.view == viewMain && m.selectedPanel == 0 {
				if m.selectedTunnel < len(m.tunnels)-1 {
					m.selectedTunnel++
					m.logScroll = 0 // Reset scroll when changing tunnel
				}
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
			}

		case "d":
			if m.view == viewMain && m.selectedPanel == 0 && len(m.tunnels) > 0 {
				// Close selected tunnel
				if m.tunnels[m.selectedTunnel].active && m.tunnels[m.selectedTunnel].cmd != nil {
					m.tunnels[m.selectedTunnel].cmd.Process.Kill()
					m.tunnels[m.selectedTunnel].active = false
				}
				// Remove from list
				m.tunnels = append(m.tunnels[:m.selectedTunnel], m.tunnels[m.selectedTunnel+1:]...)
				if m.selectedTunnel >= len(m.tunnels) && m.selectedTunnel > 0 {
					m.selectedTunnel--
				}
			}

		case "y", "Y":
			if m.view == viewNewTunnel && m.step == stepVerbose {
				m.tempVerbose = true
				m.step = stepConnecting
				m.progress.SetPercent(0)
				return m, tickProgress()
			} else if m.view == viewQuitConfirm {
				// Confirm quit
				for i := range m.tunnels {
					if m.tunnels[i].active && m.tunnels[i].cmd != nil {
						m.tunnels[i].cmd.Process.Kill()
					}
				}
				return m, tea.Quit
			}

		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}

		default:
			if m.view == viewQuitConfirm {
				// Any key except Y cancels
				m.view = viewMain
				return m, nil
			}
			
			if m.view == viewNewTunnel && (m.step == stepRemotePort || m.step == stepLocalPort) {
				if len(msg.String()) == 1 && msg.String()[0] >= '0' && msg.String()[0] <= '9' {
					m.input += msg.String()
				}
			} else if m.view == viewNewTunnel && m.step == stepTag {
				// Allow alphanumeric and hyphens for tags
				if len(msg.String()) == 1 {
					c := msg.String()[0]
					if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
						m.input += msg.String()
					}
				}
			} else if m.view == viewNewTunnel && m.step == stepManualHost {
				// Allow alphanumeric, dots, hyphens, @ for manual host
				if len(msg.String()) == 1 {
					c := msg.String()[0]
					if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '@' {
						m.input += msg.String()
					}
				}
			}
		}
	}
	return m, nil
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	if m.view == viewNewTunnel {
		switch m.step {
		case stepHost:
			m.tempHost = m.hosts[m.cursor]
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
			m.progress.SetPercent(0)
			return m, tickProgress()
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

	logChan := make(chan string, 100)
	
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
		logChan:    logChan,
	}

	// Start goroutine to read logs
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			logChan <- scanner.Text()
		}
		close(logChan)
	}()

	m.tunnels = append(m.tunnels, t)
	m.nextTunnelID++
	m.view = viewMain
	m.selectedTunnel = len(m.tunnels) - 1

	return m, m.waitForLog(tunnelID)
}


func (m *model) waitForLog(tunnelID int) tea.Cmd {
	return func() tea.Msg {
		// Find the tunnel
		for i := range m.tunnels {
			if m.tunnels[i].id == tunnelID && m.tunnels[i].logChan != nil {
				select {
				case line, ok := <-m.tunnels[i].logChan:
					if ok && line != "" {
						return logMsg{tunnelID: tunnelID, line: fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), line)}
					}
				case <-time.After(100 * time.Millisecond):
				}
			}
		}
		return nil
	}
}

func (m model) View() string {
	// Top bar
	topBar := m.renderTopBar()
	
	if m.view == viewQuitConfirm {
		return topBar + "\n" + m.renderQuitConfirm()
	}
	
	if m.view == viewNewTunnel {
		return topBar + "\n" + m.renderNewTunnelForm()
	}

	return topBar + "\n" + m.renderMainView()
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

func (m model) renderTopBar() string {
	if m.width < 10 {
		return titleStyle.Render(banner)
	}
	
	title := "SSH TUNNEL MANAGER"
	padding := (m.width - len(title) - 4) / 2
	if padding < 0 {
		padding = 0
	}
	
	topBorder := "╭" + strings.Repeat("─", m.width-2) + "╮"
	titleLine := "│" + strings.Repeat(" ", padding) + title + strings.Repeat(" ", m.width-len(title)-padding-2) + "│"
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

	sepWidth := width - 4
	if sepWidth < 1 {
		sepWidth = 1
	}

	content := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF79C6")).Render("ACTIVE TUNNELS") + "\n"
	content += strings.Repeat("─", sepWidth) + "\n\n"

	if len(m.tunnels) == 0 {
		content += subtleStyle.Render("No tunnels active\n\nPress 'n' to create one")
	} else {
		for i, t := range m.tunnels {
			status := activeStyle.Render("●")
			if !t.active {
				status = inactiveStyle.Render("●")
			}

			line := fmt.Sprintf("%s [%s]", status, t.tag)
			hostLine := fmt.Sprintf("  %s", t.host)
			portLine := fmt.Sprintf("  %s → %s", t.localPort, t.remotePort)
			
			if i == m.selectedTunnel && m.selectedPanel == 0 {
				content += selectedStyle.Render("▶ " + line) + "\n"
				content += selectedStyle.Render(hostLine) + "\n"
				content += selectedStyle.Render(portLine) + "\n"
			} else {
				content += "  " + line + "\n"
				content += subtleStyle.Render(hostLine) + "\n"
				content += subtleStyle.Render(portLine) + "\n"
			}
			
			if i < len(m.tunnels)-1 {
				content += "\n"
			}
		}
	}

	return style.Render(content)
}

func (m model) renderBody(width, height int) string {
	style := panelStyle.Width(width).Height(height)
	if m.selectedPanel == 1 {
		style = selectedPanelStyle.Width(width).Height(height)
	}

	sepWidth := width - 4
	if sepWidth < 1 {
		sepWidth = 1
	}

	content := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF79C6")).Render("TUNNEL OUTPUT") + "\n"
	content += strings.Repeat("─", sepWidth) + "\n\n"

	if len(m.tunnels) == 0 || m.selectedTunnel >= len(m.tunnels) {
		content += subtleStyle.Render("No tunnel selected")
	} else {
		t := m.tunnels[m.selectedTunnel]
		
		// Tunnel info (takes ~8 lines)
		infoLines := []string{
			successStyle.Render(fmt.Sprintf("[%s]", t.tag)),
			fmt.Sprintf("Host: %s", selectedStyle.Render(t.host)),
			fmt.Sprintf("Local Port: %s", selectedStyle.Render(t.localPort)),
			fmt.Sprintf("Remote Port: %s", selectedStyle.Render(t.remotePort)),
			fmt.Sprintf("Verbose: %v", t.verbose),
			fmt.Sprintf("Status: %s", func() string {
				if t.active {
					return activeStyle.Render("ACTIVE")
				}
				return inactiveStyle.Render("INACTIVE")
			}()),
			"",
			lipgloss.NewStyle().Bold(true).Render("Logs:"),
			strings.Repeat("─", sepWidth),
		}
		
		for _, line := range infoLines {
			content += line + "\n"
		}
		
		// Calculate available space for logs
		availableHeight := height - len(infoLines) - 4 // 4 for padding and borders
		if availableHeight < 1 {
			availableHeight = 1
		}
		
		// Show logs with scroll
		totalLogs := len(t.logs)
		if totalLogs > 0 {
			start := m.logScroll
			end := start + availableHeight
			
			if end > totalLogs {
				end = totalLogs
			}
			if start >= totalLogs {
				start = totalLogs - 1
				if start < 0 {
					start = 0
				}
			}
			
			for i := start; i < end; i++ {
				content += subtleStyle.Render(t.logs[i]) + "\n"
			}
			
			// Show scroll indicator
			if totalLogs > availableHeight {
				scrollInfo := fmt.Sprintf(" [%d-%d of %d]", start+1, end, totalLogs)
				content += "\n" + subtleStyle.Render(scrollInfo)
			}
		}
	}

	return style.Render(content)
}

func (m model) renderFooter(width int) string {
	leftHelp := "Tab: switch panel"
	centerHelp := ""
	rightHelp := "q: quit"
	
	if m.selectedPanel == 0 {
		centerHelp = "n: new tunnel • d: delete • ↑/↓: navigate"
	} else if m.selectedPanel == 1 {
		centerHelp = "↑/↓: scroll logs"
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
		content = lipgloss.NewStyle().Bold(true).Render("Select SSH Host:") + "\n\n"
		for i, host := range m.hosts {
			if m.cursor == i {
				content += selectedStyle.Render(fmt.Sprintf("  ▶  %s", host))
			} else {
				content += fmt.Sprintf("     %s", host)
			}
			if i < len(m.hosts)-1 {
				content += "\n"
			}
		}
		content += "\n\n" + subtleStyle.Render("↑/↓ to move • Enter to select • m for manual • Esc to cancel")

	case stepRemotePort:
		content = "Host: " + selectedStyle.Render(m.tempHost) + "\n\n"
		content += fmt.Sprintf("Remote port: %s█", m.input)
		content += "\n\n" + subtleStyle.Render("Enter port number • Esc to cancel")

	case stepLocalPort:
		content = "Remote port: " + successStyle.Render(m.tempRemote) + "\n\n"
		content += fmt.Sprintf("Local port: %s█", m.input)
		if m.err != nil {
			content += "\n\n" + errorStyle.Render("❌ " + m.err.Error())
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
		content += m.progress.View() + "\n\n"
		content += subtleStyle.Render(fmt.Sprintf("Host: %s\nPorts: %s → %s", m.tempHost, m.tempLocal, m.tempRemote))
	}

	return panelStyle.Width(60).Render(content)
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
	return hosts
}

func isPortInUse(port string) bool {
	cmd := exec.Command("ss", "-tuln")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), ":"+port+" ")
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
