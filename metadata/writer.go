package metadata

import (
	"log"
	"strconv"

	id3v2 "github.com/bogem/id3v2"
)

type Picture struct {
	Ext         string
	MIMEType    string
	Type        string
	Description string
	Data        []byte
}

type Metadata struct {
	Title       string
	Artist      string
	Album       string
	Year        string
	Genre       string
	Track       int
	TotalTracks int
	AlbumArt    Picture
}

func WriteMetadata(filePath string, metadata Metadata) error {
	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
	if err != nil {
		log.Printf("Error opening file %s: %v", filePath, err)
		return err
	}
	defer tag.Close()

	if metadata.Title != "" {
		tag.SetTitle(metadata.Title)
	}
	if metadata.Artist != "" {
		tag.SetArtist(metadata.Artist)
	}
	if metadata.Album != "" {
		tag.SetAlbum(metadata.Album)
	}
	if metadata.Year != "" {
		tag.SetYear(metadata.Year)
	}
	if metadata.Genre != "" {
		tag.SetGenre(metadata.Genre)
	}

	if metadata.Track > 0 {
		trackStr := strconv.Itoa(metadata.Track)
		if metadata.TotalTracks > 0 {
			trackStr += "/" + strconv.Itoa(metadata.TotalTracks)
		}
		tag.AddTextFrame(tag.CommonID("Track number/Position in set"), tag.DefaultEncoding(), trackStr)
	}

	if len(metadata.AlbumArt.Data) > 0 {
		mimeType := metadata.AlbumArt.MIMEType
		if mimeType == "" {
			if metadata.AlbumArt.Ext != "" {
				switch metadata.AlbumArt.Ext {
				case ".jpg", ".jpeg", ".JPG", ".JPEG":
					mimeType = "image/jpeg"
				case ".png", ".PNG":
					mimeType = "image/png"
				case ".gif", ".GIF":
					mimeType = "image/gif"
				default:
					mimeType = "image/jpeg"
				}
			} else {
				mimeType = "image/jpeg"
			}
		}

		description := metadata.AlbumArt.Description
		if description == "" {
			description = "Front cover"
		}

		pic := id3v2.PictureFrame{
			Encoding:    tag.DefaultEncoding(),
			MimeType:    mimeType,
			PictureType: id3v2.PTFrontCover,
			Description: description,
			Picture:     metadata.AlbumArt.Data,
		}
		tag.AddAttachedPicture(pic)
	}

	if err = tag.Save(); err != nil {
		log.Printf("Error saving metadata to file %s: %v", filePath, err)
		return err
	}

	return nil
}
