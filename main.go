package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"kanade/audio"
	"kanade/downloader"
	"kanade/library"
	"kanade/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error getting user home directory: %v\n", err)
		os.Exit(1)
	}
	logDir := homeDir + string(os.PathSeparator) + ".kanade"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("Error creating log directory: %v\n", err)
		os.Exit(1)
	}
	logFilePath := logDir + string(os.PathSeparator) + "kanade.log"
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Warning: could not open log file: %v\n", err)
	} else {
		defer logFile.Close()
		log.SetOutput(logFile)
	}

	var dir string
	if len(os.Args) > 1 {
		if os.Args[1] == "--help" || os.Args[1] == "-h" {
			fmt.Println("Usage: kanade [directory]")
			fmt.Println("If no directory is specified, it defaults to the current working directory.")
			os.Exit(0)
		}

		dir = os.Args[1]
		if err := os.Chdir(dir); err != nil {
			fmt.Printf("Error changing directory to '%s': %v\n", dir, err)
			os.Exit(1)
		}
	} else {
		currentDir, err := os.Getwd()
		if err != nil {
			fmt.Printf("Error getting current directory: %v\n", err)
			os.Exit(1)
		}
		dir = currentDir
	}

	library := &library.Library{}
	player := audio.NewPlayer()

	downloaderManager := downloader.NewManager(library, dir, 3)
	downloaderManager.Start()

	cleanup := func() {
		log.Println("Shutting down")
		downloaderManager.Stop()
		if err := player.Close(); err != nil {
			log.Printf("Error closing audio player: %v", err)
		}
		log.Println("Cleanup completed")
	}
	defer cleanup()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Received interrupt signal")
		cleanup()
		os.Exit(0)
	}()

	log.Printf("Reading songs from directory: %s", dir)
	songs, err := library.ReadDir(dir)
	if err != nil {
		fmt.Printf("Error reading directory '%s': %v\n", dir, err)
		os.Exit(1)
	}

	if len(songs) == 0 {
		fmt.Printf("No songs found in '%s'\n", dir)
		fmt.Println("Please add some .mp3 files to the directory")
		os.Exit(1)
	}

	log.Printf("Found %d songs", len(songs))

	model := tui.NewModel(library, player, downloaderManager)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	log.Println("Starting TUI application")
	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	if model, ok := finalModel.(*tui.Model); ok {
		if lastErr := model.GetLastError(); lastErr != nil {
			log.Printf("Final error state: %v", lastErr)
		}
	}

	log.Println("Application exited normally")
}
