package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/chiltom/SheetBridge/internal/apperrors"
	"github.com/chiltom/SheetBridge/internal/logger"
	"github.com/chiltom/SheetBridge/internal/models"
	"github.com/chiltom/SheetBridge/internal/repositories"
	"github.com/chiltom/SheetBridge/internal/services"
	// No direct import of main package types like application or render helpers
)

const MaxUploadSize = 20 << 20 // 20 MB

// Renderer defines an interface for rendering templates
// This decouples handlers from the specific rendering implementation in main
type Renderer interface {
	Render(w http.ResponseWriter, r *http.Request, status int, page string, data *models.TemplateData)
	ServerError(w http.ResponseWriter, r *http.Request, err error)
	ClientError(w http.ResponseWriter, r *http.Request, status int, message string)
	NotFound(w http.ResponseWriter, r *http.Request)
	MethodNotAllowed(w http.ResponseWriter, r *http.Request, allowedMethods ...string)
	NewTemplateData(r *http.Request) *models.TemplateData
}

// AppHandlers holds the necessary internal packages to implement application handlers
type AppHandlers struct {
	logger     *logger.Logger
	csvService *services.CSVService
	repo       *repositories.DBRepository
	renderer   Renderer
}

// NewAppHandlers creates a new application handler struct
func NewAppHandlers(l *logger.Logger, csv *services.CSVService, r *repositories.DBRepository, renderer Renderer) *AppHandlers {
	return &AppHandlers{
		logger:     l,
		csvService: csv,
		repo:       r,
		renderer:   renderer,
	}
}

// Home handles rendering the home page
func (h *AppHandlers) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		h.renderer.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		h.renderer.MethodNotAllowed(w, r, http.MethodGet)
		return
	}

	flash := r.URL.Query().Get("flash")
	ctx := r.Context()

	existingTables, err := h.repo.GetTableNames(ctx)
	if err != nil {
		h.logger.Error(err) // Log, but page can still render
	}

	data := h.renderer.NewTemplateData(r)
	data.Flash = flash
	data.Preview = &models.CSVPreview{ExistingTables: existingTables}

	h.renderer.Render(w, r, http.StatusOK, "home.page.tmpl", data)
}

// UploadCSV handles rendering the preview page and initiating CSV parsing
func (h *AppHandlers) UploadCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.renderer.MethodNotAllowed(w, r, http.MethodPost)
		return
	}
	ctx := r.Context()

	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize)
	if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			h.renderer.ClientError(w, r, http.StatusRequestEntityTooLarge, "File exceeds maximum allowed size of 20MB.")
			return
		}
		h.renderer.ClientError(w, r, http.StatusBadRequest, "Error parsing multipart form.")
		return
	}

	file, handler, err := r.FormFile("csvfile")
	if err != nil {
		h.logger.Errorf("Error getting form file 'csvfile': %v", err)
		redirectWithFlash(w, r, "/", "Error: Could not read uploaded file. Please try again.", true)
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(handler.Filename), ".csv") {
		redirectWithFlash(w, r, "/", "Error: Invalid file type. Please upload a .csv file.", true)
		return
	}

	headers, previewRows, tempFilePath, appErr := h.csvService.ParseUploadedCSV(handler)
	if appErr != nil {
		h.logger.Error(appErr)
		if tempFilePath != "" {
			os.Remove(tempFilePath)
		}
		redirectWithFlash(w, r, "/", fmt.Sprintf("Error parsing CSV: %s", appErr.Message), true)
		return
	}

	suggestedTableName := h.csvService.SanitizeTableName(handler.Filename)
	existingTables, dbAppErr := h.repo.GetTableNames(ctx)
	if dbAppErr != nil {
		h.logger.Error(dbAppErr)
	}

	data := h.renderer.NewTemplateData(r)
	data.Preview = &models.CSVPreview{
		OriginalFilename: handler.Filename,
		TempFilePath:     tempFilePath,
		Headers:          headers,
		PreviewRows:      previewRows,
		SuggestedTable:   suggestedTableName,
		ExistingTables:   existingTables,
	}
	data.Form = &models.CommitRequest{TableName: suggestedTableName, Action: "create"}
	h.renderer.Render(w, r, http.StatusOK, "preview.page.tmpl", data)
}

// CommitCSV handles committing the parsed spreadsheet
func (h *AppHandlers) CommitCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.renderer.MethodNotAllowed(w, r, http.MethodPost)
		return
	}
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.renderer.ClientError(w, r, http.StatusBadRequest, "Error parsing form data.")
		return
	}

	req := models.CommitRequest{
		TempFilePath:     r.PostFormValue("tempFilePath"),
		TableName:        h.csvService.SanitizeTableName(r.PostFormValue("tableName")),
		Action:           models.CommitAction(r.PostFormValue("action")),
		ColumnNames:      r.Form["columnNames"],
		ColumnTypes:      r.Form["columnTypes"],
		OriginalFilename: r.PostFormValue("originalFilename"),
	}

	if req.TableName == "" || req.TempFilePath == "" || len(req.ColumnNames) == 0 || len(req.ColumnNames) != len(req.ColumnTypes) {
		h.logger.Errorf("Commit validation failed: %+v", req)
		redirectWithFlash(w, r, "/", "Error: Invalid commit data. Missing fields or mismatched columns/types.", true)
		return
	}
	defer os.Remove(req.TempFilePath)

	columnDefs := make([]models.ColumnDefinition, len(req.ColumnNames))
	for i, rawColName := range req.ColumnNames {
		sanitizedColName := h.csvService.SanitizeSQLName(rawColName)
		if sanitizedColName == "" || sanitizedColName == "unnamed_identifier" || sanitizedColName == "sanitized_empty_identifier" {
			sanitizedColName = fmt.Sprintf("column_%d", i+1)
		}
		columnDefs[i] = models.ColumnDefinition{Name: sanitizedColName, Type: req.ColumnTypes[i]}
	}

	_, allRecords, appErr := h.csvService.ReadFullCSV(req.TempFilePath)
	if appErr != nil {
		h.logger.Error(appErr)
		redirectWithFlash(w, r, "/", fmt.Sprintf("Error reading full CSV data: %s", appErr.Message), true)
		return
	}

	tx, err := h.repo.Beginx()
	if err != nil {
		h.renderer.ServerError(w, r, apperrors.Wrap(err, apperrors.ErrDatabase, "failed to begin transaction"))
		return
	}
	// Defer rollback. If Commit() is called, this is a no-op.
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil { // Check if 'err' was set by operations below
			tx.Rollback()
		}
	}()

	tableExists, appErr := h.repo.TableExists(ctx, req.TableName)
	if appErr != nil {
		err = appErr // Set outer err for rollback
		h.renderer.ServerError(w, r, appErr)
		return
	}

	flashMessage := ""
	var operationErr *apperrors.AppError

	if tableExists {
		switch req.Action {
		case "overwrite":
			if operationErr = h.repo.DropTable(ctx, tx, req.TableName); operationErr == nil {
				operationErr = h.repo.CreateTable(ctx, tx, req.TableName, columnDefs)
			}
			if operationErr == nil {
				flashMessage = fmt.Sprintf("Success: Table '%s' overwritten.", req.TableName)
			}
		case "append":
			flashMessage = fmt.Sprintf("Success: Data appended to table '%s'.", req.TableName)
		case "create":
			operationErr = apperrors.Wrap(nil, apperrors.ErrDataConflict, fmt.Sprintf("Table '%s' already exists. Choose 'Overwrite' or 'Append'.", req.TableName))
		default:
			operationErr = apperrors.New("invalid_action", "Invalid action specified.")
		}
	} else { // Table does not exist
		if req.Action == "append" {
			operationErr = apperrors.Wrap(nil, apperrors.ErrDataConflict, fmt.Sprintf("Cannot append. Table '%s' does not exist. Choose 'Create'.", req.TableName))
		} else { // create or overwrite (implies create due to table not existing already)
			operationErr = h.repo.CreateTable(ctx, tx, req.TableName, columnDefs)
			if operationErr == nil {
				flashMessage = fmt.Sprintf("Success: Table '%s' created.", req.TableName)
			}
		}
	}

	if operationErr != nil {
		err = operationErr // Set outer err for rollback
		if apperrors.Is(operationErr, apperrors.ErrDataConflict) || operationErr.Code == "invalid_action" {
			redirectWithFlash(w, r, "/", "Error: "+operationErr.Message, true)
		} else {
			h.renderer.ServerError(w, r, operationErr)
		}
		return
	}

	if operationErr = h.repo.InsertData(ctx, tx, req.TableName, columnDefs, allRecords); operationErr != nil {
		err = operationErr // Set outer err for rollback
		h.logger.Error(operationErr)
		detailedMsg := fmt.Sprintf("Error inserting data into '%s': %s", req.TableName, operationErr.Message)
		redirectWithFlash(w, r, "/", detailedMsg, true)
		return
	}

	if err = tx.Commit(); err != nil { // This is the final commit error
		h.renderer.ServerError(w, r, apperrors.Wrap(err, apperrors.ErrDatabase, "failed to commit transaction"))
		return
	}

	redirectWithFlash(w, r, "/", flashMessage, false)
}

// HealthCheckHandler provides a route for services to check the state of the server
func (h *AppHandlers) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.renderer.MethodNotAllowed(w, r, http.MethodGet)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// redirectWithFlash is a helper (not part of AppHandlers)
func redirectWithFlash(w http.ResponseWriter, r *http.Request, path, message string, isError bool) {
	if isError && !strings.HasPrefix(strings.ToLower(message), "error: ") {
		message = "Error: " + message
	}
	http.Redirect(w, r, path+"?flash="+url.QueryEscape(message), http.StatusSeeOther)
}
