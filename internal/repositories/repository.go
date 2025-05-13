package repositories

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/chiltom/SheetBridge/internal/models"
	"github.com/chiltom/SheetBridge/internal/utils"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Repository manages all of the database operations for the application
type Repository interface {
	GetAllTables(ctx context.Context) ([]string, error)
	ValidateTable(ctx context.Context, tableName string, headers []models.ColumnInfo) error
	CreateTable(ctx context.Context, tableName string, headers []models.ColumnInfo) error
	TruncateTable(ctx context.Context, tableName string) error
	InsertData(ctx context.Context, tableName string, headers []string, rows [][]string) error
	Close()
}

// repository implements the Repository interface
type repository struct {
	db *sqlx.DB
}

// NewRepository creates a new repository for database operations
func NewRepository(ctx context.Context, cfg utils.DBConfig) (Repository, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name)
	db, err := sqlx.ConnectContext(ctx, "postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	// Might want to add db.SetMaxOpenConns and db.SetMaxIdleConns here later
	return &repository{db: db}, nil
}

// Close closes the database connection
func (r *repository) Close() {
	if r.db != nil {
		r.db.Close()
	}
}

// GetAllTables retrieves all table names
func (r *repository) GetAllTables(ctx context.Context) ([]string, error) {
	var tables []string
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		ORDER BY table_name
	`

	err := r.db.SelectContext(ctx, &tables, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	return tables, nil
}

// ValidateTable validates that a table's schema matches the CSV headers
func (r *repository) ValidateTable(ctx context.Context, tableName string, csvHeaders []models.ColumnInfo) error {
	dbCols := []struct {
		ColumnName string `db:"column_name"`
		DataType   string `db:"data_type"`
	}{}
	query := `
		SELECT column_name, data_type
		FROM information_schema.columns
		WHERE table_name = $1
		ORDER BY ordinal_position
	`

	err := r.db.SelectContext(ctx, &dbCols, query, tableName)
	if err != nil {
		return fmt.Errorf("failed to query columns for table %s: %w", tableName, err)
	}
	if len(dbCols) == 0 {
		return fmt.Errorf("table '%s' does not exist or has no columns", tableName)
	}

	dbColMap := make(map[string]string)
	for _, col := range dbCols {
		dbColMap[col.ColumnName] = col.DataType
	}

	for _, csvCol := range csvHeaders {
		dbType, exists := dbColMap[csvCol.Name]
		if !exists {
			return fmt.Errorf("column '%s' from CSV not found in table %s", csvCol.Name, tableName)
		}

		csvType := csvCol.DataType
		if !isPostgresTypeCompatible(csvType, dbType) {
			return fmt.Errorf("type mismatch for column '%s': CSV type '%s' is not compatible with DB type '%s' in table '%s'",
				csvCol.Name, csvType, dbType, tableName)
		}
	}

	if len(csvHeaders) != len(dbCols) {
		return fmt.Errorf("column count mismatch: CSV has %d columns, table '%s' has %d columns",
			len(csvHeaders), tableName, len(dbCols))
	}

	return nil
}

// isPostgresTypeCompatible checks if a CSV-inferred PostgreSQL type is compatible with db
func isPostgresTypeCompatible(csvPgType, dbPgType string) bool {
	dbPgType = strings.ToLower(dbPgType)
	csvPgType = strings.ToLower(csvPgType)

	if csvPgType == dbPgType {
		return true
	}

	switch csvPgType {
	case "int":
		if dbPgType == "bigint" || dbPgType == "numeric" || dbPgType == "float8" {
			return true
		}
	case "float8":
		if dbPgType == "numeric" || dbPgType == "double precision" {
			return true
		}
	case "date":
		if dbPgType == "timestamp" || dbPgType == "timestamp without time zone" {
			return true
		}
	case "text":
		if strings.HasPrefix(dbPgType, "varchar") || dbPgType == "text" {
			return true
		}
	}

	return false
}

// CreateTable creates a new table in the database
func (r *repository) CreateTable(ctx context.Context, tableName string, headers []models.ColumnInfo) error {
	if tableName == "" {
		return errors.New("table name cannot be empty")
	}

	tableName = strings.ReplaceAll(tableName, `"`, "")

	var cols []string
	for _, col := range headers {
		colName := strings.ReplaceAll(col.Name, `"`, "")
		cols = append(cols, fmt.Sprintf(`"%s" %s`, colName, col.DataType))
	}

	query := fmt.Sprintf(`CREATE TABLE "public"."%s" (%s)`, tableName, strings.Join(cols, ", "))
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create table '%s': %w", tableName, err)
	}
	return nil
}

// TruncateTable removes all of the rows from a database table
func (r *repository) TruncateTable(ctx context.Context, tableName string) error {
	if tableName == "" {
		return errors.New("table name cannot be empty")
	}

	tableName = strings.ReplaceAll(tableName, `"`, "")
	query := fmt.Sprintf(`TRUNCATE TABLE "public"."%s"`, tableName)

	if _, err := r.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to truncate table: %w", err)
	}
	return nil
}

// InsertData inserts rows into a database table
// This is a row-by-row insert, which can be slow for large datasets
// For better performance with large files, consider using COPY FROM
func (r *repository) InsertData(ctx context.Context, tableName string, headers []string, rows [][]string) error {
	if tableName == "" {
		return errors.New("table name cannot be empty")
	}
	tableName = strings.ReplaceAll(tableName, `"`, "")

	if len(headers) == 0 || len(rows) == 0 {
		return nil
	}

	var placeholders []string
	for i := 1; i <= len(headers); i++ {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
	}
	sanitizedHeaders := make([]string, len(headers))
	for i, h := range headers {
		sanitizedHeaders[i] = fmt.Sprintf(`"%s"`, strings.ReplaceAll(h, `"`, ""))
	}

	query := fmt.Sprintf(`INSERT INTO "public"."%s" (%s) VALUES (%s)`,
		tableName, strings.Join(sanitizedHeaders, ", "), strings.Join(placeholders, ", "))

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PreparexContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	for i, row := range rows {
		if len(row) != len(headers) {
			log.Printf("Skipping row %d due to incorrect number of fields (expected %d, got %d)",
				i+1, len(headers), len(row))
			continue
		}

		args := make([]interface{}, len(row))
		for j, val := range row {
			args[j] = val
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			log.Printf("Failed to insert row %d (%v): %v", i+1, row, err)
			return fmt.Errorf("failed to insert row %d: %w", i+1, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
