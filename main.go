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
	library := &lib.Library{}
	player := audio.NewPlayer()
	defer player.Close()

	library.ReadDir("./assets")
	songs := library.ListSongs()

	if len(songs) == 0 {
		fmt.Println("No songs found in ./assets directory")
		fmt.Println("Please add some .mp3 or .wav files to the assets folder")
		os.Exit(1)
	}

	model := tui.NewModel(library, player)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}
