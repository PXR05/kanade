//go:build linux

package hotkey

import (
	"crypto/md5"
	"fmt"
	"log"
	"os"
	"path/filepath"

	lib "kanade/library"
	"kanade/tui"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

const mprisPath = "/org/mpris/MediaPlayer2"
const mprisPlayerInterface = "org.mpris.MediaPlayer2.Player"
const mprisBaseInterface = "org.mpris.MediaPlayer2"

type MediaPlayer struct {
	model *tui.Model
	props *prop.Properties
}

func (p *MediaPlayer) Next() *dbus.Error {
	log.Println("D-Bus: Next called")
	p.model.ControlPlayback(tui.NextTrack)
	return nil
}

func (p *MediaPlayer) Previous() *dbus.Error {
	log.Println("D-Bus: Previous called")
	p.model.ControlPlayback(tui.PrevTrack)
	return nil
}

func (p *MediaPlayer) Play() *dbus.Error {
	log.Println("D-Bus: Play called")
	p.model.ControlPlayback(tui.Play)
	p.Update(p.model.SelectedSong, true)
	return nil
}

func (p *MediaPlayer) Pause() *dbus.Error {
	log.Println("D-Bus: Pause called")
	p.model.ControlPlayback(tui.Pause)
	p.Update(p.model.SelectedSong, false)
	return nil
}

func (p *MediaPlayer) PlayPause() *dbus.Error {
	log.Println("D-Bus: PlayPause called")
	p.model.ControlPlayback(tui.PlayPause)
	p.Update(p.model.SelectedSong, p.model.AudioPlayer.IsPlaying())
	return nil
}

func (p *MediaPlayer) Stop() *dbus.Error {
	log.Println("D-Bus: Stop called")
	p.model.ControlPlayback(tui.Stop)
	p.Update(nil, false)
	return nil
}

func (p *MediaPlayer) Update(song *lib.Song, isPlaying bool) {
	if p.props == nil {
		return
	}

	status := "Paused"
	if isPlaying {
		status = "Playing"
	}
	p.setPlaybackStatus(status)

	if song != nil {
		metadata := p.createMetadata(song)
		p.setMetadata(metadata)
	}
}

func (p *MediaPlayer) setMetadata(metadata map[string]dbus.Variant) {
	p.props.Set(mprisPlayerInterface, "Metadata", dbus.MakeVariant(metadata))
	log.Println("D-Bus: Metadata updated")
}

func (p *MediaPlayer) setPlaybackStatus(status string) {
	p.props.Set(mprisPlayerInterface, "PlaybackStatus", dbus.MakeVariant(status))
	log.Println("D-Bus: PlaybackStatus updated to", status)
}

func (p *MediaPlayer) createMetadata(song *lib.Song) map[string]dbus.Variant {
	meta := make(map[string]dbus.Variant)

	meta["mpris:trackid"] = dbus.MakeVariant(dbus.ObjectPath(song.Path))
	meta["xesam:title"] = dbus.MakeVariant(song.Title)
	meta["xesam:album"] = dbus.MakeVariant(song.Album)
	meta["xesam:artist"] = dbus.MakeVariant([]string{song.Artist})

	if song.Picture != nil && len(song.Picture.Data) > 0 {
		hash := md5.Sum(song.Picture.Data)
		filename := fmt.Sprintf("kanade-art-%x.jpg", hash)
		artPath := filepath.Join(os.TempDir(), filename)

		if _, err := os.Stat(artPath); os.IsNotExist(err) {
			if err := os.WriteFile(artPath, song.Picture.Data, 0644); err != nil {
				log.Printf("Failed to write album art to temp file: %v", err)
			}
		}
		meta["mpris:artUrl"] = dbus.MakeVariant("file://" + artPath)
	}

	return meta
}

func InitMediaKeys(m *tui.Model) {
	go func() {
		conn, err := dbus.ConnectSessionBus()
		if err != nil {
			log.Printf("Failed to connect to session bus: %v", err)
			return
		}
		defer conn.Close()

		player := &MediaPlayer{model: m}

		reply, err := conn.RequestName("org.mpris.MediaPlayer2.kanade", dbus.NameFlagDoNotQueue)
		if err != nil {
			log.Printf("Failed to request D-Bus name: %v", err)
			return
		}

		if reply != dbus.RequestNameReplyPrimaryOwner {
			log.Println("D-Bus name already taken, cannot register media player.")
			return
		}

		err = conn.Export(player, mprisPath, mprisPlayerInterface)
		if err != nil {
			log.Printf("Failed to export D-Bus methods: %v", err)
			return
		}

		propsSpec := map[string]map[string]*prop.Prop{
			mprisPlayerInterface: {
				"CanGoNext":      {Value: true, Writable: false, Emit: prop.EmitTrue},
				"CanGoPrevious":  {Value: true, Writable: false, Emit: prop.EmitTrue},
				"CanPlay":        {Value: true, Writable: false, Emit: prop.EmitTrue},
				"CanPause":       {Value: true, Writable: false, Emit: prop.EmitTrue},
				"CanSeek":        {Value: false, Writable: false, Emit: prop.EmitTrue},
				"CanControl":     {Value: true, Writable: false, Emit: prop.EmitTrue},
				"PlaybackStatus": {Value: "Stopped", Writable: false, Emit: prop.EmitTrue},
				"Metadata":       {Value: map[string]dbus.Variant{}, Writable: false, Emit: prop.EmitTrue},
			},
			"org.mpris.MediaPlayer2": {
				"CanQuit":             {Value: true, Writable: false, Emit: prop.EmitTrue},
				"CanRaise":            {Value: false, Writable: false, Emit: prop.EmitTrue},
				"Identity":            {Value: "Kanade", Writable: false, Emit: prop.EmitTrue},
				"DesktopEntry":        {Value: "kanade", Writable: false, Emit: prop.EmitTrue},
				"SupportedUriSchemes": {Value: []string{"file"}, Writable: false, Emit: prop.EmitTrue},
				"SupportedMimeTypes":  {Value: []string{"audio/mpeg", "audio/flac", "audio/x-wav"}, Writable: false, Emit: prop.EmitTrue},
			},
		}

		props, err := prop.Export(conn, mprisPath, propsSpec)
		if err != nil {
			log.Printf("Failed to export D-Bus properties: %v", err)
			return
		}
		player.props = props

		err = conn.Export(func() {
			log.Println("D-Bus: Quit called")
			m.ControlPlayback(tui.Stop)
		}, mprisPath, "org.mpris.MediaPlayer2")

		log.Println("Linux (D-Bus/MPRIS) media key handler started.")

		select {}
	}()
}
