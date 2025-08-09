//go:build darwin

package hotkey

import (
	"log"

	"kanade/tui"

	hk "golang.design/x/hotkey"
)

const (
	keyF7 = hk.Key(98)
	keyF8 = hk.Key(100)
	keyF9 = hk.Key(101)
)

func InitMediaKeys(m *tui.Model) {
	go func() {
		hkPlayPause := hk.New([]hk.Modifier{}, keyF8)
		if err := hkPlayPause.Register(); err != nil {
			log.Printf("Failed to register Play/Pause hotkey (F8): %v", err)
		}

		hkNext := hk.New([]hk.Modifier{}, keyF9)
		if err := hkNext.Register(); err != nil {
			log.Printf("Failed to register Next Track hotkey (F9): %v", err)
		}

		hkPrev := hk.New([]hk.Modifier{}, keyF7)
		if err := hkPrev.Register(); err != nil {
			log.Printf("Failed to register Previous Track hotkey (F7): %v", err)
		}

		log.Println("macOS media key listener started (F7/F8/F9).")

		for {
			select {
			case <-hkPlayPause.Keydown():
				m.ControlPlayback(tui.PlayPause)
			case <-hkNext.Keydown():
				m.ControlPlayback(tui.NextTrack)
			case <-hkPrev.Keydown():
				m.ControlPlayback(tui.PrevTrack)
			}
		}
	}()
}
