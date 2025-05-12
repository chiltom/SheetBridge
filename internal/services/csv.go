package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/chiltom/SheetBridge/internal/models"
	"github.com/chiltom/SheetBridge/internal/repositories"
)

// The Service interface defines the business logic for CSV ingestion
type Service interface {
	ParseCSV(r io.Reader, maxRows int) (models.CSVData, error)
	CommitCSV(ctx context.Context, req models.CommitRequest, r io.Reader) error
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

// ParseCSV parses a CSV file and returns its column types and data for review
func (s *service) ParseCSV(r io.Reader, maxRows int) (models.CSVData, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1

	headers, err := reader.Read()
	if err != nil {
		return models.CSVData{}, fmt.Errorf("failed to read headers: %w", err)
	}

	var rows [][]string
	for i := 0; i < maxRows || maxRows < 0; i++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return models.CSVData{}, fmt.Errorf("failed to read row %d: %w", i+1, err)
		}
		if len(row) != len(headers) {
			return models.CSVData{}, fmt.Errorf("row %d has %d fields, expected %d", i+1, len(row), len(headers))
		}
		rows = append(rows, row)
	}

	colTypes := inferDataTypes(headers, rows)
	return models.CSVData{Headers: colTypes, Rows: rows}, nil
}

// CommitCSV makes the specified transaction with the database
func (s *service) CommitCSV(ctx context.Context, req models.CommitRequest, r io.Reader) error {
	data, err := s.ParseCSV(r, -1)
	if err != nil {
		return fmt.Errorf("failed to parse CSV: %w", err)
	}

	switch req.Action {
	case "create":
		if err := s.repo.CreateTable(ctx, req.TableName, data.Headers); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	case "overwrite", "append":
		if err := s.repo.ValidateTable(ctx, req.TableName, data.Headers); err != nil {
			return fmt.Errorf("table validation failed: %w", err)
		}
		if req.Action == "overwrite" {
			if err := s.repo.TruncateTable(ctx, req.TableName); err != nil {
				return fmt.Errorf("failed to truncate table: %w", err)
			}
		}
	default:
		return fmt.Errorf("invalid action: %s", req.Action)
	}

	headerNames := make([]string, len(data.Headers))
	for i, h := range data.Headers {
		headerNames[i] = h.Name
	}
	if err := s.repo.InsertData(ctx, req.TableName, headerNames, data.Rows); err != nil {
		return fmt.Errorf("failed to insert data: %w", err)
	}
	return nil
}

func (s *service) GetAllTables(ctx context.Context) ([]string, error) {
	return s.repo.GetAllTables(ctx)
}

// inferDataTypes infers the data types of CSV columns
func inferDataTypes(headers []string, rows [][]string) []models.ColumnInfo {
	colTypes := make([]models.ColumnInfo, len(headers))
	for i := range headers {
		colTypes[i] = models.ColumnInfo{Name: sanitizeColumnName(headers[i]), DataType: "TEXT"}
	}

	for _, row := range rows {
		for i, val := range row {
			if val == "" {
				continue
			}
			currentType := colTypes[i].DataType
			if currentType == "TEXT" {
				if _, err := strconv.Atoi(val); err == nil {
					colTypes[i].DataType = "INTEGER"
				} else if _, err := strconv.ParseFloat(val, 64); err == nil {
					colTypes[i].DataType = "FLOAT"
				} else if _, err := time.Parse("2006-01-02", val); err == nil {
					colTypes[i].DataType = "DATE"
				} else if strings.ToLower(val) == "true" || strings.ToLower(val) == "false" {
					colTypes[i].DataType = "BOOLEAN"
				}
			}
		}
	}
	return colTypes
}

// sanitizeColumnName ensures that column names are lower- and snake-case
func sanitizeColumnName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ToLower(name)
	if name == "" {
		return "column"
	}
	return name
}
