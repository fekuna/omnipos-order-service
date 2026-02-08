package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fekuna/omnipos-order-service/config"
	"github.com/fekuna/omnipos-order-service/internal/middleware"
	"github.com/fekuna/omnipos-pkg/broker"
	"github.com/fekuna/omnipos-pkg/database/postgres"
	"github.com/fekuna/omnipos-pkg/logger"
	orderv1 "github.com/fekuna/omnipos-proto/gen/go/omnipos/order/v1"
	productv1 "github.com/fekuna/omnipos-proto/gen/go/omnipos/product/v1"

	orderH "github.com/fekuna/omnipos-order-service/internal/order/handler"
	orderRepoPkg "github.com/fekuna/omnipos-order-service/internal/order/repository"
	orderUCPkg "github.com/fekuna/omnipos-order-service/internal/order/usecase"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// 1. Load Configuration
	_ = godotenv.Load()
	cfg := config.LoadEnv()

	// 2. Initialize Logger
	logConfig := &logger.ZapLoggerConfig{
		IsDevelopment:     false,
		Encoding:          "json",
		Level:             "info",
		DisableCaller:     false,
		DisableStacktrace: false,
	}

	if cfg.Server.AppEnv == "development" {
		logConfig.IsDevelopment = true
		logConfig.Encoding = "console"
		logConfig.Level = "debug"
	}

	appLogger := logger.NewZapLogger(logConfig)
	defer appLogger.Sync()

	// 3. Connect to Database
	db, err := postgres.NewPostgres(&postgres.Config{
		Host:            cfg.Postgres.Host,
		Port:            cfg.Postgres.Port,
		User:            cfg.Postgres.User,
		Password:        cfg.Postgres.Password,
		DBName:          cfg.Postgres.DBName,
		SSLMode:         cfg.Postgres.SSLMode,
		MaxOpenConns:    cfg.Postgres.MaxOpenConns,
		MaxIdleConns:    cfg.Postgres.MaxIdleConns,
		ConnMaxLifetime: time.Duration(cfg.Postgres.ConnMaxLifetime) * time.Second,
		ConnMaxIdleTime: time.Duration(cfg.Postgres.ConnMaxIdleTime) * time.Second,
	})
	if err != nil {
		appLogger.Fatal("Could not connect to database", zap.Error(err))
	}
	defer db.Close()
	appLogger.Info("Connected to PostgreSQL database", zap.String("db_name", cfg.Postgres.DBName))

	// 4. Initialize Repository
	orderRepo := orderRepoPkg.NewPostgresRepository(db)

	// 4.5 Initialize Kafka Producer
	kafkaProducer := broker.NewProducer(&broker.Config{
		Brokers: cfg.Kafka.Brokers,
		Topic:   cfg.Kafka.Topic,
	})
	defer kafkaProducer.Close()
	appLogger.Info("Connected to Kafka", zap.Strings("brokers", cfg.Kafka.Brokers), zap.String("topic", cfg.Kafka.Topic))

	// 5. Connect to Product Service
	productConn, err := grpc.Dial(cfg.Services.ProductGRPCAddr, grpc.WithInsecure())
	if err != nil {
		appLogger.Fatal("Failed to connect to product service", zap.Error(err))
	}
	defer productConn.Close()
	productClient := productv1.NewProductServiceClient(productConn)

	// 6. Initialize UseCase
	orderUC := orderUCPkg.NewOrderUseCase(orderRepo, kafkaProducer, productClient)

	// 6. Initialize Handler
	orderHandler := orderH.NewOrderHandler(orderUC)

	// 6.5 Initialize Middleware
	authInterceptor := middleware.NewAuthContextInterceptor(appLogger)
	rbacInterceptor := middleware.NewRBACInterceptor(appLogger)

	// 7. Start gRPC Server
	port := cfg.Server.GRPCPort
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			authInterceptor.Unary(),
			rbacInterceptor.Unary(),
		),
	)

	// Register Services
	orderv1.RegisterOrderServiceServer(grpcServer, orderHandler)

	// Register Reflection
	reflection.Register(grpcServer)

	appLogger.Info("Starting gRPC server", zap.String("port", port))

	// Graceful Shutdown
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			appLogger.Fatal("failed to serve", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")
	grpcServer.GracefulStop()
	appLogger.Info("Server stopped")
}
