package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // The PostgreSQL driver
)

// DB is the global database connection pool that your handlers will use
var DB *sql.DB

// Connect initializes the PostgreSQL connection
func Connect() {
	var dsn string

	if url := os.Getenv("DATABASE_URL"); url != "" {
		// Production: Neon gives a full connection string, already includes sslmode=require
		dsn = url
	} else {
		// Local dev: build from individual parts, no SSL needed
		host := os.Getenv("DB_HOST")
		port := os.Getenv("DB_PORT")
		user := os.Getenv("DB_USER")
		password := os.Getenv("DB_PASSWORD")
		dbname := os.Getenv("DB_NAME")
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			host, port, user, password, dbname)
	}

	var err error
	DB, err = sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	if err = DB.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("✅ Successfully connected to PostgreSQL")
}