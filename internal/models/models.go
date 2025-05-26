package models

type CommitAction string

const (
	create    CommitAction = "create"
	overwrite CommitAction = "overwrite"
	append    CommitAction = "append"
)

// ColumnDefinition describes a column in a table
type ColumnDefinition struct {
	Name string `db:"column_name" json:"name"`
	Type string `db:"data_type" json:"type"`
}

// CSVPreview holds data for the preview page
type CSVPreview struct {
	OriginalFilename   string             `json:"originalFilename"`
	TempFilePath       string             `json:"tempFilePath"`
	Headers            []string           `json:"headers"`
	PreviewRows        [][]string         `json:"previewRows"`
	ExistingTables     []string           `json:"existingTables"`
	SuggestedTable     string             `json:"suggestedTable"`
	TableExists        bool               `json:"tableExists"`
	InferredColumnDefs []ColumnDefinition `json:"inferredColumnDefs"`
	ActualColumnDefs   []ColumnDefinition `json:"actualColumnDefs"`
}

// CommitRequest is what's sent from the preview page to commit
type CommitRequest struct {
	TempFilePath     string       `form:"tempFilePath"`
	TableName        string       `form:"tableName"`
	Action           CommitAction `form:"action"`
	ColumnNames      []string     `form:"columnNames"`
	ColumnTypes      []string     `form:"columnTypes"`
	OriginalFilename string       `form:"originalFilename"`
}

// TemplateData is the base data structure for HTML templates
type TemplateData struct {
	Form    any    // To hold form data and errors (e.g., CommitRequest)
	Flash   string // Success/error messages
	Preview *CSVPreview
	// Add other common fields like CSRFToken string
}
