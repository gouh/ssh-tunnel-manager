# SSH Tunnel Manager

A beautiful terminal UI for managing multiple SSH tunnels simultaneously.

![SSH Tunnel Manager](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/license-MIT-blue.svg)

## Features

✨ **Multiple Tunnels** - Create and manage multiple SSH tunnels at once  
🎨 **Beautiful UI** - Modern Dracula-themed interface with Bubbletea  
📊 **Real-time Logs** - View SSH connection logs in real-time  
🏷️ **Auto-naming** - Docker-style automatic tunnel naming  
⌨️ **Keyboard Navigation** - Efficient keyboard-driven interface  
🔄 **Live Status** - See active/inactive tunnel status at a glance  

## Screenshots

```
  ╭─────────────────────────────────────────────╮
  │           SSH TUNNEL MANAGER                │
  ╰─────────────────────────────────────────────╯

╭─────────────────────────────────╮  ╭──────────────────────────────────╮
│ ACTIVE TUNNELS                  │  │ TUNNEL OUTPUT                    │
│ ─────────────────────────────── │  │ ──────────────────────────────── │
│                                 │  │                                  │
│ ▶ ● [brave-tesla]              │  │ [brave-tesla]                    │
│     server.example.com          │  │ Host: server.example.com         │
│     8080 → 80                   │  │ Local Port: 8080                 │
│                                 │  │ Remote Port: 80                  │
│   ● [happy-curie]              │  │ Status: ACTIVE                   │
│     db.example.com              │  │                                  │
│     5432 → 5432                 │  │ Logs:                            │
│                                 │  │ ──────────────────────────────── │
╰─────────────────────────────────╯  │ [15:04:05] Tunnel started        │
                                      │ [15:04:06] Connection established│
Tab: switch panel • n: new • d: delete  ╰──────────────────────────────────╯
```

## Installation

### Quick Install (Recommended)

Install the latest version with a single command:

```bash
curl -sSL https://raw.githubusercontent.com/gouh/ssh-tunnel-manager/main/install.sh | bash
```

Or with wget:

```bash
wget -qO- https://raw.githubusercontent.com/gouh/ssh-tunnel-manager/main/install.sh | bash
```

### Manual Installation

#### Download Pre-built Binaries

Download the latest release for your platform:

**Linux (x64)**
```bash
wget https://github.com/gouh/ssh-tunnel-manager/releases/latest/download/ssh-tunnel-manager-linux-amd64
chmod +x ssh-tunnel-manager-linux-amd64
sudo mv ssh-tunnel-manager-linux-amd64 /usr/local/bin/ssh-tunnel-manager
```

**macOS (Intel)**
```bash
wget https://github.com/gouh/ssh-tunnel-manager/releases/latest/download/ssh-tunnel-manager-darwin-amd64
chmod +x ssh-tunnel-manager-darwin-amd64
sudo mv ssh-tunnel-manager-darwin-amd64 /usr/local/bin/ssh-tunnel-manager
```

**macOS (Apple Silicon)**
```bash
wget https://github.com/gouh/ssh-tunnel-manager/releases/latest/download/ssh-tunnel-manager-darwin-arm64
chmod +x ssh-tunnel-manager-darwin-arm64
sudo mv ssh-tunnel-manager-darwin-arm64 /usr/local/bin/ssh-tunnel-manager
```

### Prerequisites

- SSH client installed
- SSH config file at `~/.ssh/config` (optional, for host selection)

### Build from Source

Requirements: Go 1.21 or higher

```bash
# Clone the repository
git clone https://github.com/gouh/ssh-tunnel-manager.git
cd ssh-tunnel-manager

# Build the binary
make build

# Install to system (optional)
sudo cp ssh-tunnel-manager /usr/local/bin/
```

## Development

### Version Management

This project uses semantic versioning. To create a new version:

```bash
make bump-version
```

This will:
1. Prompt you for the new version number
2. Ask you to enter the changes (one per line)
3. Update `version.go` with the new version
4. Update `CHANGELOG.md` with the changes
5. Create a git commit
6. Create a git tag

After the bump, push the changes:

```bash
git push && git push --tags
```

## Usage

### Starting the application

```bash
ssh-tunnel-manager
```

### Keyboard shortcuts

#### Main View
- `Tab` - Switch between panels (Tunnels / Logs)
- `n` - Create new tunnel
- `d` - Delete selected tunnel
- `↑/↓` or `j/k` - Navigate tunnel list
- `q` or `Ctrl+C` - Quit (with confirmation)

#### Creating a Tunnel
1. Press `n` to start
2. Select host from list or press `m` for manual entry
3. Enter remote port
4. Enter local port
5. Enter tag (or press Enter for auto-generated name)
6. Choose verbose mode (y/n)
7. Wait for connection

#### Logs Panel
- `↑/↓` - Scroll through logs
- View real-time SSH connection output

## Configuration

### SSH Config

The application reads hosts from your `~/.ssh/config` file:

```ssh
Host myserver
    HostName server.example.com
    User myuser
    IdentityFile ~/.ssh/id_rsa
```

### Manual Host Entry

If your host isn't in the config, press `m` during host selection to enter manually:
- Format: `user@hostname` or `hostname`
- Example: `ubuntu@192.168.1.100`

## Examples

### Create a tunnel to forward local port 8080 to remote port 80
1. Run `ssh-tunnel`
2. Press `n`
3. Select your server
4. Enter `80` for remote port
5. Enter `8080` for local port
6. Press Enter for auto-generated tag
7. Press `n` for no verbose logs

### Access remote database locally
1. Create tunnel: `localhost:5432` → `db.server.com:5432`
2. Connect your local client to `localhost:5432`
3. Traffic is securely tunneled through SSH

## Troubleshooting

### Port already in use
If you see "port already in use", choose a different local port or close the application using that port.

### Connection fails
- Verify SSH access: `ssh user@host`
- Check SSH config syntax
- Enable verbose mode to see detailed logs

### Tunnels not appearing
- Ensure SSH config is properly formatted
- Check file permissions on `~/.ssh/config`

## Dependencies

- [Bubbletea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Style definitions
- [Bubbles](https://github.com/charmbracelet/bubbles) - UI components
- [Moby](https://github.com/moby/moby) - Name generation

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details

## Author

**Hugo Hernández Valdez**
- Email: hugohv10@gmail.com
- Website: [hanhgouh.me](https://hanhgouh.me)

Created with ❤️ using Go and Bubbletea

## Acknowledgments

- Charm.sh for the amazing TUI libraries
- Docker for the name generation inspiration
- The Go community

