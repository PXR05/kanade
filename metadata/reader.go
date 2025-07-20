package metadata

import (
	"os"

	tag "github.com/dhowden/tag"
)

func ExtractMetadata(filePath string) (tag.Metadata, error) {

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	metadata, err := tag.ReadFrom(file)
	if err != nil {
		return nil, err
	}

	return metadata, nil
}
