# Changelog

All notable changes to this project will be documented in this file.

## [1.1.0] - 2026-02-17

### Added

- **Help Overlay**: Press ? to see all keyboard shortcuts in a clean overlay
- **Visible Shortcuts**: Keyboard shortcuts now shown in the footer for quick reference
- **Search Functionality**: Find tunnels quickly with the new search feature
- **Toast Notifications**: Better feedback when creating or deleting tunnels

### Fixed

- Help view now closes properly with Escape key
- Improved help panel rendering and centering

### Added

- **New One Dark theme**: Interface with blue, red, green, and purple colors
- **Confirmation modals**: Now when deleting a tunnel, a confirmation modal appears
- **IP selection**: When a host has multiple IPs, you can choose which one to use
- **Mouse support**: Click on panels and tunnels to select them
- **Scroll for long lists**: Smooth navigation when there are many hosts

## [0.1.3] - 2026-02-04

### Added

- Reduce tick frequency from 250ms to 1 second
- Copy only visible logs instead of all logs
- Truncate long lines to prevent panel overflow
- Remove glamour dependency for faster text rendering


## [0.1.2] - 2026-02-04

### Added

- Multi-platform build system (Linux amd64, macOS Intel, macOS Apple Silicon)
- Build outputs to build/ directory
- Install script uses latest release automatically


## [0.1.1] - 2026-02-04

### Added

- Added version display in topbar
- Added Makefile for version management
- Improved UI styling with lipgloss


## [0.1.0] - 2026-02-04

### Added
- Initial release
- Multiple SSH tunnel management
- Beautiful Dracula-themed UI
- Real-time logs
- Auto-naming with Docker-style names
- Keyboard navigation
