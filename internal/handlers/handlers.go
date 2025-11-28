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
	"github.com/vmorsell/global-volume/internal/ratelimit"
	"github.com/vmorsell/global-volume/internal/storage"
	"github.com/vmorsell/global-volume/pkg/model"
	"go.uber.org/zap"
)

const (
	RouteKeyConnect             = "$connect"
	RouteKeyDisconnect          = "$disconnect"
	RouteKeyGetVolume           = "getVolume"
	RouteKeyGetConnectedClients = "getConnectedClientsCount"
	RouteKeyRequestVolumeChange = "reqVolumeChange"

	VolumeMin = 0
	VolumeMax = 100

	MaxRequestBodySize    = 1024
	MaxGlobalConnections  = 10
	VolumeChangeRateLimit = 2

	ErrInvalidPayload   = "invalid payload, expected {\"volume\": int}"
	ErrVolumeOutOfRange = "volume must be between 0 and 100"
	ErrInvalidRoute     = "invalid route"
	ErrRateLimited      = "rate limit exceeded"
	ErrConnectionLimit  = "connection limit exceeded"
	ErrMessageTooLarge  = "request body too large"
)

type Handler struct {
	logger                *zap.Logger
	awsConfig             aws.Config
	storage               *storage.Storage
	volumeChangeRateLimit *ratelimit.RateLimiter
}

func NewHandler(logger *zap.Logger, awsConfig aws.Config, storage *storage.Storage) *Handler {
	return &Handler{
		logger:                logger,
		awsConfig:             awsConfig,
		storage:               storage,
		volumeChangeRateLimit: ratelimit.NewRateLimiter(VolumeChangeRateLimit, ratelimit.DefaultWindowSize),
	}
}

func (h *Handler) HandleRequest(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	routeKey := req.RequestContext.RouteKey

	switch routeKey {
	case RouteKeyConnect:
		return h.handleConnect(ctx, req)
	case RouteKeyDisconnect:
		return h.handleDisconnect(ctx, req)
	case RouteKeyGetVolume:
		return h.handleGetVolume(ctx, req)
	case RouteKeyGetConnectedClients:
		return h.handleGetConnectedClientsCount(ctx, req)
	case RouteKeyRequestVolumeChange:
		return h.handleRequestVolumeChange(ctx, req)
	default:
		h.logger.Warn("unknown route key", zap.String("routeKey", routeKey))
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       ErrInvalidRoute,
		}, nil
	}
}

func (h *Handler) handleConnect(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionID := req.RequestContext.ConnectionID
	sourceIP := h.getSourceIP(req)

	connectionIDs, err := h.storage.GetConnections(ctx)
	if err != nil {
		h.logger.Error("failed to get connections for limit check", zap.Error(err))
	} else if len(connectionIDs) >= MaxGlobalConnections {
		h.logger.Warn("connection limit reached",
			zap.String("connectionID", connectionID),
			zap.Int("current", len(connectionIDs)),
			zap.Int("max", MaxGlobalConnections))
		return events.APIGatewayProxyResponse{
			StatusCode: 503,
			Body:       fmt.Sprintf("%s: maximum connections reached (%d)", ErrConnectionLimit, MaxGlobalConnections),
		}, nil
	}

	connectionIDs, err = h.storage.AddConnection(ctx, connectionID, sourceIP)
	if err != nil {
		h.logger.Error("failed to add connection", zap.String("connectionID", connectionID), zap.Error(err))
		return h.errorResponse(500, "failed to add connection"), nil
	}

	clientCount := len(connectionIDs)
	message := model.ConnectedClientsMessage{
		Type:    model.MessageTypeConnectedClients,
		Clients: clientCount,
	}
	if err := h.broadcastToOthers(ctx, req, connectionIDs, connectionID, message); err != nil {
		h.logger.Error("failed to broadcast client count", zap.Error(err))
	}

	return h.successResponse(), nil
}

func (h *Handler) handleDisconnect(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionID := req.RequestContext.ConnectionID

	if err := h.storage.DeleteConnection(ctx, connectionID, "disconnect"); err != nil {
		h.logger.Error("failed to delete connection", zap.String("connectionID", connectionID), zap.Error(err))
		return h.errorResponse(500, "failed to delete connection"), nil
	}

	connectionIDs, err := h.storage.GetConnections(ctx)
	if err != nil {
		h.logger.Error("failed to get connections", zap.Error(err))
		return h.errorResponse(500, "failed to get connections"), nil
	}

	message := model.ConnectedClientsMessage{
		Type:    model.MessageTypeConnectedClients,
		Clients: len(connectionIDs),
	}
	if err := h.broadcastToOthers(ctx, req, connectionIDs, "", message); err != nil {
		h.logger.Error("failed to broadcast", zap.Error(err))
	}

	return h.successResponse(), nil
}

func (h *Handler) handleGetVolume(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	volume, err := h.storage.GetVolume(ctx)
	if err != nil {
		h.logger.Error("failed to get volume", zap.Error(err))
		return h.errorResponse(500, "failed to get volume"), nil
	}

	message := model.VolumeMessage{
		Type:   model.MessageTypeVolume,
		Volume: volume,
	}

	if err := h.sendToConnection(ctx, req, req.RequestContext.ConnectionID, message); err != nil {
		h.logger.Error("failed to send volume to connection",
			zap.String("connectionID", req.RequestContext.ConnectionID),
			zap.Error(err))
		return h.errorResponse(500, "failed to send volume"), nil
	}

	return h.successResponse(), nil
}

func (h *Handler) handleGetConnectedClientsCount(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionIDs, err := h.storage.GetConnections(ctx)
	if err != nil {
		h.logger.Error("failed to get connections", zap.Error(err))
		return h.errorResponse(500, "failed to get connections"), nil
	}

	message := model.ConnectedClientsMessage{
		Type:    model.MessageTypeConnectedClients,
		Clients: len(connectionIDs),
	}

	if err := h.sendToConnection(ctx, req, req.RequestContext.ConnectionID, message); err != nil {
		h.logger.Error("failed to send client count", zap.Error(err))
		return h.errorResponse(500, "failed to send client count"), nil
	}

	return h.successResponse(), nil
}

func (h *Handler) handleRequestVolumeChange(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionID := req.RequestContext.ConnectionID

	if len(req.Body) > MaxRequestBodySize {
		return h.errorResponse(400, ErrMessageTooLarge), nil
	}

	if !h.volumeChangeRateLimit.Allow(connectionID) {
		return h.errorResponse(429, ErrRateLimited), nil
	}

	var body volumeChangeRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       ErrInvalidPayload,
		}, nil
	}

	if err := validateVolume(body.Volume); err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       err.Error(),
		}, nil
	}

	requestTimestamp := req.RequestContext.RequestTimeEpoch
	written, err := h.storage.SaveVolume(ctx, body.Volume, requestTimestamp)
	if err != nil {
		h.logger.Error("failed to save volume", zap.Error(err))
		return h.errorResponse(500, "failed to save volume"), nil
	}

	if !written {
		return h.successResponse(), nil
	}

	connectionIDs, err := h.storage.GetConnections(ctx)
	if err != nil {
		h.logger.Error("failed to get connections", zap.Error(err))
		return h.errorResponse(500, "failed to get connections"), nil
	}

	message := model.VolumeMessage{
		Type:   model.MessageTypeVolume,
		Volume: body.Volume,
	}
	if err := h.broadcastToOthers(ctx, req, connectionIDs, connectionID, message); err != nil {
		h.logger.Error("failed to broadcast", zap.Error(err))
	}

	return h.successResponse(), nil
}

func (h *Handler) broadcastToOthers(ctx context.Context, req events.APIGatewayWebsocketProxyRequest, connectionIDs []string, excludeConnectionID string, message interface{}) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	apiClient := h.newAPIClient(req.RequestContext.APIID, req.RequestContext.Stage)

	for _, connID := range connectionIDs {
		if connID == excludeConnectionID {
			continue
		}

		if err := h.sendToConnectionWithClient(ctx, apiClient, connID, payload); err != nil {
			var gone *types.GoneException
			if errors.As(err, &gone) {
				_ = h.storage.DeleteConnection(ctx, connID, "gone")
				continue
			}
			h.logger.Error("failed to send", zap.String("connectionID", connID), zap.Error(err))
		}
	}

	return nil
}

func (h *Handler) sendToConnection(ctx context.Context, req events.APIGatewayWebsocketProxyRequest, connectionID string, message interface{}) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	apiClient := h.newAPIClient(req.RequestContext.APIID, req.RequestContext.Stage)
	return h.sendToConnectionWithClient(ctx, apiClient, connectionID, payload)
}

func (h *Handler) sendToConnectionWithClient(ctx context.Context, apiClient *apigatewaymanagementapi.Client, connectionID string, payload []byte) error {
	_, err := apiClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: &connectionID,
		Data:         payload,
	})
	if err != nil {
		return fmt.Errorf("post to connection %s: %w", connectionID, err)
	}
	return nil
}

func (h *Handler) newAPIClient(apiID, stage string) *apigatewaymanagementapi.Client {
	endpoint := fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/%s", apiID, h.awsConfig.Region, stage)
	return apigatewaymanagementapi.NewFromConfig(
		h.awsConfig,
		func(o *apigatewaymanagementapi.Options) {
			o.EndpointResolver = apigatewaymanagementapi.EndpointResolverFromURL(endpoint)
		},
		apigatewaymanagementapi.WithSigV4SigningName("execute-api"),
		apigatewaymanagementapi.WithSigV4SigningRegion(h.awsConfig.Region),
	)
}

func (h *Handler) getSourceIP(req events.APIGatewayWebsocketProxyRequest) string {
	if req.RequestContext.Identity.SourceIP != "" {
		return req.RequestContext.Identity.SourceIP
	}
	return req.RequestContext.ConnectionID
}

func validateVolume(volume int) error {
	if volume < VolumeMin || volume > VolumeMax {
		return fmt.Errorf("%s", ErrVolumeOutOfRange)
	}
	return nil
}

type volumeChangeRequest struct {
	Volume int `json:"volume"`
}

func (h *Handler) successResponse() events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{StatusCode: 200}
}

func (h *Handler) errorResponse(statusCode int, message string) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Body:       message,
	}
}
