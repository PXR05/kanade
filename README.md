# 🎵 GMP - Go Music Player

A beautiful terminal-based music player built with Go, featuring a modern Terminal UI powered by Bubble Tea.

## ✨ Features

### 🎼 Core Functionality

- **Audio Playback**: Support for MP3 and WAV files
- **Library Management**: Automatic discovery and organization of music files
- **Metadata Extraction**: Displays artist, title, album, genre, and embedded album art
- **Playback Controls**: Play, pause, stop, seek, and position tracking

### 🖥️ Terminal UI

- **Modern Interface**: Built with Bubble Tea and styled with Lip Gloss
- **Two-View Design**:
  - **Library View**: Browse and select songs with keyboard navigation
  - **Player View**: Rich playback interface with album art and controls
- **Album Art Display**: ASCII art rendering of embedded album artwork
- **Responsive Design**: Adapts to different terminal sizes
- **Real-time Updates**: Live progress tracking and status updates

### 🎨 Visual Design

- **Styled Components**: Beautiful borders, colors, and typography
- **Progress Bars**: Visual representation of playback progress
- **Keyboard Shortcuts**: Intuitive navigation and controls
- **Error Handling**: User-friendly error messages and graceful fallbacks

## 🚀 Installation

### Prerequisites

- Go 1.19 or later
- Audio files in MP3 or WAV format

### Building from Source

```bash
git clone <repository-url>
cd gmp
go mod tidy
go build .
```

## 📁 Setup

1. Create an `assets` directory in the project root:

```bash
mkdir assets
```

2. Add your music files to the `assets` directory:

```bash
cp /path/to/your/music/*.mp3 assets/
cp /path/to/your/music/*.wav assets/
```

## 🎮 Usage

### Starting the Player

```bash
./gmp
```

### Library View Controls

| Key               | Action                       |
| ----------------- | ---------------------------- |
| `↑` / `k`         | Move selection up            |
| `↓` / `j`         | Move selection down          |
| `Enter` / `Space` | Select song and go to player |
| `Home` / `g`      | Go to first song             |
| `End` / `G`       | Go to last song              |
| `q`               | Quit application             |

### Player View Controls

| Key           | Action                   |
| ------------- | ------------------------ |
| `Space` / `p` | Play/Pause toggle        |
| `s`           | Stop playback            |
| `←` / `h`     | Seek backward 10 seconds |
| `→` / `l`     | Seek forward 10 seconds  |
| `0`           | Seek to beginning        |
| `Esc` / `q`   | Back to library view     |

## 🏗️ Architecture

### Project Structure

```
gmp/
├── audio/           # Audio playback engine
│   └── player.go   # Audio player implementation
├── library/         # Music library management
│   └── manager.go  # Library and metadata handling
├── tui/            # Terminal UI components
│   ├── model.go    # Main TUI model and state management
│   ├── libraryview.go  # Library browsing interface
│   ├── playerview.go   # Playback interface
│   └── albumart.go     # Album art ASCII rendering
├── metadata/       # Metadata extraction utilities
├── assets/         # Music files directory
└── main.go        # Application entry point
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.
