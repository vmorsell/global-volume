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

	store := &storage.Storage{
		Logger:    logger,
		Client:    dynamoClient,
		TableName: tableName,
	}

	h := &handlers.Handler{
		Logger:    logger,
		AWSConfig: cfg,
		Storage:   store,
	}

	router := func(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (interface{}, error) {
		switch req.RequestContext.RouteKey {
		case "$connect":
			return h.ConnectHandler(ctx, req)
		case "$disconnect":
			return h.DisconnectHandler(ctx, req)
		case "getState":
			return h.GetStateHandler(ctx, req)
		case "reqVolumeChange":
			return h.ReqVolumeChangeHandler(ctx, req)
		default:
			return events.APIGatewayProxyResponse{
				StatusCode: 400,
				Body:       "invalid route",
			}, nil
		}
	}

	lambda.Start(router)
}
