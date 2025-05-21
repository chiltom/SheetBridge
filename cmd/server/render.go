package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/chiltom/SheetBridge/internal/models"
	"github.com/chiltom/SheetBridge/web"
)

// humanDate transforms dates into readable formats
func humanDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:00")
}

// currentYear returns the current year as an integer
func currentYear() int {
	return time.Now().Year()
}

// findErrorClass checks for the presence of an error in a flash message
func findErrorClass(flashMsg string) bool {
	lowerMsg := strings.ToLower(flashMsg)
	return strings.Contains(lowerMsg, "error: ") || strings.Contains(lowerMsg, "failed: ") || strings.Contains(lowerMsg, "invalid: ")
}

// templateFunctions defines the function map that will be attached for use in all templates
var templateFunctions = template.FuncMap{
	"humanDate":      humanDate,
	"currentYear":    currentYear,
	"findErrorClass": findErrorClass,
}

// newTemplateCache creates a new template cache
func newTemplateCache() (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}

	// Use web.Files (embedded) to get all files ending with .page.tmpl
	// The path inside the embed FS is "templates/*.page.tmpl"
	pages, err := fs.Glob(web.Files, "templates/*.page.tmpl")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.Base(page) // e.g., home.page.tmpl

		// Create a new template set for each page
		// Add functions, then parse base layout
		ts, err := template.New(name).Funcs(templateFunctions).ParseFS(web.Files, "templates/base.layout.tmpl")
		if err != nil {
			return nil, fmt.Errorf("parsing base layout for %s: %w", name, err)
		}

		// Parse partials into the set
		// Ensure glob pattern is correct for embedded FS
		partials, _ := fs.Glob(web.Files, "templates/*.partial.tmpl")
		if len(partials) > 0 {
			ts, err = ts.ParseFS(web.Files, "templates/*.partial.tmpl")
			if err != nil {
				return nil, fmt.Errorf("parsing partials for %s: %w", name, err)
			}
		}

		// Parse the specific page template into the set
		ts, err = ts.ParseFS(web.Files, page) // page is like "templates/home.page.tmpl"
		if err != nil {
			return nil, fmt.Errorf("parsing page %s: %w", page, err)
		}
		cache[name] = ts
	}
	return cache, nil
}

// render executes the template for a specific page and renders it
func (app *application) render(w http.ResponseWriter, r *http.Request, status int, page string, data *models.TemplateData) {
	ts, ok := app.templateCache[page]
	if !ok {
		app.serverError(w, r, fmt.Errorf("the template %s does not exist", page))
		return
	}

	if app.config.IsDevelopment() { // Reload cache in dev mode
		freshCache, err := newTemplateCache()
		if err != nil {
			app.serverError(w, r, fmt.Errorf("rebuilding template cache: %w", err))
			return
		}
		ts, ok = freshCache[page]
		if !ok {
			app.serverError(w, r, fmt.Errorf("the template %s does not exist after refresh", page))
			return
		}
		app.templateCache = freshCache // Update app's cache
	}

	buf := new(bytes.Buffer)
	// Execute the "base" template definition, which then includes the specific page
	err := ts.ExecuteTemplate(buf, "base", data)
	if err != nil {
		app.serverError(w, r, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

// newTemplateData initializes common template data
func (app *application) newTemplateData(r *http.Request) *models.TemplateData {
	return &models.TemplateData{
		// Initialize with common data if any
	}
}
