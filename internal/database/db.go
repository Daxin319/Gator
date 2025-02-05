package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	_ "github.com/lib/pq"
)

type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

type Queries struct {
	db DBTX
}

func (q *Queries) WithTx(tx *sql.Tx) *Queries {
	return &Queries{
		db: tx,
	}
}

// PostgreSQL connection details

const (
	dbHost = "localhost"
	dbPort = 5432
	dbUser = "postgres"
	dbPass = "postgres"
	dbName = "gator"
)

// Check if PostgreSQL server is running
func isPostgresRunning() bool {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return false
	}
	defer db.Close()

	// Check connection
	err = db.Ping()
	return err == nil
}

// isAdmin checks if the program is running as administrator
func isAdmin() bool {
	var token syscall.Token
	currentProcess, err := syscall.GetCurrentProcess()
	if err != nil {
		fmt.Println("ERROR: ", err)
		os.Exit(1)
	}
	err = syscall.OpenProcessToken(currentProcess, syscall.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()

	var elevation uint32
	var size uint32
	err = syscall.GetTokenInformation(token, syscall.TokenElevation, (*byte)(unsafe.Pointer(&elevation)), uint32(unsafe.Sizeof(elevation)), &size)
	if err != nil {
		return false
	}
	return elevation != 0
}

// relaunchAsAdmin restarts the program with elevated privileges
func RelaunchAsAdmin() {
	if runtime.GOOS != "windows" || isAdmin() {
		return // Already running as admin or not on Windows
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Println("Failed to get executable path:", err)
		os.Exit(1)
	}

	// Relaunch the process as Administrator
	cmd := exec.Command("powershell", "-Command",
		"Start-Process", exe, "-ArgumentList", fmt.Sprintf(`"%s"`, strings.Join(os.Args[1:], " ")), "-Verb", "runAs")

	err = cmd.Start()
	if err != nil {
		fmt.Println("Failed to restart as administrator:", err)
		os.Exit(1)
	}

	os.Exit(0) // Exit the non-admin instance
}

// runAsAdmin ensures commands are executed with admin privileges
func runAsAdmin(cmd *exec.Cmd) error {
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error: %v\noutput: %s", err, string(output))
	}

	fmt.Println(string(output))
	return nil
}

// Try to start PostgreSQL
func startPostgres() error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("sudo", "systemctl", "start", "postgresql")
	case "darwin":
		cmd = exec.Command("brew", "services", "start", "postgresql")
	case "windows":
		cmd = exec.Command("net", "start", "postgresql")
		return runAsAdmin(cmd)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start PostgreSQL: %s - %s", err, string(output))
	}

	fmt.Println(string(output))
	return nil
}

// Ensure postgres is running, else start it
func EnsurePostgresRunning() {
	if isPostgresRunning() {
		fmt.Println("PostgreSQL is already running.")
		return
	}

	fmt.Println("PostgreSQL is not running. Attempting to start it...")
	err := startPostgres()
	if err != nil {
		fmt.Println("ERROR:", err)
		fmt.Println("Attempting to initialize PostgreSQL...")

		// Try initializing PostgreSQL if it's not yet set up
		err = initializePostgres()
		if err != nil {
			fmt.Println("CRITICAL ERROR: Failed to initialize PostgreSQL:", err)
			os.Exit(1)
		}

		// Retry starting PostgreSQL after initialization
		err = startPostgres()
		if err != nil {
			fmt.Println("CRITICAL ERROR: PostgreSQL failed to start even after initialization:", err)
			os.Exit(1)
		}
	}

	fmt.Println("PostgreSQL is now running!")
}

// Initialize postgres server if needed
func initializePostgres() error {
	var cmd *exec.Cmd
	var dataDir string

	switch runtime.GOOS {
	case "linux":
		dataDir = "/var/lib/postgresql/data"
		cmd = exec.Command("sudo", "-u", "postgres", "initdb", "-D", dataDir)

	case "darwin":
		dataDir = "/usr/local/var/postgres"
		cmd = exec.Command("initdb", "-D", dataDir)

	case "windows":
		// Use a user-writable directory instead of "C:\Program Files\PostgreSQL\data"
		dataDir = "C:\\Users\\lyleh\\postgres_data"

		// Ensure the directory exists
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			fmt.Println("Creating PostgreSQL data directory:", dataDir)
			if err := os.Mkdir(dataDir, 0700); err != nil {
				return fmt.Errorf("failed to create data directory: %s", err)
			}
		}

		// Run initdb
		cmd = exec.Command("pg_ctl", "initdb", "-D", dataDir)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	cmd.Env = os.Environ() // Preserve environment variables
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to initialize PostgreSQL: %s - %s", err, string(output))
	}

	fmt.Println("PostgreSQL initialized successfully.")

	// Start PostgreSQL after initialization (for Windows)
	if runtime.GOOS == "windows" {
		startCmd := exec.Command("pg_ctl", "start", "-D", dataDir, "-l", "postgres.log")
		startOutput, startErr := startCmd.CombinedOutput()
		if startErr != nil {
			return fmt.Errorf("failed to start PostgreSQL: %s - %s", startErr, string(startOutput))
		}
		fmt.Println("PostgreSQL started successfully.")
	}

	return nil
}

// Create database if no database exists
func EnsureDatabaseExists() error {

	// Connect to the default "postgres" database to check for 'gator'
	dsn := "host=localhost port=5432 user=postgres password=postgres dbname=gator sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("ERROR: Cannot connect to PostgreSQL: %w", err)
	}
	defer db.Close()

	// Check if gator db exists
	var exists bool
	err = db.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'gator')").Scan(&exists)
	if err != nil {
		return fmt.Errorf("ERROR: Failed to check database existence: %w", err)
	}

	// If the database does not exist, create it
	if !exists {
		_, err = db.Exec("CREATE DATABASE gator")
		if err != nil {
			return fmt.Errorf("ERROR: Failed to create database: %w", err)
		}
	}

	// connect to gator db and apply migrations
	gatorDSN := "host=localhost port=5432 user=postgres password=postgres dbname=gator sslmode=disable"
	dbGator, err := sql.Open("postgres", gatorDSN)
	if err != nil {
		return fmt.Errorf("ERROR: Failed to connect to 'gator' database: %w", err)
	}
	defer dbGator.Close()

	return applyMigrations(dbGator)
}

// executes the schema creation SQL in order
func applyMigrations(db *sql.DB) error {
	fmt.Println("Verifying files...")

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
		    id UUID PRIMARY KEY,
		    created_at TIMESTAMP NOT NULL,
		    updated_at TIMESTAMP NOT NULL,
		    name TEXT UNIQUE NOT NULL
		);`,

		`CREATE TABLE IF NOT EXISTS feeds (
		    id UUID PRIMARY KEY,
		    created_at TIMESTAMP NOT NULL,
		    updated_at TIMESTAMP NOT NULL,
		    name TEXT NOT NULL,
		    url TEXT UNIQUE NOT NULL,
		    user_id UUID NOT NULL,
		    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,

		`CREATE TABLE IF NOT EXISTS feed_follows (
		    id UUID PRIMARY KEY,
		    created_at TIMESTAMP NOT NULL,
		    updated_at TIMESTAMP NOT NULL,
		    user_id UUID NOT NULL,
		    feed_id UUID NOT NULL,
		    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
		    FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE,
		    UNIQUE (user_id, feed_id)
		);`,

		`ALTER TABLE feeds
		ADD COLUMN IF NOT EXISTS last_fetched_at TIMESTAMP;`,

		`CREATE TABLE IF NOT EXISTS posts (
		    id UUID PRIMARY KEY,
		    created_at TIMESTAMP NOT NULL,
		    updated_at TIMESTAMP NOT NULL,
		    title TEXT NOT NULL,
		    url TEXT NOT NULL UNIQUE,
		    description TEXT NOT NULL,
		    published_at TEXT NOT NULL,
		    feed_id UUID NOT NULL,
		    FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE
		);`,
	}

	for i, migration := range migrations {
		_, err := db.Exec(migration)
		if err != nil {
			return fmt.Errorf("failed to execute update %d: %w", i+1, err)
		}
	}

	fmt.Println("All files verified!")
	return nil
}
