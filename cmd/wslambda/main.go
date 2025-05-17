package main

import (
	"context"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/vmorsell/global-volume/internal/connstorage"
	"github.com/vmorsell/global-volume/internal/handlers"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	tableName := os.Getenv("CONNECTIONS_TABLE")
	if tableName == "" {
		logger.Fatal("CONNECTIONS_TABLE is not set")
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		logger.Fatal("failed to load AWS config", zap.Error(err))
	}
	dynamoClient := dynamodb.NewFromConfig(cfg)

	connStorage := &connstorage.ConnectionStorage{
		Logger:    logger,
		Client:    dynamoClient,
		TableName: tableName,
	}

	h := &handlers.Handler{
		Logger:      logger,
		AWSConfig:   cfg,
		ConnStorage: connStorage,
	}

	router := func(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (interface{}, error) {
		switch req.RequestContext.RouteKey {
		case "$connect":
			return h.ConnectHandler(ctx, req)
		case "$disconnect":
			return h.DisconnectHandler(ctx, req)
		case "broadcast":
			return h.BroadcastHandler(ctx, req)
		default:
			return events.APIGatewayProxyResponse{
				StatusCode: 400,
				Body:       "invalid route",
			}, nil
		}
	}

	lambda.Start(router)
}
