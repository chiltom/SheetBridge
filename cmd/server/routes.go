package main

import (
	"fmt"
	"io/fs"
	"net/http"

	"github.com/chiltom/SheetBridge/web" // For embedded files
)

// routes maps the application endpoints to their respective handlers
func (app *application) routes() (http.Handler, error) {
	mux := http.NewServeMux()

	// Serve static files from the embedded filesystem
	// The path must be "static/" because web.Files embeds the "static" directory
	// http.StripPrefix is important to map /static/css/styles.css to static/css/style.css in embed.FS
	staticFS, err := fs.Sub(web.Files, "static")
	if err != nil {
		return nil, fmt.Errorf("error creating a filesystem for static dir: %w", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Dynamic application routes
	mux.HandleFunc("/", app.handlers.Home)
	mux.HandleFunc("/upload", app.handlers.UploadCSV)
	mux.HandleFunc("/commit", app.handlers.CommitCSV)
	mux.HandleFunc("/healthz", app.handlers.HealthCheckHandler)

	var chain http.Handler = mux
	chain = app.logRequest(chain)
	chain = app.recoverPanic(chain)
	// Add other global middleware here

	return chain, nil
}
