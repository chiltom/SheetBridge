package models

// ColumnInfo represents the schema information of a spreadsheet
type ColumnInfo struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
}

// CSVData represents the individual data for each spreadsheet
type CSVData struct {
	Headers []ColumnInfo `json:"headers"`
	Rows    [][]string   `json:"rows"`
}

// UploadResponse represents the response for a spreadsheet upload
type UploadResponse struct {
	CSVData
	TableNames []string `json:"tableNames"`
}

// CommitRequest represents an upload commit request and its operation
type CommitRequest struct {
	TableName string `json:"tableName"`
	Action    string `json:"action"` // "overwrite", "append", "create"
}
