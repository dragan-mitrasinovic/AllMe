package download

import (
	"all-me-backend/pkg/models"
	"archive/zip"
	"fmt"
	"io"
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

	for _, file := range files {
		if err := s.addFileToZip(zipWriter, file, token); err != nil {
			// Continue with other files even if one fails
			continue
		}
	}

	return nil
}

// addFileToZip downloads a file from cloud storage and adds it to the ZIP archive
func (s *Service) addFileToZip(zipWriter *zip.Writer, file *models.CloudItem, token *models.Token) error {
	// Get file stream from cloud storage
	fileStream, err := s.storageService.GetFileStream(file, token)
	if err != nil {
		return fmt.Errorf("failed to get file stream: %w", err)
	}
	defer fileStream.Close()

	// Create a new file entry in the ZIP archive
	zipFile, err := zipWriter.Create(file.Name)
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
