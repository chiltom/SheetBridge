# SheetBridge - CSV to PostgreSQL Ingestion Tool

SheetBridge is a web-based tool built with Go that allows users to easily upload CSV files and ingest their data into a PostgreSQL database. It provides a user-friendly interface to define table schemas, preview data, and manage how data is imported (create, overwrite, or append).

## Features

- **Drag & Drop CSV Upload:** Simple file uploading (standard file input also supported).
- **Schema Detection & Customization:**
  - Automatic detection of CSV headers.
  - User-configurable column names and data types (INT, TEXT, DECIMAL, DATE, TIMESTAMP, BOOLEAN).
  - Suggested table name based on the CSV filename.
- **Data Preview:** Preview the first 50 rows of the CSV before committing.
- **Table Management:**
  - Create new tables in PostgreSQL.
  - Overwrite existing tables.
  - Append data to existing tables.
- **User-Friendly Interface:** Built with Go templates, Tailwind CSS, and DaisyUI for a clean and modern look.
- **Standardized Logging & Errors:** Clear and descriptive logging and error handling.
- **Embeddable UI:** Static assets and templates are embedded into the Go binary for easy deployment.

## Screenshots

**1. Home Page / Upload Interface:**
`![Home Page](./docs/landing-page.png)`

**2. Preview & Configuration Page:**
`![Preview Page](./docs/preview-screenshot.png)`

**3. Success/Error Flash Messages:**
`![Flash Message](./docs/successful-upload.png)`

---

## Tech Stack

- **Backend:** Go (Golang)
  - Standard Library (net/http, html/template, embed)
  - `github.com/jmoiron/sqlx` (for PostgreSQL interaction)
  - `github.com/lib/pq` (PostgreSQL driver)
  - `github.com/joho/godotenv` (for environment variable management in development)
- **Frontend:**
  - HTML5 (Go Templates)
  - Tailwind CSS (utility-first CSS framework)
  - DaisyUI (Tailwind CSS component library)
- **Database:** PostgreSQL
- **Development Tools:**
  - `make` (for build automation)
  - `air` (for live reloading during Go development)
  - Tailwind CSS Standalone CLI (for CSS compilation)

---

## Prerequisites

- **Go:** Version 1.21 or higher.
- **PostgreSQL:** A running PostgreSQL instance.
- **Make:** GNU Make (or compatible).
- **Tailwind CSS Standalone CLI (for CSS development/rebuild):**
  - Download from [Tailwind CSS Installation](https://tailwindcss.com/blog/standalone-cli).
  - Place the executable (e.g., `tailwindcss`) in the project root or ensure it's on your system `PATH`.
- **Node.js & npm (for Tailwind plugin - DaisyUI - during CSS development):**
  - Needed _only if you modify CSS/Tailwind configuration_ that uses plugins like DaisyUI. The Tailwind CLI needs to `require()` these plugins.
  - Run `npm install -D daisyui tailwindcss postcss autoprefixer` in the project root once to set up `node_modules`. This folder should be in `.gitignore`.

---

## Setup & Installation

1.  **Clone the Repository:**

    ```bash
    git clone https://github.com/chiltom/SheetBridge.git
    cd SheetBridge
    ```

2.  **Configuration:**

    - Copy the example environment file (create `.env.example` first if it doesn't exist):
      ```bash
      cp .env.example .env
      ```
    - Edit `.env` and configure both your server and database parameters.
    - Ensure the database specified in your environment variables exists in your PostgreSQL instance.

3.  **Install Go Dependencies & Vendor:**

    ```bash
    make tidy
    make vendor
    ```

4.  **Tailwind CSS & DaisyUI Setup (for CSS development):**
    - If you plan to modify styles or `tailwind.config.js`:
      - Ensure you have the Tailwind CSS Standalone CLI (see Prerequisites).
      - Run `npm init -y` (if no `package.json` exists).
      - Run `npm install -D daisyui tailwindcss postcss autoprefixer`. This creates `node_modules` which is used by the Tailwind CLI to find DaisyUI.
    - Build the initial CSS:
      ```bash
      make css-build
      ```
      This will generate `web/static/css/output.css`. If the Tailwind CLI is not found, this step will be skipped (useful if `output.css` is already committed or provided).

---

## Development

1.  **Run the Application (Go + CSS):**

    - **Terminal 1 (Go Application with Air + Initial CSS Build):**

      ```bash
      make run
      ```

      This command will:

      1.  Attempt to build `web/static/css/output.css` using Tailwind CLI (if found).
      2.  Build the Go application.
      3.  Start the Go application using `air` for live reloading of Go code changes.
          The application will be available at `http://localhost:PORT` (e.g., `http://localhost:4000`).

    - **Terminal 2 (Live CSS Rebuilds - Optional but Recommended for UI dev):**
      If you have the Tailwind CLI and want CSS to rebuild automatically when you change templates or `input.css`:
      ```bash
      make css-watch
      ```
      Refresh your browser to see style changes.

2.  **Directory Structure:**
    - `cmd/server/`: Main application entry point, server setup, routing, middleware.
    - `internal/`: Core application logic.
      - `apperrors/`: Custom error types.
      - `handlers/`: HTTP request handlers.
      - `logger/`: Application logger.
      - `models/`: Data structures.
      - `repositories/`: Database interaction logic (using `sqlx`).
      - `services/`: Business logic (e.g., CSV parsing).
      - `utils/`: Utility functions (e.g., configuration loading).
    - `migrations/`: SQL database migration files. (Use a separate migration tool to apply these).
    - `web/`: Frontend assets.
      - `static/`: CSS, JavaScript files.
        - `css/input.css`: Source file for Tailwind CSS.
        - `css/output.css`: Compiled Tailwind CSS (generated).
      - `templates/`: Go HTML templates.
    - `tailwind.config.js`: Tailwind CSS configuration.
    - `Makefile`: Build, run, and utility commands.
    - `.air.toml`: Configuration for `air` live reloader.

---

## Building for Production

To create a production build (Go binary and compiled CSS):

```bash
make build
```

This command will:

1.  Attempt to build `web/static/css/output.css` using Tailwind CLI (if found and configured).
2.  Build the Go application binary into the `bin/` directory (e.g., `bin/sheetbridge`).

The Go binary embeds the templates and static assets (including `output.css`), so for deployment, you primarily need the binary.

---

## Deployment (Example to `/opt`)

1.  **Build the Application:**
    On your development machine or CI server (where Tailwind CLI might be available):

    ```bash
    make build
    ```

2.  **Prepare Deployment Files:**
    You will need:

    - The compiled Go binary: `bin/sheetbridge`
    - (Optionally, if not embedding or for reference) The `web/static/css/output.css` file if it was generated. The binary embeds this, but having it can be useful.
    - An `.env` file for the production environment, or ensure environment variables (`PORT`, `DB_DSN`, `APP_ENV=prod`, etc.) are set on the server.

3.  **Copy to Server:**
    Transfer the Go binary (and `.env` file if used) to your server, for example, `/opt/sheetbridge/`:

    ```bash
    scp bin/sheetbridge youruser@yourserver:/opt/sheetbridge/sheetbridge
    scp .env.production youruser@yourserver:/opt/sheetbridge/.env # Example
    ```

    Ensure the binary is executable on the server: `chmod +x /opt/sheetbridge/sheetbridge`.

4.  **Run the Application on the Server:**
    Navigate to `/opt/sheetbridge/` on the server and run the binary:

    ```bash
    cd /opt/sheetbridge
    ./sheetbridge
    ```

    Consider running it as a systemd service or using a process manager like `supervisor` for robust production deployment.

5.  **Database Migrations:**
    Apply any pending database migrations to your production PostgreSQL database using your chosen migration tool. The migration files are in the `migrations/` directory.

---

## Makefile Commands

- `make run`: Start the development server with Air (includes initial CSS build).
- `make build`: Create a production build (Go binary and CSS).
- `make go-build`: Build only the Go application binary.
- `make css-build`: Compile Tailwind CSS into `web/static/css/output.css`.
- `make css-watch`: Watch for CSS changes and rebuild automatically (for development).
- `make tidy`: Run `go mod tidy`.
- `make vendor`: Run `go mod vendor`.
- `make clean`: Remove build artifacts and temporary files.
- `make test`: Run Go tests.
- `make help`: Display available Make commands.

---

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.

---

## License

[MIT](LICENSE)
