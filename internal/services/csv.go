package services

import (
	"encoding/csv"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chiltom/SheetBridge/internal/apperrors"
	"github.com/chiltom/SheetBridge/internal/models"
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

func (s *CSVService) InferSchemaFromPreview(headers []string, previewRows [][]string) []models.ColumnDefinition {
	numCols := len(headers)
	if numCols == 0 {
		return []models.ColumnDefinition{}
	}

	columnDefinitions := make([]models.ColumnDefinition, numCols)

	for colIdx, headerName := range headers {
		// For each column, try to determine its type by inspecting its values in the previewRows
		isStillBoolean := true
		isStillInteger := true
		isStillReal := true
		isStillDate := true
		isStillTimestamp := true

		hasAtLeastOneNonEmptyValueInColumn := false

		for _, row := range previewRows {
			if colIdx >= len(row) {
				// This column value is missing for this row, doesn't help inference much
				// but doesn't necessarily invalidate types unless all are missing.
				continue
			}
			valStr := strings.TrimSpace(row[colIdx])

			if valStr == "" {
				continue // Empty strings are compatible with any type for inference purposes
			}
			hasAtLeastOneNonEmptyValueInColumn = true

			// Check Boolean: "true", "false", "t", "f", "yes", "no", "1", "0"
			if isStillBoolean {
				lcVal := strings.ToLower(valStr)
				if !(lcVal == "true" || lcVal == "false" || lcVal == "t" || lcVal == "f" ||
					lcVal == "yes" || lcVal == "no" || lcVal == "0" || lcVal == "1") {
					isStillBoolean = false
				}
			}

			// Check Integer (standard int64)
			if isStillInteger {
				if _, err := strconv.ParseInt(valStr, 10, 64); err != nil {
					isStillInteger = false
				}
			}

			// Check Real (float64) - this will also parse integers
			if isStillReal {
				if _, err := strconv.ParseFloat(valStr, 64); err != nil {
					isStillReal = false
				}
			}

			// Check Timestamp (more specific than Date)
			// Common formats for inference
			tsLayouts := []string{
				time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02T15:04:05Z07:00",
				"01/02/2006 15:04:05", "1/2/2006 15:04:05", "Jan 2, 2006 3:04:05 PM", "2006-01-02 15:04",
				"01/02/2006 12:00",
			}
			if isStillTimestamp {
				parsed := false
				for _, layout := range tsLayouts {
					if _, err := time.Parse(layout, valStr); err == nil {
						parsed = true
						break
					}
				}
				if !parsed {
					isStillTimestamp = false
				}
			}

			// Check Date (less specific than Timestamp)
			dateLayouts := []string{
				"2006-01-02", "01/02/2006", "1/2/2006", "2006/01/02", "Jan 2, 2006", "2-Jan-2006",
			}
			if isStillDate {
				parsed := false
				for _, layout := range dateLayouts {
					if _, err := time.Parse(layout, valStr); err == nil {
						parsed = true
						break
					}
				}
				if !parsed {
					isStillDate = false
				}
			}
		} // End of row loop for a column

		// Determine final inferred type based on what's still true
		// Order of preference: BOOLEAN, INTEGER, REAL, TIMESTAMP, DATE, TEXT
		inferredAppType := "TEXT" // Default if nothing more specific matches or no non-empty values
		if hasAtLeastOneNonEmptyValueInColumn {
			if isStillBoolean {
				inferredAppType = "BOOLEAN"
			} else if isStillInteger { // If it's an integer, it's also a real number. Prioritize INTEGER.
				inferredAppType = "INTEGER" // Or BIGINT if you differentiate and isStillBigInt is true
			} else if isStillReal {
				inferredAppType = "REAL" // Could be NUMERIC or REAL depending on your app types
			} else if isStillTimestamp { // Timestamp is more specific than Date
				inferredAppType = "TIMESTAMP"
			} else if isStillDate {
				inferredAppType = "DATE"
			}
		}

		columnDefinitions[colIdx] = models.ColumnDefinition{
			Name: headerName, // Use the original CSV header name
			Type: inferredAppType,
		}
	}

	return columnDefinitions
}
