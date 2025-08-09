//go:build linux

package hotkey

import (
	"crypto/md5"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	lib "kanade/library"
	"kanade/tui"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

const mprisPath = "/org/mpris/MediaPlayer2"
const mprisPlayerInterface = "org.mpris.MediaPlayer2.Player"
const mprisBaseInterface = "org.mpris.MediaPlayer2"

type MediaPlayer struct {
	model       *tui.Model
	props       *prop.Properties
	lastTrackID string
}

type mprisRoot struct{ model *tui.Model }

func (r *mprisRoot) Raise() *dbus.Error { return nil }

func (r *mprisRoot) Quit() *dbus.Error {
	log.Println("D-Bus: Quit called")
	r.model.ControlPlayback(tui.Stop)
	return nil
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

	status := "Stopped"
	if song != nil {
		if isPlaying {
			status = "Playing"
		} else {
			status = "Paused"
		}
	}
	p.setPlaybackStatus(status)

	if song != nil {
		metadata, trackID := p.createMetadata(song)
		if trackID != p.lastTrackID {
			p.lastTrackID = trackID
			p.setMetadata(metadata)
		}
	}

	if p.model != nil && p.model.AudioPlayer != nil {
		pos := p.model.AudioPlayer.GetPlaybackPosition().Microseconds()
		if err := p.props.Set(mprisPlayerInterface, "Position", dbus.MakeVariant(int64(pos))); err != nil {
			log.Printf("D-Bus: failed updating Position: %v", err)
		}
	}
}

func (p *MediaPlayer) setMetadata(metadata map[string]dbus.Variant) {
	if err := p.props.Set(mprisPlayerInterface, "Metadata", dbus.MakeVariant(metadata)); err != nil {
		log.Printf("D-Bus: failed updating Metadata: %v", err)
		return
	}
	log.Println("D-Bus: Metadata updated")
}

func (p *MediaPlayer) setPlaybackStatus(status string) {
	if err := p.props.Set(mprisPlayerInterface, "PlaybackStatus", dbus.MakeVariant(status)); err != nil {
		log.Printf("D-Bus: failed updating PlaybackStatus: %v", err)
		return
	}
	log.Println("D-Bus: PlaybackStatus updated to", status)
}

func (p *MediaPlayer) createMetadata(song *lib.Song) (map[string]dbus.Variant, string) {
	meta := make(map[string]dbus.Variant)

	sum := md5.Sum([]byte(song.Path))
	trackID := fmt.Sprintf("/org/mpris/MediaPlayer2/kanade/track/%x", sum)
	meta["mpris:trackid"] = dbus.MakeVariant(dbus.ObjectPath(trackID))

	meta["xesam:title"] = dbus.MakeVariant(song.Title)
	meta["xesam:album"] = dbus.MakeVariant(song.Album)
	meta["xesam:artist"] = dbus.MakeVariant([]string{song.Artist})
	meta["xesam:url"] = dbus.MakeVariant("file://" + song.Path)

	if p.model != nil && p.model.AudioPlayer != nil {
		lengthUS := p.model.AudioPlayer.GetTotalLength().Microseconds()
		if lengthUS > 0 {
			meta["mpris:length"] = dbus.MakeVariant(int64(lengthUS))
		}
	}

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

	return meta, trackID
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
				"Position":       {Value: int64(0), Writable: false, Emit: prop.EmitTrue},
			},
			"org.mpris.MediaPlayer2": {
				"CanQuit":             {Value: true, Writable: false, Emit: prop.EmitTrue},
				"CanRaise":            {Value: false, Writable: false, Emit: prop.EmitTrue},
				"HasTrackList":        {Value: false, Writable: false, Emit: prop.EmitTrue},
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

		if err := conn.Export(&mprisRoot{model: m}, mprisPath, "org.mpris.MediaPlayer2"); err != nil {
			log.Printf("Failed to export base interface: %v", err)
			return
		}

		log.Println("Linux (D-Bus/MPRIS) media key handler started.")

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		player.Update(m.SelectedSong, m.AudioPlayer != nil && m.AudioPlayer.IsPlaying())

		for {
			select {
			case <-ticker.C:
				player.Update(m.SelectedSong, m.AudioPlayer != nil && m.AudioPlayer.IsPlaying())
			}
		}
	}()
}
