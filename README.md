# Go-RomM-Sync

> [!IMPORTANT]
> This application is a client for synchronizing and playing games from your RomM library locally on your device. It is **not** intended for managing or adding new games to your self-hosted RomM instance. Please use the RomM web interface for administrative tasks.

Go-RomM-Sync is a desktop application built with Wails and React that synchronizes your RomM library with a local RetroArch installation. It provides a gaming-console-like experience with full controller navigation support, making it perfect for use with gamepads on a home theater setup.

## Features

- **Enhanced Metadata**: Fetches detailed game summaries, genres, and cover art directly from your RomM instance.
- **Library Management**: One-click download of ROMs to your local storage, organized by platform.
- **RetroArch Integration**: Launch games directly into RetroArch with automatic core detection and RetroAchievements support.
- **Spatial Navigation**: Full support for gamepad and keyboard navigation, with "sticky" focus that remembers your position as you browse.
- **Cross-Platform**: Designed to work on macOS, Linux, and Windows.

## Screenshots

*(Coming Soon: High-quality screenshots of the interface and navigation)*

## Technology Stack

- **Backend**: Go with [Wails v2](https://wails.io/) for native OS integration.
- **Frontend**: React + TypeScript + Vite.
- **Styling**: Vanilla CSS for a lightweight, performant UI.
- **Navigation**: Norigin Spatial Navigation for console-like input handling.

## Getting Started

### Prerequisites

- A running RomM instance with API access enabled.
- RetroArch installed on your system.

## Development

### Running Locally

To run the application in development mode with hot reloading:

```bash
wails dev
```

### Building

To build a production-ready package for your current platform:

```bash
wails build
```

#### Linux Specifics

For modern Linux distributions (like Ubuntu 22.04+), you may need to install additional dependencies and use specific build tags:

```bash
# Install dependencies (Ubuntu/Debian)
sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev

# Build with WebKit 4.1 tags
wails build -tags webkit2_41
```

## Setup and Usage

1. Build or run the application (see Development section).
2. Launch the application.
3. Enter your RomM host URL and login credentials.
4. In Settings, select your RetroArch executable.
5. Set your RetroAchievements credentials (optional).
6. Browse your collection and start playing!

## Architecture

- **Backend**: Go (Wails) for high-performance file management, RetroArch configuration, and API communication.
- **Frontend**: React with TypeScript for a responsive, high-performance UI.
- **Navigation**: Norigin Spatial Navigation for seamless gamepad support.

## Roadmap

- [ ] **Save Syncing Management**: Bidirectional synchronization of saves and states between local storage and RomM.
- [ ] **Detailed Achievements**: View RetroAchievements progress and badges directly on the game page.
- [ ] **Advanced Filtering**: Filter games by genre, platform, or download status.