package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Server   ServerConfig
	Logger   LoggerConfig
	Postgres PostgresConfig
	Kafka    KafkaConfig
	Services ServicesConfig
}

type ServerConfig struct {
	AppEnv   string
	GRPCPort string
}

type LoggerConfig struct {
	Level             string
	Encoding          string
	DisableCaller     bool
	DisableStacktrace bool
}

type PostgresConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int
	ConnMaxIdleTime int
}

type KafkaConfig struct {
	Brokers []string
	Topic   string
}

type ServicesConfig struct {
	ProductGRPCAddr  string
	CustomerGRPCAddr string
	StoreGRPCAddr    string
}

func LoadEnv() *Config {
	return &Config{
		Server: ServerConfig{
			AppEnv:   getEnv("APP_ENV", "dev"),
			GRPCPort: getEnv("GRPC_PORT", ":8083"),
		},
		Logger: LoggerConfig{
			Level:             getEnv("LOGGER_LEVEL", "debug"),
			Encoding:          getEnv("LOGGER_ENCODING", "console"),
			DisableCaller:     getEnvBool("LOGGER_DISABLE_CALLER", false),
			DisableStacktrace: getEnvBool("LOGGER_DISABLE_STACKTRACE", true),
		},
		Postgres: PostgresConfig{
			Host:            getEnv("POSTGRES_HOST", "localhost"),
			Port:            getEnv("POSTGRES_PORT", "5433"),
			User:            getEnv("POSTGRES_USER", "omnipos"),
			Password:        getEnv("POSTGRES_PASSWORD", "omnipos"),
			DBName:          getEnv("POSTGRES_DB", "omnipos_order_db"),
			SSLMode:         getEnv("POSTGRES_SSLMODE", "disable"),
			MaxOpenConns:    getEnvInt("POSTGRES_MAX_OPEN_CONNS", 10),
			MaxIdleConns:    getEnvInt("POSTGRES_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvInt("POSTGRES_CONN_MAX_LIFETIME", 300),
			ConnMaxIdleTime: getEnvInt("POSTGRES_CONN_MAX_IDLE_TIME", 60),
		},
		Kafka: KafkaConfig{
			Brokers: getEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
			Topic:   getEnv("KAFKA_TOPIC_ORDERS", "orders.events"),
		},
		Services: ServicesConfig{
			ProductGRPCAddr:  getEnv("PRODUCT_GRPC_ADDR", ":50051"),
			CustomerGRPCAddr: getEnv("CUSTOMER_GRPC_ADDR", ":50052"),
			StoreGRPCAddr:    getEnv("STORE_GRPC_ADDR", ":50053"),
		},
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvSlice(key string, fallback []string) []string {
	if value, ok := os.LookupEnv(key); ok {
		return strings.Split(value, ",")
	}
	return fallback
}
