package store

import "github.com/jackc/pgx/v5/pgxpool"

type Repository struct {
	DB *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return Repository{DB: db}
}
