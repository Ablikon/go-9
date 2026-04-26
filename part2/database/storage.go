package database

import "database/sql"

type DBStore struct {
	DB *sql.DB
}

func NewDBStore(db *sql.DB) *DBStore {
	return &DBStore{DB: db}
}
