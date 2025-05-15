package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/chiltom/SheetBridge/internal/models"
	"github.com/chiltom/SheetBridge/internal/repositories"
)

// The Service interface defines the business logic for CSV ingestion
type Service interface {
	ParseCSVForPreview(r io.Reader, maxRows int) (models.CSVData, error)
	CommitCSV(ctx context.Context, req models.CommitRequest, csvFile io.ReadSeeker) error
	GetAllTables(ctx context.Context) ([]string, error)
}

// The service struct uses the application Repository to implement the Service interface
type service struct {
	repo repositories.Repository
}

// NewService returns a new service
func NewService(repo repositories.Repository) Service {
	return &service{repo: repo}
}

// ParseCSVForPreview parses a CSV file for header inference and data preview
func (s *service) ParseCSVForPreview(r io.Reader, maxRows int) (models.CSVData, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // Allow variable fields, though we validate later
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return models.CSVData{}, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	var previewRows [][]string
	if maxRows > 0 {
		for i := 0; i < maxRows; i++ {
			row, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				if perr, ok := err.(*csv.ParseError); ok {
					return models.CSVData{}, fmt.Errorf("parse error at line %d, column %d previewing CSV: %w", perr.Line, perr.Column, perr.Err)
				}
				return models.CSVData{}, fmt.Errorf("error reading row %d for preview: %w", i+1, err)
			}
			// Ensure row has same number of columns as headers for preview consistency
			if len(row) != len(headers) {
				// Pad or truncate if necessary, or error. For preview, padding is acceptable
				fixedRow := make([]string, len(headers))
				copy(fixedRow, row)
				previewRows = append(previewRows, fixedRow)
			} else {
				previewRows = append(previewRows, row)
			}
		}
	}

	colInfos := s.inferAndMapDataTypes(headers, previewRows) // Infer based on preview rows
	return models.CSVData{Headers: colInfos, Rows: previewRows}, nil
}

// inferAndMapDataTypes infers data types from sample rows and maps them to Postgres data types
// It also sanitizes header names.
func (s *service) inferAndMapDataTypes(headers []string, sampleRows [][]string) []models.ColumnInfo {
	colInfos := make([]models.ColumnInfo, len(headers))

	for i, headerName := range headers {
		sanitizedName := sanitizeColumnName(headerName)
		inferredType := "TEXT" // Default type

		if len(sampleRows) > 0 {
			isPotentiallyBool := true
			isPotentiallyInt := true
			isPotentiallyFloat := true
			isPotentiallyDate := true

			for _, row := range sampleRows {
				if len(row) <= i {
					continue
				} // Skip if row is shorter than headers
				val := strings.TrimSpace(row[i])
				if val == "" {
					continue
				}

				if isPotentiallyBool {
					lVal := strings.ToLower(val)
					if !(lVal == "true" || lVal == "false" || lVal == "t" || lVal == "f") {
						isPotentiallyBool = false
					}
				}
				if isPotentiallyInt {
					if _, err := strconv.ParseInt(val, 10, 64); err != nil {
						isPotentiallyInt = false
					}
				}
				if isPotentiallyFloat {
					if _, err := strconv.ParseFloat(val, 64); err != nil {
						isPotentiallyFloat = false
					}
				}
				if isPotentiallyDate {
					formats := []string{
						"2006-01-02",
						"1/2/2006",
						"2006/01/02",
						"01-02-2006",
						"2025-01-02 12:00:00",
						"2006-01-02T15:04:05Z07:00",
						time.RFC3339,
					}
					parsed := false
					for _, format := range formats {
						if _, err := time.Parse(format, val); err == nil {
							parsed = true
							break
						}
					}
					if !parsed {
						isPotentiallyDate = false
					}
				}
			}
			// Determine the most specific type
			if isPotentiallyBool {
				inferredType = "BOOLEAN"
			} else if isPotentiallyInt {
				inferredType = "INTEGER"
			} else if isPotentiallyFloat {
				inferredType = "FLOAT"
			} else if isPotentiallyDate {
				inferredType = "DATE"
			}
		}

		colInfos[i] = models.ColumnInfo{
			Name:     sanitizedName,
			DataType: mapInferredTypeToPostgres(inferredType), // Map to pg type
		}
	}
	return colInfos
}

// CommitCSV makes the specified transaction with the database
func (s *service) CommitCSV(ctx context.Context, req models.CommitRequest, csvFile io.ReadSeeker) error {
	if _, err := csvFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to reset CSV reader for schema parsing: %w", err)
	}

	parsedDataForSchema, err := s.ParseCSVForPreview(csvFile, 50)
	if err != nil {
		return fmt.Errorf("failed to parse CSV for schema determination: %w", err)
	}
	dbColInfo := parsedDataForSchema.Headers // These are already ColumnInfo with pg types

	switch req.Action {
	case "create":
		if err := s.repo.CreateTable(ctx, req.TableName, dbColInfo); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	case "overwrite":
		if err := s.repo.ValidateTable(ctx, req.TableName, dbColInfo); err != nil {
			return fmt.Errorf("table validation failed for overwrite: %w", err)
		}
		if err := s.repo.TruncateTable(ctx, req.TableName); err != nil {
			return fmt.Errorf("failed to truncate table for overwrite: %w", err)
		}
	case "append":
		if err := s.repo.ValidateTable(ctx, req.TableName, dbColInfo); err != nil {
			return fmt.Errorf("table validation failed for append: %w", err)
		}
	default:
		return fmt.Errorf("invalid action: %s", req.Action)
	}

	if _, err := csvFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to reset CSV reader for data insertion: %w", err)
	}

	if err := s.repo.InsertData(ctx, req.TableName, dbColInfo, csvFile); err != nil {
		return fmt.Errorf("failed to insert data: %w", err)
	}
	return nil
}

func (s *service) GetAllTables(ctx context.Context) ([]string, error) {
	tables, err := s.repo.GetAllTables(ctx)
	if err != nil {
		return nil, err
	}

	if tables == nil {
		return make([]string, 0), nil
	}
	return tables, nil
}

// inferDataTypes infers the data types of CSV columns
// It prioritizes more specific types (INTEGER, FLOAT, BOOLEAN, DATE) over TEXT.
func inferDataTypes(headers []string, rows [][]string) []models.ColumnInfo {
	colTypes := make([]models.ColumnInfo, len(headers))
	for i := range headers {
		colTypes[i] = models.ColumnInfo{Name: sanitizeColumnName(headers[i]), DataType: "TEXT"}
	}

	for _, row := range rows {
		if len(row) != len(headers) {
			continue
		}
		for i, val := range row {
			if val == "" {
				continue
			}
			currentType := colTypes[i].DataType
			switch currentType {
			case "TEXT":
				if _, err := strconv.ParseInt(val, 10, 64); err == nil {
					colTypes[i].DataType = "INTEGER"
				} else if _, err := strconv.ParseFloat(val, 64); err == nil {
					colTypes[i].DataType = "FLOAT"
				} else if _, err := time.Parse("2006-01-02", val); err == nil {
					colTypes[i].DataType = "DATE"
				} else if strings.ToLower(val) == "true" || strings.ToLower(val) == "false" {
					colTypes[i].DataType = "BOOLEAN"
				}
			case "INTEGER":
				if _, err := strconv.ParseInt(val, 10, 64); err != nil {
					if _, err := strconv.ParseFloat(val, 64); err == nil {
						colTypes[i].DataType = "FLOAT"
					} else {
						colTypes[i].DataType = "TEXT"
					}
				}
			case "FLOAT":
				if _, err := strconv.ParseFloat(val, 64); err != nil {
					colTypes[i].DataType = "TEXT"
				}
			case "DATE":
				if _, err := time.Parse("2006-01-02", val); err != nil {
					colTypes[i].DataType = "TEXT"
				}
			case "BOOLEAN":
				if strings.ToLower(val) != "true" && strings.ToLower(val) != "false" {
					colTypes[i].DataType = "TEXT"
				}
			}
		}
	}
	return colTypes
}

// sanitizeColumnName ensures that column names are lower- and snake-case
func sanitizeColumnName(name string) string {
	name = strings.TrimSpace(name)
	var sb strings.Builder
	prevWasUnderscore := true // Treat beginning as if preceded by an underscore

	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			prevWasUnderscore = false
		} else if r >= 'A' && r <= 'Z' { // Convert to lowercase, add underscore if camelCase
			if !prevWasUnderscore && sb.Len() > 0 {
				sb.WriteRune('_')
			}
			sb.WriteRune(unicode.ToLower(r))
			prevWasUnderscore = false
		} else if r == ' ' || r == '-' || r == '.' || r == '/' {
			if !prevWasUnderscore && sb.Len() > 0 {
				sb.WriteRune('_')
				prevWasUnderscore = true
			}
		}
		// Other characters are ignored
	}

	finalName := strings.Trim(sb.String(), "_")
	if finalName == "" {
		return "unnamed_column"
	}
	// Ensure name doesn't start with a digit
	if unicode.IsDigit(rune(finalName[0])) {
		finalName = "_" + finalName
	}
	return finalName
}

// mapInferredTypeToPostgres maps the inferred data type to a PostgreSQL data type
func mapInferredTypeToPostgres(inferredType string) string {
	switch inferredType {
	case "INTEGER":
		return "INT" // Use INT for general integers
	case "FLOAT":
		return "FLOAT8" // Use FLOAT8 for double precision floating-point numbers
	case "BOOLEAN":
		return "BOOLEAN"
	case "DATE":
		return "DATE"
	case "TEXT":
		return "TEXT"
	default:
		return "TEXT"
	}
}
