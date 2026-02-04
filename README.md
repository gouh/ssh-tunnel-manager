# SSH Tunnel Manager

Interactive TUI for creating SSH tunnels through VPN using Bubbletea.

## Features

- ✅ VPN status check and auto-start
- ✅ Interactive host selection from SSH config
- ✅ Port validation and availability check
- ✅ Verbose logging option
- ✅ Clean, modern TUI interface

## Installation

```bash
go build -o ssh-tunnel
sudo cp ssh-tunnel /usr/local/bin/
```

## Usage

```bash
ssh-tunnel
```

## Controls

- `↑/↓` or `j/k` - Navigate host list
- `Enter` - Confirm selection
- `y/n` - Yes/No prompts
- `q` or `Ctrl+C` - Quit

## Requirements

- Go 1.21+
- OpenVPN (for VPN functionality)
- SSH config at `~/.ssh/config`
