package main

import (
	"fmt"
	"log"
	"os"

	"gmp/audio"
	lib "gmp/library"
	"gmp/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Initialize library and audio player
	library := &lib.Library{}
	player := audio.NewPlayer()
	defer player.Close()

	// Load music library
	library.ReadDir("./assets")
	songs := library.ListSongs()

	if len(songs) == 0 {
		fmt.Println("No songs found in ./assets directory")
		fmt.Println("Please add some .mp3 or .wav files to the assets folder")
		os.Exit(1)
	}

	// Create TUI model
	model := tui.NewModel(library, player)

	// Start Bubble Tea program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Run the program
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}
