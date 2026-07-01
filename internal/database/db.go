package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func InitDB() *sql.DB {
	host := "localhost"
	port := 5432
	user := "admin"
	password := "secretpassword"
	dbname := "adtech_db"

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatalf("Błąd krytyczny podczas otwierania połączenia z bazą: %v", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatalf("Nie można połączyć się z bazą danych (Ping failed): %v", err)
	}

	fmt.Println("Pomyślnie połączono z bazą PostgreSQL!")

	return db
}
