# CSV Ingestion Tool

A Go-based web application for ingesting CSV files into a Postgres database with a modern, drag-and-drop frontend.

## Features
- Drag-and-drop CSV upload.
- Automatic data type inference (INTEGER, FLOAT, BOOLEAN, DATE, TEXT).
- Select existing Postgres tables for overwrite/append or create new tables.
- Preview first 50 rows before committing.
- Polished, responsive UI with animations.

## Requirements
- Go 1.21+
- Postgres
- Dependencies: `lib/pq`, `sqlx`, `godotenv`, `migrate`

## Setup
1. **Clone the repository**:
   ```bash
   git clone https://github.com/yourusername/project.git
   cd project
   ```

2. **Vendored dependencies** (for air-gapped):
   - Copy `vendor/` with:
     - `github.com/jmoiron/sqlx@v1.3.5`
     - `github.com/lib/pq@v1.10.9`
     - `github.com/joho/godotenv@v1.5.1`
     - `github.com/golang-migrate/migrate/v4@v4.17.0`

3. **Configure environment**:
   - Copy `.env.example` to `.env` and update with Postgres credentials:
     ```env
     DB_HOST=localhost
     DB_PORT=5432
     DB_USER=postgres
     DB_PASSWORD=your_password
     DB_NAME=spreadsheet_db
     SERVER_PORT=:8080
     ```

4. **Apply migrations**:
   ```bash
   migrate -path migrations -database "postgres://user:password@localhost:5432/spreadsheet_db?sslmode=disable" up
   ```

5. **Run the application**:
   ```bash
   go run cmd/server/main.go
   ```
   - Access at `http://localhost:8080`.

## Project Structure
```
├── cmd
│   └── server
│       └── main.go
├── internal
│   ├── handlers
│   ├── models
│   ├── repositories
│   ├── services
│   └── utils
├── migrations
├── vendor
├── web
│   ├── static
│   │   ├── css
│   │   └── js
│   └── templates
├── .env
├── .gitignore
├── LICENSE
├── README.md
├── go.mod
└── go.sum
```

## License
MIT License. See [LICENSE](LICENSE) for details.