package repositories

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/chiltom/SheetBridge/internal/apperrors"
	"github.com/chiltom/SheetBridge/internal/models"
	"github.com/chiltom/SheetBridge/internal/utils"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// DBRepository represents all of the database CRUD operations for the application
type DBRepository struct {
	db *sqlx.DB
}

// NewDBRepository returns a new DBRepository
func NewDBRepository(cfg *utils.Config) (*DBRepository, error) {
	db, err := sqlx.Connect("postgres", cfg.DB.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database with sqlx: %w", err)
	}

	db.SetMaxOpenConns(cfg.DB.MaxOpenConns)
	db.SetMaxIdleConns(cfg.DB.MaxIdleConns)
	db.SetConnMaxIdleTime(cfg.DB.MaxIdleTime)

	// Verify the connection (Connect already does this, but Ping is good for explicit check)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DBRepository{db: db}, nil
}

// Close closes the database connection
func (r *DBRepository) Close() error {
	return r.db.Close()
}

// Beginx starts a transaction
func (r *DBRepository) Beginx() (*sqlx.Tx, error) {
	return r.db.Beginx()
}

// GetTableNames fetches all table names from the database
func (r *DBRepository) GetTableNames(ctx context.Context) ([]string, *apperrors.AppError) {
	query := `
		SELECT tablename
		FROM pg_catalog.pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename;
	`
	var names []string
	if err := r.db.SelectContext(ctx, &names, query); err != nil {
		return nil, apperrors.Wrap(err, apperrors.ErrDatabase, "failed to query table names")
	}
	return names, nil
}

// TableExists checks the existence of a table by name
func (r *DBRepository) TableExists(ctx context.Context, tableName string) (bool, *apperrors.AppError) {
	query := `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		);
	`
	var exists bool
	if err := r.db.GetContext(ctx, &exists, query, tableName); err != nil {
		return false, apperrors.Wrap(err, apperrors.ErrDatabase, fmt.Sprintf("failed to check if table '%s' exists", tableName))
	}
	return exists, nil
}

// mapToPostgresType maps similar datatypes to their PostgreSQL equivalent
func mapToPostgresType(userType string) string {
	switch strings.ToUpper(userType) {
	case "INT", "INTEGER":
		return "INTEGER"
	case "BIGINT":
		return "BIGINT"
	case "DECIMAL", "NUMERIC":
		return "NUMERIC"
	case "REAL", "FLOAT", "DOUBLE":
		return "REAL"
	case "DATE":
		return "DATE"
	case "TIMESTAMP", "DATETIME":
		return "TIMESTAMP WITHOUT TIME ZONE"
	case "BOOLEAN":
		return "BOOLEAN"
	default:
		return "TEXT"
	}
}

// mapToAppType maps similar datatypes from Postgres to their application equivalent
func mapToAppType(pgType string) string {
	pgType = strings.ToLower(strings.TrimSpace(pgType))

	switch {
	case pgType == "integer" || pgType == "smallint" || pgType == "serial" || pgType == "smallserial":
		return "INTEGER"
	case pgType == "bigint" || pgType == "bigserial":
		return "BIGINT"
	case pgType == "numeric" || pgType == "decimal":
		return "NUMERIC"
	case pgType == "real" || pgType == "double precision":
		return "REAL"
	case pgType == "boolean":
		return "BOOLEAN"
	case pgType == "date":
		return "DATE"
	case strings.HasPrefix(pgType, "timestamp"): // "timestamp without time zone", "timestamp with time zone"
		return "TIMESTAMP"
	case pgType == "text":
		return "TEXT"
	case strings.HasPrefix(pgType, "character varying"): // varchar(n)
		return "TEXT"
	case strings.HasPrefix(pgType, "char"): // char(n)
		return "TEXT"
	case pgType == "json" || pgType == "jsonb":
		return "TEXT"
	case pgType == "uuid":
		return "TEXT"
	case pgType == "bytea":
		return "TEXT"
	default:
		return "TEXT"
	}
}

// GetTableSchema retrieves the column names and mapped application types for a given table
func (r *DBRepository) GetTableSchema(ctx context.Context, tableName string) ([]models.ColumnDefinition, *apperrors.AppError) {
	query := `
		SELECT
			column_name,
			data_type
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position;
	`

	var rawDbColumns []models.ColumnDefinition
	err := r.db.SelectContext(ctx, &rawDbColumns, query, tableName)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ErrDatabase, fmt.Sprintf("failed to query schema for table '%s'", tableName))
	}

	if len(rawDbColumns) == 0 {
		return nil, apperrors.Wrap(nil, apperrors.ErrNotFound, fmt.Sprintf("no columns found for table '%s' in public schema, or table does not exist", tableName))
	}

	appColDefinitions := make([]models.ColumnDefinition, len(rawDbColumns))
	for i, col := range rawDbColumns {
		appColDefinitions[i] = models.ColumnDefinition{
			Name: col.Name,
			Type: mapToAppType(col.Type),
		}
	}
	return appColDefinitions, nil
}

// CreateTable creates a new table in the database using either the provided sqlx transaction or the repository database
func (r *DBRepository) CreateTable(ctx context.Context, tx *sqlx.Tx, tableName string, columns []models.ColumnDefinition) *apperrors.AppError {
	if len(columns) == 0 {
		return apperrors.New("invalid_operation_create_table", "no columns defined for table creation")
	}

	var defs []string
	for _, col := range columns {
		defs = append(defs, fmt.Sprintf("%s %s", pq.QuoteIdentifier(col.Name), mapToPostgresType(col.Type)))
	}
	query := fmt.Sprintf("CREATE TABLE public.%s (%s);", pq.QuoteIdentifier(tableName), strings.Join(defs, ", "))

	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query)
	} else {
		_, err = r.db.ExecContext(ctx, query)
	}

	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrDatabase, fmt.Sprintf("failed to create table '%s'", tableName))
	}
	return nil
}

// DropTable drops a table in the database using either the provided sqlx transaction or the repository database
func (r *DBRepository) DropTable(ctx context.Context, tx *sqlx.Tx, tableName string) *apperrors.AppError {
	query := fmt.Sprintf("DROP TABLE IF EXISTS public.%s;", pq.QuoteIdentifier(tableName))
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query)
	} else {
		_, err = r.db.ExecContext(ctx, query)
	}

	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrDatabase, fmt.Sprintf("failed to drop table '%s'", tableName))
	}
	return nil
}

// InsertData inserts rows into the specified table in the database
func (r *DBRepository) InsertData(ctx context.Context, tx *sqlx.Tx, tableName string, columnDefs []models.ColumnDefinition, records [][]string) *apperrors.AppError {
	if len(records) == 0 {
		return nil // No data to insert
	}
	if len(columnDefs) == 0 {
		return apperrors.New("invalid_operation_insert_data", "column definitions are required for data insertion")
	}

	var colNames []string
	for _, cd := range columnDefs {
		colNames = append(colNames, pq.QuoteIdentifier(cd.Name))
	}

	placeholders := make([]string, len(colNames))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	stmtStr := fmt.Sprintf("INSERT INTO public.%s (%s) VALUES (%s);",
		pq.QuoteIdentifier(tableName),
		strings.Join(colNames, ","),
		strings.Join(placeholders, ","))

	var stmt *sqlx.Stmt
	var err error
	if tx != nil {
		stmt, err = tx.PreparexContext(ctx, stmtStr)
	} else {
		stmt, err = r.db.PreparexContext(ctx, stmtStr)
	}

	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrDatabase, fmt.Sprintf("failed to prepare insert statement for table '%s'", tableName))
	}
	defer stmt.Close()

	for i, record := range records {
		if len(record) != len(columnDefs) {
			return apperrors.New("data_mismatch", fmt.Sprintf("row %d (1-indexed) has %d values, expected %d", i+1, len(record), len(columnDefs)))
		}

		values := make([]any, len(record))
		for j, valStr := range record {
			colType := strings.ToUpper(columnDefs[j].Type)
			cleanValStr := strings.TrimSpace(valStr)

			if cleanValStr == "" {
				values[j] = nil
				continue
			}

			var convErr error
			switch colType {
			case "INT", "INTEGER", "BIGINT":
				values[j], convErr = strconv.ParseInt(cleanValStr, 10, 64)
			case "DECIMAL", "NUMERIC", "REAL", "FLOAT", "DOUBLE":
				values[j], convErr = strconv.ParseFloat(cleanValStr, 64)
			case "DATE":
				layouts := []string{
					"2006-01-02",
					"01/02/2006",
					"2006/01/02",
					"Jan 2, 2006",
					"2-Jan-2006",
					time.RFC3339[:10],
				}
				parsed := false
				for _, layout := range layouts {
					if t, perr := time.Parse(layout, cleanValStr); perr == nil {
						values[j] = t.Format("2006-01-02")
						parsed = true
						break
					}
				}
				if !parsed {
					values[j] = cleanValStr
				}
			case "TIMESTAMP", "DATETIME":
				layouts := []string{
					time.RFC3339,
					time.RFC3339Nano,
					"2006-01-02 15:04:05",
					"2006-01-02T15:04:05Z07:00",
					"2006-01-02T15:04:05",
					"01/02/2006 15:04:05",
					"Jan 2, 2006 15:04:05",
				}
				parsed := false
				for _, layout := range layouts {
					if t, perr := time.Parse(layout, cleanValStr); perr == nil {
						values[j] = t.Format("2006-01-02 15:04:05.999999")
						parsed = true
						break
					}
				}
				if !parsed {
					values[j] = cleanValStr
				}
			case "BOOLEAN":
				lowerVal := strings.ToLower(cleanValStr)
				if lowerVal == "true" || lowerVal == "1" || lowerVal == "yes" || lowerVal == "t" {
					values[j] = true
				} else if lowerVal == "false" || lowerVal == "0" || lowerVal == "no" || lowerVal == "f" {
					values[j] = false
				} else {
					convErr = fmt.Errorf("unrecognized boolean value '%s'", valStr)
				}
			default: // TEXT
				values[j] = valStr
			}

			if convErr != nil {
				return apperrors.Wrap(convErr, apperrors.ErrTypeConversion, fmt.Sprintf("Row %d, Column '%s': Failed to parse '%s' as %s", i+1, columnDefs[j].Name, valStr, colType))
			}
		}

		_, err = stmt.ExecContext(ctx, values...)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok {
				return apperrors.Wrap(err, apperrors.ErrDatabase, fmt.Sprintf("Failed to insert row %d into table '%s'. DB Error: %s (Detail: %s, Code: %s)", i+1, tableName, pqErr.Message, pqErr.Detail, pqErr.Code))
			}
			return apperrors.Wrap(err, apperrors.ErrDatabase, fmt.Sprintf("failed to insert row %d into table '%s'", i+1, tableName))
		}
	}
	return nil
}
