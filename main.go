package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type step int

const (
	stepVPN step = iota
	stepHost
	stepRemotePort
	stepLocalPort
	stepVerbose
	stepConfirm
	stepRunning
)

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
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).MarginBottom(1)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true)
)

func initialModel() model {
	vpnRunning := checkVPN()
	hosts := getSSHHosts()
	return model{
		step:       stepVPN,
		vpnRunning: vpnRunning,
		hosts:      hosts,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
			if m.step == stepVPN && !m.vpnRunning {
				startVPN()
				m.vpnRunning = true
				m.step = stepHost
			} else if m.step == stepVerbose {
				m.verbose = true
				m.step = stepConfirm
			}

		case "n", "N":
			if m.step == stepVPN && !m.vpnRunning {
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
	case stepVPN:
		if m.vpnRunning {
			m.step = stepHost
		}

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
	s := titleStyle.Render("🔗 SSH Tunnel Manager") + "\n\n"

	switch m.step {
	case stepVPN:
		if m.vpnRunning {
			s += successStyle.Render("✅ VPN is running") + "\n\n"
			s += "Press Enter to continue..."
		} else {
			s += warningStyle.Render("⚠️  VPN is not running") + "\n\n"
			s += "Start VPN? (y/n): "
		}

	case stepHost:
		s += "Select SSH host:\n\n"
		for i, host := range m.hosts {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
				s += selectedStyle.Render(fmt.Sprintf("%s %d. %s\n", cursor, i+1, host))
			} else {
				s += fmt.Sprintf("%s %d. %s\n", cursor, i+1, host)
			}
		}
		s += "\n(↑/↓ to move, Enter to select)"

	case stepRemotePort:
		s += fmt.Sprintf("Selected host: %s\n\n", selectedStyle.Render(m.hosts[m.selectedHost]))
		s += fmt.Sprintf("Remote port: %s_\n", m.input)

	case stepLocalPort:
		s += fmt.Sprintf("Remote port: %s\n", m.remotePort)
		s += fmt.Sprintf("Local port: %s_\n", m.input)
		if m.err != nil {
			s += "\n" + errorStyle.Render(m.err.Error())
		}

	case stepVerbose:
		s += "Show verbose logs? (y/n): "

	case stepConfirm:
		s += successStyle.Render("Ready to create tunnel:") + "\n\n"
		s += fmt.Sprintf("  Host:   %s\n", m.hosts[m.selectedHost])
		s += fmt.Sprintf("  Local:  localhost:%s\n", m.localPort)
		s += fmt.Sprintf("  Remote: %s\n", m.remotePort)
		s += fmt.Sprintf("  Verbose: %v\n\n", m.verbose)
		s += "Press Enter to start..."
	}

	s += "\n\n(ctrl+c or q to quit)"
	return s
}

func checkVPN() bool {
	cmd := exec.Command("pgrep", "-x", "openvpn")
	return cmd.Run() == nil
}

func startVPN() {
	exec.Command("sudo", "pkill", "openvpn").Run()
	exec.Command("sudo", "openvpn", "--config", os.Getenv("HOME")+"/vpn/mor/pfsense-sorg-UDP4-1194-hugo.hernandez.ovpn", "--daemon").Run()
	exec.Command("sleep", "5").Run()
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

	fmt.Printf("\n🔗 Tunnel active: localhost:%s → %s:%s\n", localPort, host, remotePort)
	fmt.Println("Press Ctrl+C to stop\n")

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
