package download

import (
	"all-me-backend/pkg/models"
	"archive/zip"
	"fmt"
	"io"
	"path"
	"strings"
)

type Service struct {
	storageService StorageService
}

func NewService(storageService StorageService) *Service {
	return &Service{
		storageService: storageService,
	}
}

// StreamZipArchive streams multiple files into a ZIP archive directly to the writer
// It downloads files from cloud storage and adds them to the ZIP without temporary storage
func (s *Service) StreamZipArchive(writer io.Writer, files []*models.CloudItem, token *models.Token) error {
	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	usedNames := make(map[string]bool)
	for _, file := range files {
		if err := s.addFileToZip(zipWriter, file, safeZipName(file.Name, usedNames), token); err != nil {
			// Continue with other files even if one fails
			continue
		}
	}

	return nil
}

// addFileToZip downloads a file from cloud storage and adds it to the ZIP archive
func (s *Service) addFileToZip(zipWriter *zip.Writer, file *models.CloudItem, entryName string, token *models.Token) error {
	// Get file stream from cloud storage
	fileStream, err := s.storageService.GetFileStream(file, token)
	if err != nil {
		return fmt.Errorf("failed to get file stream: %w", err)
	}
	defer fileStream.Close()

	// Create a new file entry in the ZIP archive
	zipFile, err := zipWriter.Create(entryName)
	if err != nil {
		return fmt.Errorf("failed to create ZIP entry: %w", err)
	}

	// Copy the file content to the ZIP archive
	_, err = io.Copy(zipFile, fileStream)
	if err != nil {
		return fmt.Errorf("failed to write file to ZIP: %w", err)
	}

	return nil
}

// safeZipName turns a file name from the request into a flat ZIP entry name: no directories, so nothing
// can be extracted outside the target folder, and duplicates get a " (n)" suffix instead of overwriting.
func safeZipName(name string, used map[string]bool) string {
	base := path.Base(strings.ReplaceAll(name, "\\", "/"))
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == ".." || base == "/" {
		base = "photo"
	}

	candidate := base
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 2; used[strings.ToLower(candidate)]; i++ {
		candidate = fmt.Sprintf("%s (%d)%s", stem, i, ext)
	}
	used[strings.ToLower(candidate)] = true
	return candidate
}
