package config

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hashicorp/vault-client-go"
	"github.com/spf13/viper"
	_ "github.com/spf13/viper/remote"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	config       = viper.New()
	configHolder atomic.Value
	backend      = "consul"
	backendAddr  = "127.0.0.1:8500"
	backendPath  = "development" // e.g., app/<env>/<service_name>
	configType   = "yaml"
)

type Config struct {
	Platform struct {
		ID          string `mapstructure:"TENANT_ID"`
		Name        string `mapstructure:"TENANT_NAME"`
		CountryCode string `mapstructure:"COUNTRY_CODE"`
		Timezone    string `mapstructure:"TIMEZONE"`
		Domain      string `mapstructure:"DOMAIN"`
	} `mapstructure:"PLATFORM"`
	RootDomain   string `mapstructure:"ROOT_DOMAIN"`
	AppEnv       string `mapstructure:"APP_ENV"`
	AppName      string `mapstructure:"APP_NAME"`
	AppVersion   string `mapstructure:"APP_VERSION"`
	AppNamespace string `mapstructure:"APP_NAMESPACE"`
	TLS          struct {
		Enable   bool   `mapstructure:"ENABLE"`
		CertPath string `mapstructure:"CERT_PATH"`
		KeyPath  string `mapstructure:"KEY_PATH"`
	} `mapstructure:"TLS"`
	Otel struct {
		Addr string `mapstructure:"ADDR"`
	} `mapstructure:"OTEL"`
	Pyroscope struct {
		Addr string `mapstructure:"ADDR"`
	} `mapstructure:"PYROSCOPE"`
	Server struct {
		Addr           string        `mapstructure:"ADDR"`
		ReadTimeout    time.Duration `mapstructure:"READ_TIMEOUT"`
		WriteTimeout   time.Duration `mapstructure:"WRITE_TIMEOUT"`
		IdleTimeout    time.Duration `mapstructure:"IDLE_TIMEOUT"`
		UseUnixSocket  bool          `mapstructure:"USE_UNIX_SOCKET"`
		UnixSocketPath string        `mapstructure:"UNIX_SOCKET_PATH"`
	} `mapstructure:"HTTP_SERVER"`
	Grpc struct {
		Addr string `mapstructure:"ADDR"`
	} `mapstructure:"GRPC_SERVER"`
	Session struct {
		Type   string `mapstructure:"TYPE"`
		Name   string `mapstructure:"NAME"`
		Secret string `mapstructure:"SECRET"`
	} `mapstructure:"SESSION"`
	Database struct {
		Type           string `mapstructure:"TYPE"`
		Host           string `mapstructure:"HOST"`
		Port           string `mapstructure:"PORT"`
		DBNAME         string `mapstructure:"DBNAME"`
		User           string `mapstructure:"USER"`
		Password       string `mapstructure:"PASSWORD"`
		SSLMode        string `mapstructure:"SSLMODE"`
		Timezone       string `mapstructure:"TIMEZONE"`
		ConnectionPool struct {
			MaxIdleConn     int           `mapstructure:"MAX_IDLE_CONN"`
			MaxOpenConn     int           `mapstructure:"MAX_OPEN_CONN"`
			MaxOpenConns    int           `mapstructure:"MAX_OPEN_CONNS"`
			ConnMaxLifetime time.Duration `mapstructure:"CONN_MAX_LIFETIME"`
			ConnMaxIdleTime time.Duration `mapstructure:"CONN_MAX_IDLE_TIME"`
		} `mapstructure:"CONNECTION_POOL"`
	} `mapstructure:"DATABASE"`
	Redis struct {
		Addr        string        `mapstructure:"ADDR"`
		Password    string        `mapstructure:"PASSWORD"`
		DB          int           `mapstructure:"DB"`
		PoolSize    int           `mapstructure:"POOL_SIZE"`
		PoolTimeout time.Duration `mapstructure:"POOL_TIMEOUT"`
	} `mapstructure:"REDIS"`
	Kafka struct {
		Addrs string `mapstructure:"ADDR"`
	}
	AccessControl struct {
		Model  string `mapstructure:"MODEL"`
		Policy string `mapstructure:"POLICY"`
	} `mapstructure:"ACCESS_CONTROL"`
	SecretAES string `mapstructure:"SECRET_AES"`
	Flagsmith struct {
		Addr   string `mapstructure:"ADDR"`
		ApiKey string `mapstructure:"API_KEY"`
	}
	Minio struct {
		Endpoint   string `mapstructure:"ENDPOINT"`
		AccessKey  string `mapstructure:"ACCESS_KEY"`
		SecretKey  string `mapstructure:"SECRET_KEY"`
		Secure     bool   `mapstructure:"SECURE"`
		BucketName string `mapstructure:"BUCKET_NAME"`
	} `mapstructure:"MINIO"`
	Consul struct {
		Addr string `mapstructure:"ADDR"`
	} `mapstructure:"CONSUL"`
	Temporal struct {
		Addr      string `mapstructure:"ADDR"`
		Namespace string `mapstructure:"NAMESPACE"`
	} `mapstructure:"TEMPORAL"`
	RuleEngineURL string `mapstructure:"RULE_ENGINE_URL"`
	LedgerURL     string `mapstructure:"LEDGER_URL"`
}

var Module = fx.Module("config", fx.Provide(LoadConfig))
var RemoteModule = fx.Module("remote.config", fx.Provide(LoadRemote))

type Params struct {
	fx.In
	Vault *vault.Client `optional:"true"`
}

func LoadConfig(p Params) *Config {

	config.SetConfigName("config")
	config.SetConfigType("yaml")
	config.AddConfigPath(".")

	config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	config.AutomaticEnv()

	if err := config.ReadInConfig(); err != nil {
		os.Exit(1)
	}

	var cfg Config
	if err := config.Unmarshal(&cfg); err != nil {
		os.Exit(1)
	}

	if p.Vault != nil {
		// START - Vault
		client := p.Vault
		ctx := context.Background()

		zap.L().Info("Starting Get Secrets", zap.String("path", cfg.AppEnv))
		secret, err := client.Secrets.KvV2Read(ctx, cfg.AppEnv, vault.WithMountPath("secret"))
		if err != nil {
			zap.L().Error("failed get secret from vault", zap.Error(err))
			os.Exit(1)
		}
		zap.L().Info("Success Get Secret")

		get := func(key string) string {
			if val, ok := secret.Data.Data[key].(string); ok {
				return val
			}
			return ""
		}

		cfg.Database.User = get("postgres_user")
		cfg.Database.Password = get("postgres_password")
		cfg.Redis.Password = get("redis_password")
		cfg.SecretAES = get("secret_aes")
		cfg.Flagsmith.ApiKey = get("flagsmith_api_key")
		// END - Vault
	}

	return &cfg
}

func LoadRemote(p Params) *Config {
	if p.Vault == nil {
		zap.L().Error("vault can't provide")
		os.Exit(1)
	}

	if v, ok := os.LookupEnv("REMOTE_CONFIG_PROVIDER"); ok {
		backend = v
	}

	if v, ok := os.LookupEnv("REMOTE_CONFIG_ADDR"); ok {
		backendAddr = v
	}

	if v, ok := os.LookupEnv("REMOTE_CONFIG_PATH"); ok {
		backendPath = v
	}

	config.SetConfigType(configType)
	if err := config.AddRemoteProvider(backend, backendAddr, backendPath); err != nil {
		os.Exit(1)
	}

	if err := config.ReadRemoteConfig(); err != nil {
		os.Exit(1)
	}

	var cfg Config
	if err := config.Unmarshal(&cfg); err != nil {
		os.Exit(1)
	}
	configHolder.Store(&cfg)

	go func() {
		for {
			time.Sleep(time.Second * 5) // delay after each request

			// currently, only tested with etcd support
			if err := config.WatchRemoteConfig(); err != nil {
				zap.L().Error("unable to read remote config", zap.Error(err))
				continue
			}

			// unmarshal new config into our runtime config struct. you can also use channel
			// to implement a signal to notify the system of the changes
			var newcfg Config
			config.Unmarshal(&newcfg)
			configHolder.Store(&newcfg)
		}
	}()

	// START - Vault
	client := p.Vault
	ctx := context.Background()

	zap.L().Info("Starting Get Secrets", zap.String("path", cfg.AppEnv))
	secret, err := client.Secrets.KvV2Read(ctx, cfg.AppEnv, vault.WithMountPath("secret"))
	if err != nil {
		zap.L().Error("failed get secret from vault", zap.Error(err))
		os.Exit(1)
	}
	zap.L().Info("Success Get Secret")

	get := func(key string) string {
		if val, ok := secret.Data.Data[key].(string); ok {
			return val
		}
		return ""
	}

	cfg.Database.User = get("postgres_user")
	cfg.Database.Password = get("postgres_password")
	cfg.Redis.Password = get("redis_password")
	cfg.SecretAES = get("secret_aes")
	cfg.Flagsmith.ApiKey = get("flagsmith_api_key")
	// END - Vault

	return &cfg
}
