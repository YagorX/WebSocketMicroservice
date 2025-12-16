package main

import (
	"errors"
	"flag"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var (
		databaseURL     string
		migrationsPath  string
		migrationsTable string
	)

	flag.StringVar(&databaseURL, "database-url", "", "PostgreSQL connection URL")
	flag.StringVar(&migrationsPath, "migrations-path", "", "path to migrations")
	flag.StringVar(&migrationsTable, "migrations-table", "migrations", "name of migrations table")
	flag.Parse()

	if databaseURL == "" {
		panic("database-url is required")
	}
	if migrationsPath == "" {
		panic("migrations-path is required")
	}

	// separator := "?"
	// if strings.Contains(databaseURL, "?") {
	// 	separator = "&"
	// }
	m, err := migrate.New(
		"file://"+migrationsPath,
		databaseURL,
	)
	// dsn := fmt.Sprintf(
	// 	"%s%cx-migrations-table=%s",
	// 	databaseURL,
	// 	separator[0],
	// 	migrationsTable,
	// )

	// m, err := migrate.New(
	// 	"file://"+migrationsPath,
	// 	dsn,
	// )
	if err != nil {
		panic(err)
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("no migrations to apply")
			return
		}
		panic(err)
	}

	fmt.Println("migrations applied successfully")
}
