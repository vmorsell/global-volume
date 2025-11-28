package main

import (
	"context"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/vmorsell/global-volume/internal/handlers"
	"github.com/vmorsell/global-volume/internal/storage"
	"go.uber.org/zap"
)

const (
	envConnectionsTable = "CONNECTIONS_TABLE"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	tableName := os.Getenv(envConnectionsTable)
	if tableName == "" {
		logger.Fatal("CONNECTIONS_TABLE environment variable is not set")
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		logger.Fatal("failed to load AWS config", zap.Error(err))
	}

	dynamoClient := dynamodb.NewFromConfig(cfg)
	store := storage.NewStorage(logger, dynamoClient, tableName)

	h := handlers.NewHandler(logger, cfg, store)

	router := func(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
		return h.HandleRequest(ctx, req)
	}

	lambda.Start(router)
}
