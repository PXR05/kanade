package library

import (
	"gmp/metadata"
	"os"

	"github.com/dhowden/tag"
)

type Song struct {
	Title   string
	Artist  string
	Genre   string
	Album   string
	Picture *tag.Picture
	Path    string
}

type Library struct {
	Songs []Song
}

func (l *Library) AddSong(song Song) {
	l.Songs = append(l.Songs, song)
}

func (l *Library) RemoveSong(songPath string) {
	for i, song := range l.Songs {
		if song.Path == songPath {
			l.Songs = append(l.Songs[:i], l.Songs[i+1:]...)
			break
		}
	}
}

func (l *Library) GetSong(songPath string) *Song {
	for _, song := range l.Songs {
		if song.Path == songPath {
			return &song
		}
	}
	return nil
}

func (l *Library) ListSongs() []Song {
	return l.Songs
}

func (l *Library) Clear() {
	l.Songs = []Song{}
}

func (l *Library) Count() int {
	return len(l.Songs)
}

func (l *Library) Contains(songPath string) bool {
	for _, song := range l.Songs {
		if song.Path == songPath {
			return true
		}
	}
	return false
}

func (l *Library) UpdateSong(songPath string, updatedSong Song) {
	for i, song := range l.Songs {
		if song.Path == songPath {
			l.Songs[i] = updatedSong
			return
		}
	}
}

func (l *Library) FindByTitle(title string) []Song {
	var results []Song
	for _, song := range l.Songs {
		if song.Title == title {
			results = append(results, song)
		}
	}
	return results
}

func (l *Library) FindByArtist(artist string) []Song {
	var results []Song
	for _, song := range l.Songs {
		if song.Artist == artist {
			results = append(results, song)
		}
	}
	return results
}

func (l *Library) FindByAlbum(album string) []Song {
	var results []Song
	for _, song := range l.Songs {
		if song.Album == album {
			results = append(results, song)
		}
	}
	return results
}

func (l *Library) FindByGenre(genre string) []Song {
	var results []Song
	for _, song := range l.Songs {
		if song.Genre == genre {
			results = append(results, song)
		}
	}
	return results
}

func (l *Library) FindByPath(path string) []Song {
	var results []Song
	for _, song := range l.Songs {
		if song.Path == path {
			results = append(results, song)
		}
	}
	return results
}

func (l *Library) ReadDir(dir string) ([]Song, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var songs []Song
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := dir + "/" + file.Name()
		meta, err := metadata.ExtractMetadata(filePath)
		if err != nil {
			return nil, err
		}

		song := Song{
			Title:   meta.Title(),
			Artist:  meta.Artist(),
			Genre:   meta.Genre(),
			Album:   meta.Album(),
			Picture: meta.Picture(),
			Path:    filePath,
		}
		songs = append(songs, song)
	}
	l.Songs = append(l.Songs, songs...)

	return songs, nil
}
