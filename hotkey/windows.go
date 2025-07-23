//go:build windows

package hotkey

import (
	"log"

	"kanade/tui"

	hk "golang.design/x/hotkey"
)

const (
	KeyMediaNextTrack = hk.Key(0xB0)
	KeyMediaPrevTrack = hk.Key(0xB1)
	KeyMediaPlayPause = hk.Key(0xB3)
)

func InitMediaKeys(m *tui.Model) {
	go func() {
		hkPlayPause := hk.New([]hk.Modifier{}, KeyMediaPlayPause)
		if err := hkPlayPause.Register(); err != nil {
			log.Printf("Failed to register Play/Pause hotkey: %v", err)
		}

		hkNext := hk.New([]hk.Modifier{}, KeyMediaNextTrack)
		if err := hkNext.Register(); err != nil {
			log.Printf("Failed to register Next Track hotkey: %v", err)
		}

		hkPrev := hk.New([]hk.Modifier{}, KeyMediaPrevTrack)
		if err := hkPrev.Register(); err != nil {
			log.Printf("Failed to register Previous Track hotkey: %v", err)
		}

		log.Println("Windows media key listener started.")

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
