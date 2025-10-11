package db

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

func IsDuplicateKeyErr(err error) bool {
	if err == nil {
		return false
	}

	// GORM wraps error di dalam gorm.Err* → unwrap dulu
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}

	// PostgreSQL (error code 23505)
	if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
		return true
	}

	// MySQL (error code 1062)
	if strings.Contains(err.Error(), "Error 1062") {
		return true
	}

	// SQLite (error code 2067)
	if strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return true
	}

	return false
}
