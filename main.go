package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type step int

const (
	stepCheckVPN step = iota
	stepVPNPrompt
	stepStartingVPN
	stepHost
	stepRemotePort
	stepLocalPort
	stepVerbose
	stepConfirm
	stepRunning
)

type vpnCheckMsg bool
type vpnStartedMsg bool

type model struct {
	step         step
	vpnRunning   bool
	hosts        []string
	selectedHost int
	remotePort   string
	localPort    string
	verbose      bool
	input        string
	err          error
	cursor       int
	spinner      spinner.Model
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true)
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true)
	boxStyle      = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 3).
			Width(60)
	subtleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

const banner = `
  ╔═══════════════════════════════════════════════╗
  ║                                               ║
  ║         🚀  SSH TUNNEL MANAGER  🚀           ║
  ║                                               ║
  ║           Secure tunnels made easy            ║
  ║                                               ║
  ╚═══════════════════════════════════════════════╝
`

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	
	return model{
		step:    stepCheckVPN,
		spinner: s,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, checkVPNCmd())
}

func checkVPNCmd() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("pgrep", "-x", "openvpn")
		err := cmd.Run()
		return vpnCheckMsg(err == nil)
	}
}

func startVPNCmd() tea.Cmd {
	return func() tea.Msg {
		exec.Command("sudo", "pkill", "openvpn").Run()
		time.Sleep(2 * time.Second)
		exec.Command("sudo", "openvpn", "--config", os.Getenv("HOME")+"/vpn/mor/pfsense-sorg-UDP4-1194-hugo.hernandez.ovpn", "--daemon").Run()
		time.Sleep(5 * time.Second)
		return vpnStartedMsg(true)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case vpnCheckMsg:
		m.vpnRunning = bool(msg)
		if m.vpnRunning {
			m.hosts = getSSHHosts()
			m.step = stepHost
		} else {
			m.step = stepVPNPrompt
		}
		return m, nil

	case vpnStartedMsg:
		m.vpnRunning = true
		m.hosts = getSSHHosts()
		m.step = stepHost
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.step == stepHost && m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.step == stepHost && m.cursor < len(m.hosts)-1 {
				m.cursor++
			}

		case "y", "Y":
			if m.step == stepVPNPrompt {
				m.step = stepStartingVPN
				return m, tea.Batch(m.spinner.Tick, startVPNCmd())
			} else if m.step == stepVerbose {
				m.verbose = true
				m.step = stepConfirm
			}

		case "n", "N":
			if m.step == stepVPNPrompt {
				return m, tea.Quit
			} else if m.step == stepVerbose {
				m.verbose = false
				m.step = stepConfirm
			}

		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}

		default:
			if m.step == stepRemotePort || m.step == stepLocalPort {
				if len(msg.String()) == 1 && msg.String()[0] >= '0' && msg.String()[0] <= '9' {
					m.input += msg.String()
				}
			}
		}
	}
	return m, nil
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepHost:
		m.selectedHost = m.cursor
		m.step = stepRemotePort

	case stepRemotePort:
		if m.input != "" {
			m.remotePort = m.input
			m.input = ""
			m.step = stepLocalPort
		}

	case stepLocalPort:
		if m.input != "" {
			if isPortInUse(m.input) {
				m.err = fmt.Errorf("port %s is already in use", m.input)
				m.input = ""
			} else {
				m.localPort = m.input
				m.input = ""
				m.err = nil
				m.step = stepVerbose
			}
		}

	case stepConfirm:
		m.step = stepRunning
		go runTunnel(m.hosts[m.selectedHost], m.remotePort, m.localPort, m.verbose)
		return m, tea.Quit
	}
	return m, nil
}

func (m model) View() string {
	var s string

	s += titleStyle.Render(banner) + "\n\n"

	switch m.step {
	case stepCheckVPN:
		content := fmt.Sprintf("%s  Checking VPN status...", m.spinner.View())
		s += boxStyle.Render(content)

	case stepVPNPrompt:
		content := warningStyle.Render("⚠️  VPN is not running") + "\n\n"
		content += "Start VPN? " + subtleStyle.Render("(y/n)")
		s += boxStyle.Render(content)

	case stepStartingVPN:
		content := fmt.Sprintf("%s  Starting VPN...\n\n", m.spinner.View())
		content += subtleStyle.Render("This may take a few seconds...")
		s += boxStyle.Render(content)

	case stepHost:
		content := successStyle.Render("✅ VPN Connected") + "\n\n"
		content += lipgloss.NewStyle().Bold(true).Render("Select SSH Host:") + "\n\n"
		
		for i, host := range m.hosts {
			if m.cursor == i {
				content += selectedStyle.Render(fmt.Sprintf("  ▶  %s\n", host))
			} else {
				content += fmt.Sprintf("     %s\n", host)
			}
		}
		content += "\n" + subtleStyle.Render("↑/↓ to move • Enter to select")
		s += boxStyle.Render(content)

	case stepRemotePort:
		content := "Host: " + selectedStyle.Render(m.hosts[m.selectedHost]) + "\n\n"
		content += fmt.Sprintf("Remote port: %s█", m.input)
		s += boxStyle.Render(content)

	case stepLocalPort:
		content := "Remote port: " + successStyle.Render(m.remotePort) + "\n\n"
		content += fmt.Sprintf("Local port: %s█", m.input)
		if m.err != nil {
			content += "\n\n" + errorStyle.Render("❌ " + m.err.Error())
		}
		s += boxStyle.Render(content)

	case stepVerbose:
		content := "Show verbose SSH logs? " + subtleStyle.Render("(y/n)")
		s += boxStyle.Render(content)

	case stepConfirm:
		art := `
      ┌───────────────────────────┐
      │   🔗 TUNNEL READY 🔗     │
      └───────────────────────────┘
`
		content := lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Render(art) + "\n"
		content += fmt.Sprintf("  Host:    %s\n", successStyle.Render(m.hosts[m.selectedHost]))
		content += fmt.Sprintf("  Local:   %s\n", successStyle.Render("localhost:"+m.localPort))
		content += fmt.Sprintf("  Remote:  %s\n", successStyle.Render(m.remotePort))
		content += fmt.Sprintf("  Verbose: %s\n\n", successStyle.Render(fmt.Sprintf("%v", m.verbose)))
		content += subtleStyle.Render("Press Enter to start...")
		s += boxStyle.Render(content)
	}

	s += "\n" + subtleStyle.Render("ctrl+c or q to quit")
	return s
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

func runTunnel(host, remotePort, localPort string, verbose bool) {
	args := []string{"-N", "-L", fmt.Sprintf("%s:localhost:%s", localPort, remotePort)}
	if verbose {
		args = append(args, "-v")
	}
	args = append(args, host)

	fmt.Println("\n" + strings.Repeat("═", 50))
	fmt.Printf("🔗 Tunnel active: localhost:%s → %s:%s\n", localPort, host, remotePort)
	fmt.Println(strings.Repeat("═", 50))
	fmt.Println("\nPress Ctrl+C to stop\n")

	cmd := exec.Command("ssh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
