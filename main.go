package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"gmp/audio"
	"gmp/downloader"
	lib "gmp/library"
	"gmp/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {

	logFile, err := os.OpenFile("gmp.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Warning: could not open log file: %v\n", err)
	} else {
		defer logFile.Close()
		log.SetOutput(logFile)
	}

	var dir string
	if len(os.Args) > 1 {
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

	library := &lib.Library{}
	player := audio.NewPlayer()

	downloadDir := filepath.Join(dir, "downloads")
	err = os.MkdirAll(downloadDir, 0755)
	if err != nil {
		fmt.Printf("Error creating download directory: %v\n", err)
		os.Exit(1)
	}

	downloaderManager := downloader.NewManager(library, downloadDir, 3)
	downloaderManager.Start()

	cleanup := func() {
		log.Println("Shutting down...")
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
