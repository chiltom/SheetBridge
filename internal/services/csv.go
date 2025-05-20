package services

import (
	"encoding/csv"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chiltom/SheetBridge/internal/apperrors"
)

// MaxPreviewSize defines the number of rows to be returned for a preview
const MaxPreviewSize = 50

// Regular expression definitions for field name sanitation
var (
	nonAlphanumericRegex    = regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	leadingNumericRegex     = regexp.MustCompile(`^[0-9]`)
	multipleUnderscoreRegex = regexp.MustCompile(`_+`)
)

// CSVService represents the CSV parsing service
type CSVService struct {
	// Dependencies like logger can be added here if needed
}

// NewCSVService returns a new CSV parsing service instance
func NewCSVService() *CSVService {
	return &CSVService{}
}

// ParseUploadedCSV Parses an uploaded CSV, stores it temporarily, and returns the headers and preview rows
func (s *CSVService) ParseUploadedCSV(fileHeader *multipart.FileHeader) (headers []string, previewRows [][]string, tempFilePath string, appErr *apperrors.AppError) {
	src, err := fileHeader.Open()
	if err != nil {
		return nil, nil, "", apperrors.Wrap(err, apperrors.ErrFileOperation, "failed to open uploaded file")
	}
	defer src.Close()

	// Create a temporary file in the system's default temp directory
	// For /opt deployment, ensure this temp dir is writable by the app user
	// Or, configure a specific temp dir path.
	tempFile, err := os.CreateTemp("", "sheetbridge-upload-*.csv")
	if err != nil {
		return nil, nil, "", apperrors.Wrap(err, apperrors.ErrFileOperation, "failed to create temp file")
	}
	tempFilePath = tempFile.Name()

	tee := io.TeeReader(src, tempFile)
	csvReader := csv.NewReader(tee)

	headers, err = csvReader.Read()
	if err != nil {
		tempFile.Close()
		os.Remove(tempFilePath)
		if err == io.EOF {
			return nil, nil, "", apperrors.Wrap(err, apperrors.ErrCSVProcessing, "CSV file is empty or has no headers")
		}
		return nil, nil, "", apperrors.Wrap(err, apperrors.ErrCSVProcessing, "failed to read CSV headers")
	}

	for i := 0; i < MaxPreviewSize; i++ {
		record, readErr := csvReader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			previewRows = append(previewRows, []string{fmt.Sprintf("Error reading preview row %d: %v", i+1, readErr)})
			break
		}
		previewRows = append(previewRows, record)
	}

	// TODO: Find a suitable alternate for io.Copy to write and dump rest of CSV file
	// if _, copyErr := io.Copy(io.Discard, csvReader); copyErr != nil { // Ensure rest of file is written
	// 	tempFile.Close()
	// 	os.Remove(tempFilePath)
	// 	return nil, nil, "", apperrors.Wrap(copyErr, apperrors.ErrFileOperation, "failed to complete writing to temp file")
	// }

	if err := tempFile.Close(); err != nil {
		os.Remove(tempFilePath)
		return nil, nil, "", apperrors.Wrap(err, apperrors.ErrFileOperation, "failed to close temp file after writing")
	}

	return headers, previewRows, tempFilePath, nil
}

// ReadFullCSV reads in an entire CSV file and returns the headers and all records
func (s *CSVService) ReadFullCSV(filePath string) (headers []string, allRecords [][]string, appErr *apperrors.AppError) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, apperrors.Wrap(err, apperrors.ErrFileOperation, "failed to open CSV file for full read")
	}
	defer file.Close()

	reader := csv.NewReader(file)
	headers, err = reader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, nil, apperrors.Wrap(err, apperrors.ErrCSVProcessing, "CSV file is empty or has no headers (full read)")
		}
		return nil, nil, apperrors.Wrap(err, apperrors.ErrCSVProcessing, "failed to read CSV headers (full read)")
	}

	allRecords, err = reader.ReadAll()
	if err != nil {
		return headers, nil, apperrors.Wrap(err, apperrors.ErrCSVProcessing, "failed to read all CSV records")
	}

	return headers, allRecords, nil
}

// SanitizeSQLName ensures that field names follow standard naming conventions
func (s *CSVService) SanitizeSQLName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)

	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "/", "_")

	name = nonAlphanumericRegex.ReplaceAllString(name, "")
	name = multipleUnderscoreRegex.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")

	if name == "" {
		return "unnamed_identifier"
	}
	if leadingNumericRegex.MatchString(name) {
		name = "_" + name
	}

	if len(name) > 63 {
		name = name[:63]
		name = strings.TrimRight(name, "_")
	}
	if name == "" {
		return "sanatized_empty_identifier"
	}
	return name
}

// SanitizeTableName ensures that PostgreSQL table names follow standard naming conventions
func (s *CSVService) SanitizeTableName(filename string) string {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	sanitized := s.SanitizeSQLName(name)
	if sanitized == "unnamed_identifier" || sanitized == "sanatized_empty_identifier" || sanitized == "" {
		return "imported_table"
	}
	return sanitized
}
