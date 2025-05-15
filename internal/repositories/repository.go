package repositories

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/chiltom/SheetBridge/internal/models"
	"github.com/chiltom/SheetBridge/internal/utils"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Repository manages all of the database operations for the application
type Repository interface {
	GetAllTables(ctx context.Context) ([]string, error)
	ValidateTable(ctx context.Context, tableName string, csvHeaders []models.ColumnInfo) error
	CreateTable(ctx context.Context, tableName string, headers []models.ColumnInfo) error
	TruncateTable(ctx context.Context, tableName string) error
	InsertData(ctx context.Context, tableName string, colInfo []models.ColumnInfo, csvDataReader io.Reader) error
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

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

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
	tables := make([]string, 0)
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
		WHERE table_name = $1 AND table_schema = 'public'
		ORDER BY ordinal_position
	`

	err := r.db.SelectContext(ctx, &dbCols, query, tableName)
	if err != nil {
		return fmt.Errorf("failed to query columns for table %s: %w", tableName, err)
	}
	if len(dbCols) == 0 {
		return fmt.Errorf("table '%s' does not exist or has no columns", tableName)
	}
	if len(csvHeaders) != len(dbCols) {
		return fmt.Errorf("column count mismatch: CSV has %d columns, table '%s' has %d columns",
			len(csvHeaders), tableName, len(dbCols))
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

		if !isPostgresTypeCompatible(csvCol.DataType, dbType) {
			return fmt.Errorf("type mismatch for column '%s': CSV type '%s' is not compatible with DB type '%s' in table '%s'",
				csvCol.Name, csvCol.DataType, dbType, tableName)
		}
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
	// Allow TEXT in CSV to be inserted into various character types
	if csvPgType == "text" && (strings.HasPrefix(dbPgType, "character varying") || dbPgType == "varchar" || dbPgType == "char" || dbPgType == "bpchar") {
		return true
	}
	// Allow INT from CSV to go into BIGINT or NUMERIC or REAL/DOUBLE PRECISION in DB
	if csvPgType == "int" && (dbPgType == "bigint" || dbPgType == "numeric" || dbPgType == "real" || dbPgType == "double precision" || dbPgType == "float4" || dbPgType == "float8") {
		return true
	}
	// Allow FLOAT8 from CSV to go into NUMERIC or DOUBLE PRECISION in DB
	if csvPgType == "float8" && (dbPgType == "numeric" || dbPgType == "double precision" || dbPgType == "real" || dbPgType == "float4") {
		return true
	}
	// Allow DATE from CSV to go into TIMESTAMP types
	if csvPgType == "date" && (dbPgType == "timestamp" || dbPgType == "timestamp without time zone" || dbPgType == "timestamptz" || dbPgType == "timestamp with time zone") {
		return true
	}
	// A common case: numeric in database can accept int or float from CSV
	if dbPgType == "numeric" && (csvPgType == "int" || csvPgType == "float8") {
		return true
	}
	return false
}

// CreateTable creates a new table in the database
func (r *repository) CreateTable(ctx context.Context, tableName string, headers []models.ColumnInfo) error {
	if strings.TrimSpace(tableName) == "" {
		return errors.New("table name cannot be empty")
	}

	safeTableName := pq.QuoteIdentifier(tableName)

	var colDefinitions []string
	for _, col := range headers {
		safeColName := pq.QuoteIdentifier(col.Name)
		colDefinitions = append(colDefinitions, fmt.Sprintf("%s %s", safeColName, col.DataType))
	}

	query := fmt.Sprintf(`CREATE TABLE public.%s (%s)`, safeTableName, strings.Join(colDefinitions, ", "))
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create table '%s': %w", tableName, err)
	}
	return nil
}

// TruncateTable removes all of the rows from a database table
func (r *repository) TruncateTable(ctx context.Context, tableName string) error {
	if strings.TrimSpace(tableName) == "" {
		return errors.New("table name cannot be empty")
	}

	safeTableName := pq.QuoteIdentifier(tableName)
	query := fmt.Sprintf(`TRUNCATE TABLE public.%s`, safeTableName)

	if _, err := r.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to truncate table %s: %w", tableName, err)
	}
	return nil
}

// InsertData inserts rows into a database table using COPY FROM STDIN
// for efficient bulk insertion.
func (r *repository) InsertData(ctx context.Context, tableName string, colInfo []models.ColumnInfo, csvDataReader io.Reader) error {
	if strings.TrimSpace(tableName) == "" {
		return errors.New("table name cannot be empty")
	}
	if len(colInfo) == 0 {
		return errors.New("no column information provided for insert")
	}

	// Use sql.Tx for pq.CopyIn
	txn, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repository: failed to begin transaction for COPY: %w", err)
	}
	// Defer rollback/commit handling
	var opErr error
	defer func() {
		if p := recover(); p != nil {
			txn.Rollback()
			panic(p)
		} else if opErr != nil {
			txn.Rollback()
		} else {
			opErr = txn.Commit()
		}
	}()

	pqColumnNames := make([]string, len(colInfo))
	for i, ci := range colInfo {
		pqColumnNames[i] = ci.Name // pq.CopyInSchema handles quoting internally
	}

	// Prepare the COPY statement using pq.CopyInSchema
	stmt, err := txn.PrepareContext(ctx, pq.CopyInSchema("public", tableName, pqColumnNames...))
	if err != nil {
		opErr = fmt.Errorf("repository: failed to prepare COPY statement using pq.CopyInSchema: %w", err)
		return opErr
	}
	defer stmt.Close()

	// Parse the CSV from the reader and feed rows to the COPY statement
	parser := csv.NewReader(csvDataReader)
	parser.LazyQuotes = true
	parser.TrimLeadingSpace = true

	if _, err := parser.Read(); err != nil { // Skip header row from the input reader
		opErr = fmt.Errorf("repository: failed to read/skip header from CSV for COPY: %w", err)
		return opErr
	}

	for {
		row, err := parser.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			opErr = fmt.Errorf("repository: error reading CSV row for COPY: %w", err)
			return opErr
		}

		args := make([]interface{}, len(row))
		for i, val := range row {
			// For COPY, pq expects string values or nil for NULL.
			// Empty strings are inserted as empty strings by default.
			// If a column is not text and an empty string is provided,
			// Postgres might error or convert it based on type.
			// To treat empty strings as NULL for non-text types:
			if val == "" && !isTextType(colInfo[i].DataType) {
				args[i] = nil
			} else {
				args[i] = val
			}
			// For simplicity with COPY, you can pass strings as is.
			// args[i] = val
		}
		if _, err = stmt.ExecContext(ctx, args...); err != nil {
			opErr = fmt.Errorf("repository: error executing COPY for row %v: %w", row, err)
			return opErr
		}
	}

	// Finalize COPY operation (flushes buffers)
	if _, err = stmt.ExecContext(ctx); err != nil {
		opErr = fmt.Errorf("repository: error finalized COPY operation: %w", err)
		return opErr
	}

	return opErr // Will be nil if commit is successful
}

// Returns if a field data type is text or not
func isTextType(pgType string) bool {
	return strings.ToLower(pgType) == "text"
}
