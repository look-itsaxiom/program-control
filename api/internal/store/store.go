// Package store is your data layer. Nothing here is required by the tests; shape it as you see
// fit.
package store

import "github.com/jackc/pgx/v5/pgxpool"

// DB is a placeholder alias so this package is non-empty before you write your own types.
type DB = *pgxpool.Pool
