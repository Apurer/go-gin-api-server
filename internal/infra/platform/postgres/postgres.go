package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Connect opens a PostgreSQL connection via GORM and verifies connectivity.
func Connect(ctx context.Context, dsn string) (*gorm.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("postgres DSN is empty")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return db, nil
}
