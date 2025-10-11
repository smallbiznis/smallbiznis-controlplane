package db

import (
	"fmt"

	"smallbiznis-controlplane/pkg/config"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Dialect(cfg *config.Config) (gorm.Dialector, error) {
	switch cfg.Database.Type {
	case "mysql":
		return mysql.Open(fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=%s",
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.DBNAME,
			cfg.Database.Timezone,
		)), nil
	case "postgres":
		return postgres.Open(fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
			cfg.Database.Host,
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.DBNAME,
			cfg.Database.Port,
			cfg.Database.SSLMode,
			cfg.Database.Timezone,
		)), nil
	case "sqlite":
		return sqlite.Open("gorm.db"), nil
	default:
		return nil, fmt.Errorf("invalid dialector type")
	}

}
