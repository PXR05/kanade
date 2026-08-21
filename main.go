package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"kanade/audio"
	"kanade/hotkey"
	"kanade/library"
	"kanade/tui"

	tea "github.com/charmbracelet/bubbletea"
)

const usage = `Kanade — a minimal terminal music player.

Usage:
  kanade [directory]

Arguments:
  directory    Folder containing your music (default: current directory)

Supported formats: .mp3 .wav .flac .ogg

Once running, press ? for keybindings or :help for commands.`

func main() {
	dir, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if dir == "" {
		os.Exit(0)
	}

	setupLogging()

	player := audio.NewPlayer()
	defer func() {
		if err := player.Close(); err != nil {
			log.Printf("Error closing audio player: %v", err)
		}
	}()

	log.Printf("Reading songs from directory: %s", dir)
	lib := &library.Library{}
	songs, err := lib.ReadDir(dir)
	if err != nil {
		fmt.Printf("Error reading directory '%s': %v\n", dir, err)
		os.Exit(1)
	}

	if len(songs) == 0 {
		fmt.Printf("No supported audio files found in '%s'.\n%s\n", dir, usage)
		os.Exit(1)
	}
	log.Printf("Found %d songs", len(songs))

	model := tui.NewModel(lib, player, dir)

	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
	)
	model.SetProgram(program.Send)

	hotkey.InitMediaKeys(model)

	log.Println("Starting TUI application")
	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running application: %v\n", err)
		os.Exit(1)
	}

	if lastErr := model.GetLastError(); lastErr != nil {
		log.Printf("Final error state: %v", lastErr)
	}
	log.Println("Application exited normally")
}

func parseArgs(args []string) (string, error) {
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			fmt.Println(usage)
			return "", nil
		default:
			abs, err := filepath.Abs(arg)
			if err != nil {
				return "", fmt.Errorf("invalid directory '%s': %w", arg, err)
			}
			return abs, nil
		}
	}
	return os.Getwd()
}

func setupLogging() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Warning: could not determine home directory for logs: %v\n", err)
		return
	}

	logDir := filepath.Join(homeDir, ".kanade")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		fmt.Printf("Warning: could not create log directory: %v\n", err)
		return
	}

	logFile, err := os.OpenFile(
		filepath.Join(logDir, "kanade.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0o666,
	)
	if err != nil {
		fmt.Printf("Warning: could not open log file: %v\n", err)
		return
	}

	log.SetOutput(logFile)
}
