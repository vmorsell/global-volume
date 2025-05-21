package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
	"github.com/vmorsell/global-volume/internal/connstorage"
	"github.com/vmorsell/global-volume/pkg/model"
	"go.uber.org/zap"
)

type Handler struct {
	Logger      *zap.Logger
	AWSConfig   aws.Config
	ConnStorage *connstorage.ConnectionStorage
}

func (h *Handler) ConnectHandler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	id := req.RequestContext.ConnectionID
	if err := h.ConnStorage.SaveConnection(ctx, id); err != nil {
		h.Logger.Error("save connection", zap.Error(err), zap.String("id", id))
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
		}, err
	}
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
	}, nil
}

func (h *Handler) DisconnectHandler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	id := req.RequestContext.ConnectionID
	if err := h.ConnStorage.DeleteConnection(ctx, id); err != nil {
		h.Logger.Error("delete connection", zap.Error(err), zap.String("id", id))
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
		}, err
	}
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
	}, nil
}

func (h *Handler) GetStateHandler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	conns, err := h.ConnStorage.ListConnections(ctx)
	if err != nil {
		h.Logger.Error("list connections", zap.Error(err))
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
		}, err
	}

	payload, _ := json.Marshal(model.State{
		Users: len(conns),
	})

	apiClient := newApiClient(h.AWSConfig, req.RequestContext.DomainName, req.RequestContext.Stage)
	_, err = apiClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: &req.RequestContext.ConnectionID,
		Data:         payload,
	})
	if err != nil {
		h.Logger.Error("post to connection", zap.Error(err), zap.String("id", req.RequestContext.ConnectionID))
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
		}, err
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
	}, nil
}

type ReqVolumeChangeBody struct {
	Volume int `json:"volume"`
}

func (h *Handler) ReqVolumeChangeHandler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	var body ReqVolumeChangeBody
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       "invalid payload, expected {\"volume\": int}",
		}, nil
	}

	vol := body.Volume
	if vol < 0 || vol > 100 {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       "volume must be between 0 and 100",
		}, nil
	}

	conns, err := h.ConnStorage.ListConnections(ctx)
	if err != nil {
		h.Logger.Error("list connections", zap.Error(err))
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
		}, err
	}

	payload, _ := json.Marshal(model.State{
		Volume: vol,
		Users:  len(conns),
	})

	apiClient := newApiClient(h.AWSConfig, req.RequestContext.DomainName, req.RequestContext.Stage)
	for _, id := range conns {
		_, err := apiClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: &id,
			Data:         payload,
		})
		if err != nil {
			var gone *types.GoneException
			if errors.As(err, &gone) {
				if delErr := h.ConnStorage.DeleteConnection(ctx, id); delErr != nil {
					h.Logger.Warn("cleanup stale connection", zap.Error(delErr), zap.String("id", id))
				}
				continue
			}
			h.Logger.Error("post to connection", zap.Error(err), zap.String("id", id))
		}
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
	}, nil
}

func newApiClient(config aws.Config, domainName, stage string) *apigatewaymanagementapi.Client {
	endpoint := fmt.Sprintf("https://%s/%s", domainName, stage)
	client := apigatewaymanagementapi.NewFromConfig(
		config, func(o *apigatewaymanagementapi.Options) {
			o.EndpointResolver = apigatewaymanagementapi.EndpointResolverFromURL(endpoint)
		},
		apigatewaymanagementapi.WithSigV4SigningName("execute-api"),
		apigatewaymanagementapi.WithSigV4SigningRegion(config.Region),
	)

	return client
}
