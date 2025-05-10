package repositories

import (
	"context"
	"fmt"

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
