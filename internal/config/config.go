package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Redis    RedisConfig
	Worker   WorkerConfig
	Queue    QueueConfig
	Metrics  MetricsConfig
	Auth     AuthConfig
	LogLevel string
}

type ServerConfig struct {
	Host         string
	Port         int
	AdminPort    int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type WorkerConfig struct {
	ID                string
	Concurrency       int
	HeartbeatInterval time.Duration
	HeartbeatTimeout  time.Duration
	ShutdownTimeout   time.Duration
}

type QueueConfig struct {
	StreamPrefix        string
	ConsumerGroup       string
	MaxQueueSize        int64
	BlockTimeout        time.Duration
	ClaimMinIdle        time.Duration
	RecoveryInterval    time.Duration
	RetryMaxAttempts    int
	RetryInitialBackoff time.Duration
	RetryMaxBackoff     time.Duration
	RetryBackoffFactor  float64
	TaskRetentionDays   int
	RateLimitRPS        int
}

type MetricsConfig struct {
	Enabled bool
	Path    string
}

type AuthConfig struct {
	Enabled   bool
	JWTSecret string
	APIKeys   []string
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/taskqueue")

	// Set defaults
	setDefaults()

	// Environment variable binding
	viper.SetEnvPrefix("TASKQUEUE")
	viper.AutomaticEnv()

	// Read config file (optional)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.adminport", 8081)
	viper.SetDefault("server.readtimeout", 30*time.Second)
	viper.SetDefault("server.writetimeout", 30*time.Second)
	viper.SetDefault("server.idletimeout", 120*time.Second)

	// Redis defaults
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.poolsize", 100)
	viper.SetDefault("redis.minidleconns", 10)
	viper.SetDefault("redis.maxretries", 3)
	viper.SetDefault("redis.dialtimeout", 5*time.Second)
	viper.SetDefault("redis.readtimeout", 3*time.Second)
	viper.SetDefault("redis.writetimeout", 3*time.Second)

	// Worker defaults
	viper.SetDefault("worker.id", "")
	viper.SetDefault("worker.concurrency", 10)
	viper.SetDefault("worker.heartbeatinterval", 5*time.Second)
	viper.SetDefault("worker.heartbeattimeout", 15*time.Second)
	viper.SetDefault("worker.shutdowntimeout", 30*time.Second)

	// Queue defaults
	viper.SetDefault("queue.streamprefix", "tasks")
	viper.SetDefault("queue.consumergroup", "workers")
	viper.SetDefault("queue.maxqueuesize", 1000000)
	viper.SetDefault("queue.blocktimeout", 5*time.Second)
	viper.SetDefault("queue.claimminidle", 30*time.Second)
	viper.SetDefault("queue.recoveryinterval", 10*time.Second)
	viper.SetDefault("queue.retrymaxattempts", 3)
	viper.SetDefault("queue.retryinitialbackoff", 1*time.Second)
	viper.SetDefault("queue.retrymaxbackoff", 5*time.Minute)
	viper.SetDefault("queue.retrybackofffactor", 2.0)
	viper.SetDefault("queue.taskretentiondays", 7)
	viper.SetDefault("queue.ratelimitrps", 1000)

	// Metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.path", "/metrics")

	// Auth defaults
	viper.SetDefault("auth.enabled", false)
	viper.SetDefault("auth.jwtsecret", "")
	viper.SetDefault("auth.apikeys", []string{})

	// Logging defaults
	viper.SetDefault("loglevel", "info")
}
