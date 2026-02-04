package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type view int

const (
	viewMain view = iota
	viewNewTunnel
)

type tunnelStep int

const (
	stepHost tunnelStep = iota
	stepRemotePort
	stepLocalPort
	stepVerbose
)

type tunnel struct {
	id         int
	host       string
	localPort  string
	remotePort string
	verbose    bool
	cmd        *exec.Cmd
	logs       []string
	active     bool
}

type model struct {
	view          view
	tunnels       []tunnel
	selectedPanel int // 0=tunnels list, 1=new tunnel, 2=logs
	selectedTunnel int
	
	// New tunnel form
	step         tunnelStep
	hosts        []string
	cursor       int
	input        string
	tempHost     string
	tempRemote   string
	tempLocal    string
	tempVerbose  bool
	err          error
	
	nextTunnelID int
	width        int
	height       int
}

var (
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	successStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true)
	selectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true)
	subtleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	activeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	inactiveStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2)
	
	selectedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("212")).
				Padding(1, 2)
)

const banner = `
  ╭─────────────────────────────────────────────╮
  │           SSH TUNNEL MANAGER                │
  ╰─────────────────────────────────────────────╯
`

func initialModel() model {
	return model{
		view:          viewMain,
		hosts:         getSSHHosts(),
		selectedPanel: 0,
		nextTunnelID:  1,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// Close all tunnels before quitting
			for i := range m.tunnels {
				if m.tunnels[i].active && m.tunnels[i].cmd != nil {
					m.tunnels[i].cmd.Process.Kill()
				}
			}
			return m, tea.Quit

		case "tab":
			if m.view == viewMain {
				m.selectedPanel = (m.selectedPanel + 1) % 3
			}

		case "n":
			if m.view == viewMain && m.selectedPanel == 0 {
				m.view = viewNewTunnel
				m.step = stepHost
				m.cursor = 0
				m.input = ""
				m.err = nil
			}

		case "esc":
			if m.view == viewNewTunnel {
				m.view = viewMain
			}

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.view == viewMain && m.selectedPanel == 0 {
				if m.selectedTunnel > 0 {
					m.selectedTunnel--
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
				return m.createTunnel()
			}

		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}

		default:
			if m.view == viewNewTunnel && (m.step == stepRemotePort || m.step == stepLocalPort) {
				if len(msg.String()) == 1 && msg.String()[0] >= '0' && msg.String()[0] <= '9' {
					m.input += msg.String()
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
					m.step = stepVerbose
				}
			}
		
		case stepVerbose:
			m.tempVerbose = false
			return m.createTunnel()
		}
	}
	return m, nil
}

func (m *model) createTunnel() (tea.Model, tea.Cmd) {
	args := []string{"-N", "-L", fmt.Sprintf("%s:localhost:%s", m.tempLocal, m.tempRemote)}
	if m.tempVerbose {
		args = append(args, "-v")
	}
	args = append(args, m.tempHost)

	cmd := exec.Command("ssh", args...)
	cmd.Start()

	t := tunnel{
		id:         m.nextTunnelID,
		host:       m.tempHost,
		localPort:  m.tempLocal,
		remotePort: m.tempRemote,
		verbose:    m.tempVerbose,
		cmd:        cmd,
		active:     true,
		logs:       []string{fmt.Sprintf("Tunnel started at %s", time.Now().Format("15:04:05"))},
	}

	m.tunnels = append(m.tunnels, t)
	m.nextTunnelID++
	m.view = viewMain
	m.selectedTunnel = len(m.tunnels) - 1

	return m, nil
}

func (m model) View() string {
	s := titleStyle.Render(banner) + "\n"

	if m.view == viewNewTunnel {
		return s + m.renderNewTunnelForm()
	}

	return s + m.renderMainView()
}

func (m model) renderMainView() string {
	// Three panels: Active Tunnels | Actions | Logs
	tunnelsPanel := m.renderTunnelsPanel()
	actionsPanel := m.renderActionsPanel()
	logsPanel := m.renderLogsPanel()

	row := lipgloss.JoinHorizontal(lipgloss.Top, tunnelsPanel, actionsPanel, logsPanel)
	
	help := "\n" + subtleStyle.Render("Tab: switch panel • n: new tunnel • d: delete • q: quit")
	
	return row + help
}

func (m model) renderTunnelsPanel() string {
	style := panelStyle
	if m.selectedPanel == 0 {
		style = selectedPanelStyle
	}

	content := lipgloss.NewStyle().Bold(true).Render("Active Tunnels") + "\n\n"

	if len(m.tunnels) == 0 {
		content += subtleStyle.Render("No tunnels active\nPress 'n' to create one")
	} else {
		for i, t := range m.tunnels {
			status := activeStyle.Render("●")
			if !t.active {
				status = inactiveStyle.Render("●")
			}

			line := fmt.Sprintf("%s #%d %s:%s→%s", status, t.id, t.host, t.localPort, t.remotePort)
			
			if i == m.selectedTunnel && m.selectedPanel == 0 {
				content += selectedStyle.Render("▶ " + line) + "\n"
			} else {
				content += "  " + line + "\n"
			}
		}
	}

	return style.Width(35).Height(15).Render(content)
}

func (m model) renderActionsPanel() string {
	style := panelStyle
	if m.selectedPanel == 1 {
		style = selectedPanelStyle
	}

	content := lipgloss.NewStyle().Bold(true).Render("Actions") + "\n\n"
	content += "n - New tunnel\n"
	content += "d - Delete tunnel\n"
	content += "↑/↓ - Navigate\n"
	content += "Tab - Switch panel\n"
	content += "q - Quit\n"

	return style.Width(25).Height(15).Render(content)
}

func (m model) renderLogsPanel() string {
	style := panelStyle
	if m.selectedPanel == 2 {
		style = selectedPanelStyle
	}

	content := lipgloss.NewStyle().Bold(true).Render("Logs") + "\n\n"

	if len(m.tunnels) == 0 || m.selectedTunnel >= len(m.tunnels) {
		content += subtleStyle.Render("No tunnel selected")
	} else {
		t := m.tunnels[m.selectedTunnel]
		content += fmt.Sprintf("Tunnel #%d\n", t.id)
		content += fmt.Sprintf("%s:%s → %s\n\n", t.host, t.localPort, t.remotePort)
		
		for _, log := range t.logs {
			content += subtleStyle.Render(log) + "\n"
		}
	}

	return style.Width(40).Height(15).Render(content)
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
		content += "\n\n" + subtleStyle.Render("↑/↓ to move • Enter to select • Esc to cancel")

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

	case stepVerbose:
		content = "Show verbose SSH logs? " + subtleStyle.Render("(y/n or just Enter for no)")
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
