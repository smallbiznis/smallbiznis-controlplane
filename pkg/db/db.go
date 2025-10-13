package db

import (
	"context"
	"os"
	"strings"
	"time"

	"smallbiznis-controlplane/pkg/config"

	"github.com/uptrace/opentelemetry-go-extra/otelgorm"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/prometheus"
)

var Module = fx.Module("database",
	fx.Provide(
		Dialect,
		New,
	),
	// fx.Invoke(RegisterConnectionPool),
)

func New(cfg *config.Config, dialector gorm.Dialector, opts ...gorm.Option) *gorm.DB {
	var db *gorm.DB
	var err error

	var logLevel logger.LogLevel
	var showSQL bool

	if cfg.AppEnv == "production" {
		logLevel = logger.Warn
		showSQL = false
	} else {
		logLevel = logger.Info
		showSQL = true
	}

	gormLogger := NewZapGormLogger(zap.L(), logLevel, showSQL)

	for i := 0; i < 5; i++ {
		db, err = gorm.Open(dialector, &gorm.Config{
			Logger: gormLogger,
		})
		if err == nil {
			break
		}
		zap.L().Warn("[DB] Database not ready, retrying in 3 seconds... ", zap.Int("retry", i+1), zap.Error(err))
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		zap.L().Error("[DB] Failed to connect to database", zap.Error(err))
		os.Exit(1)
	}

	zap.L().Info("[DB] ✅ Database connection successfully configured.")

	return db
}

func NewTest() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

type connectionPoolParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	DB        *gorm.DB
	Config    *config.Config
}

func RegisterConnectionPool(p connectionPoolParams) {
	if p.DB == nil {
		zap.L().Error("[DB] Skipping connection pool setup (no db instance)")
		os.Exit(1)
	}

	sqlDB, err := p.DB.DB()
	if err != nil {
		zap.L().Error("[DB] ❌ Failed to get sql.DB from gorm", zap.Error(err))
		os.Exit(1)
	}

	cp := p.Config.Database.ConnectionPool
	sqlDB.SetMaxIdleConns(cp.MaxIdleConn)
	sqlDB.SetMaxOpenConns(cp.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cp.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cp.ConnMaxIdleTime)

	zap.L().Info("[DB] ✅ Database connection successfully configured with connection pooling.")
	p.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			zap.L().Info("[DB] Closing connection pool...")
			return sqlDB.Close()
		},
	})
}

// func RegisterAuditHooks(db *gorm.DB) {
// 	db.Callback().Create().Before("gorm:create").Register("audit_before_create", BeforeCreate)
// 	db.Callback().Update().Before("gorm:update").Register("audit_before_update", BeforeSave)
// }

func Otel(db *gorm.DB) error {
	// Register the OpenTelemetry plugin with GORM
	if err := db.Use(otelgorm.NewPlugin()); err != nil {
		zap.L().Error("❌ Failed to register db telemetry", zap.Error(err))
		return err
	}

	return nil
}

func Metric(db *gorm.DB) error {
	if err := db.Use(prometheus.New(prometheus.Config{
		DBName:          getDBNameFromDialector(db.Dialector), // use `DBName` as metrics label
		RefreshInterval: 15,                                   // Refresh metrics interval (default 15 seconds)
		PushAddr:        "localhost:9090",                     // push metrics if `PushAddr` configured
		StartServer:     true,                                 // start http server to expose metrics
		HTTPServerPort:  8080,                                 // configure http server port, default port 8080 (if you have configured multiple instances, only the first `HTTPServerPort` will be used to start server)
		MetricsCollector: []prometheus.MetricsCollector{
			&prometheus.Postgres{
				VariableNames: []string{"Threads_running"},
			},
		}, // user defined metrics
	})); err != nil {
		zap.L().Error("❌ Failed to register db metrics", zap.Error(err))
		return err
	}
	return nil
}

// Helper function to extract DB name from DSN string
func extractDBNameFromDSN(dsn string) string {
	// Split the DSN into parts (space-separated for PostgreSQL, semicolon-separated for MySQL)
	parts := strings.Fields(dsn) // Fields splits by any whitespace (e.g., spaces)
	for _, part := range parts {
		// Look for the "dbname=" parameter and extract the database name
		if strings.HasPrefix(part, "dbname=") {
			return strings.TrimPrefix(part, "dbname=")
		}
	}
	return "unknown"
}

// Function to get the DB name based on the Dialector type (Postgres/MySQL)
func getDBNameFromDialector(dialector gorm.Dialector) string {
	switch d := dialector.(type) {
	case *postgres.Dialector:
		// For PostgreSQL, extract the DB name from DSN
		return extractDBNameFromDSN(d.Config.DSN)
	case *mysql.Dialector:
		// For MySQL, extract the DB name from DSN
		return extractDBNameFromDSN(d.Config.DSN)
	default:
		return "unknown"
	}
}
