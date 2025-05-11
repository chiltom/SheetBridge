package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/chiltom/SheetBridge/internal/models"
	"github.com/chiltom/SheetBridge/internal/utils"
	"github.com/jmoiron/sqlx"
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
	return &repository{db: db}, nil
}

// Close closes the database connection
func (r *repository) Close() {
	r.db.Close()
}

// GetAllTables retrieves all table names
func (r *repository) GetAllTables(ctx context.Context) ([]string, error) {
	var tables []string
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
	`

	err := r.db.SelectContext(ctx, &tables, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	return tables, nil
}

// ValidateTable validates that a table's schema matches a spreadsheet or not
func (r *repository) ValidateTable(ctx context.Context, tableName string, headers []models.ColumnInfo) error {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT column_name, data_type
	  FROM information_schema.columns
		WHERE table_name = $1
	`, tableName)
	if err != nil {
		return fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()

	tableCols := make(map[string]string)
	for rows.Next() {
		var colName, dataType string
		if err := rows.Scan(&colName, &dataType); err != nil {
			return fmt.Errorf("failed to scan column: %w", err)
		}
		tableCols[colName] = normalizeDataType(dataType)
	}

	for _, col := range headers {
		dbType, exists := tableCols[col.Name]
		if !exists {
			return fmt.Errorf("column %s not found in table %s", col.Name, tableName)
		}
		if !isCompatibleType(col.DataType, dbType) {
			return fmt.Errorf("type mismatch for column %s: CSV %s, DB %s", col.Name, col.DataType, dbType)
		}
	}
	return nil
}

// CreateTable creates a new table in the database
func (r *repository) CreateTable(ctx context.Context, tableName string, headers []models.ColumnInfo) error {
	var cols []string
	for _, col := range headers {
		cols = append(cols, fmt.Sprintf("%s %s", col.Name, col.DataType))
	}
	query := fmt.Sprintf("CREATE TABLE %s (%s)", tableName, strings.Join(cols, ", "))
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	return nil
}

// TruncateTable removes all of the rows from a database table
func (r *repository) TruncateTable(ctx context.Context, tableName string) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s", tableName))
	if err != nil {
		return fmt.Errorf("failed to truncate table: %w", err)
	}
	return nil
}

// InsertData inserts rows into a database table
func (r *repository) InsertData(ctx context.Context, tableName string, headers []string, rows [][]string) error {
	var placeholders []string
	for i := 1; i <= len(headers); i++ {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName, strings.Join(headers, ", "), strings.Join(placeholders, ", "))

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, row := range rows {
		args := make([]interface{}, len(row))
		for i, val := range row {
			args[i] = val
		}
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return fmt.Errorf("failed to insert row: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// normalizeDataType returns a generalized value from different postgres data types for comparison
func normalizeDataType(pgType string) string {
	switch strings.ToLower(pgType) {
	case "integer", "bigint":
		return "INTEGER"
	case "real", "double precision":
		return "FLOAT"
	case "boolean":
		return "BOOLEAN"
	case "date", "timestamp", "timestamp without time zone":
		return "DATE"
	default:
		return "TEXT"
	}
}

func isCompatibleType(csvType, dbType string) bool {
	if csvType == dbType {
		return true
	}
	if csvType == "INTEGER" && dbType == "FLOAT" {
		return true
	}
	return false
}
