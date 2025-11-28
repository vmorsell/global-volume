package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/vmorsell/global-volume/pkg/model"
	"go.uber.org/zap"
)

const (
	partitionKey = "pk"

	documentKeyVolume      = "volume"
	documentKeyConnections = "connections"

	maxDeleteRetries         = 3
	dynamoDBOperationTimeout = 5 * time.Second
)

var (
	// ErrItemNotFound is returned when a DynamoDB item doesn't exist.
	ErrItemNotFound = errors.New("item not found")
	// ErrConnectionNotFound is returned when attempting to delete a non-existent connection.
	ErrConnectionNotFound = errors.New("connection not found")
)

type Storage struct {
	logger    *zap.Logger
	client    *dynamodb.Client
	tableName string
}

func NewStorage(logger *zap.Logger, client *dynamodb.Client, tableName string) *Storage {
	return &Storage{
		logger:    logger,
		client:    client,
		tableName: tableName,
	}
}

func (s *Storage) SaveVolume(ctx context.Context, volume int, requestTimestamp int64) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, dynamoDBOperationTimeout)
	defer cancel()

	key := s.volumeKey()

	updateExpr := "SET volume = :volume, #ts = :timestamp"
	conditionExpr := "attribute_not_exists(#ts) OR #ts < :timestamp"

	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           &s.tableName,
		Key:                 key,
		UpdateExpression:    aws.String(updateExpr),
		ConditionExpression: aws.String(conditionExpr),
		ExpressionAttributeNames: map[string]string{
			"#ts": "timestamp",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":volume":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", volume)},
			":timestamp": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", requestTimestamp)},
		},
	})

	if err != nil {
		var condCheckErr *types.ConditionalCheckFailedException
		if errors.As(err, &condCheckErr) {
			return false, nil
		}
		return false, fmt.Errorf("update volume item: %w", err)
	}

	return true, nil
}

func (s *Storage) GetVolume(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, dynamoDBOperationTimeout)
	defer cancel()

	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.tableName,
		Key:       s.volumeKey(),
	})
	if err != nil {
		return 0, fmt.Errorf("get volume item: %w", err)
	}

	if len(result.Item) == 0 {
		return 0, nil
	}

	var volumeState model.VolumeState
	if err := attributevalue.UnmarshalMap(result.Item, &volumeState); err != nil {
		return 0, fmt.Errorf("unmarshal volume state: %w", err)
	}

	return volumeState.Volume, nil
}

func (s *Storage) GetVolumeWithTimestamp(ctx context.Context) (int, int64, error) {
	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.tableName,
		Key:       s.volumeKey(),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("get volume item: %w", err)
	}

	if len(result.Item) == 0 {
		return 0, 0, nil
	}

	var volumeState model.VolumeState
	if err := attributevalue.UnmarshalMap(result.Item, &volumeState); err != nil {
		return 0, 0, fmt.Errorf("unmarshal volume state: %w", err)
	}

	return volumeState.Volume, volumeState.Timestamp, nil
}

func (s *Storage) GetConnections(ctx context.Context) ([]string, error) {
	connections, err := s.GetConnectionsWithIPs(ctx)
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(connections))
	for i, conn := range connections {
		ids[i] = conn.ConnectionID
	}
	return ids, nil
}

func (s *Storage) GetConnectionsWithIPs(ctx context.Context) ([]model.ConnectionInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, dynamoDBOperationTimeout)
	defer cancel()

	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      &s.tableName,
		ConsistentRead: aws.Bool(true),
		Key:            s.connectionsKey(),
	})
	if err != nil {
		return nil, fmt.Errorf("get connections item: %w", err)
	}

	if len(result.Item) == 0 {
		return []model.ConnectionInfo{}, nil
	}

	var connectionsState model.ConnectionsState
	if err := attributevalue.UnmarshalMap(result.Item, &connectionsState); err != nil {
		var oldState struct {
			ConnectionIDs []string `dynamodbav:"connectionIds"`
		}
		if oldErr := attributevalue.UnmarshalMap(result.Item, &oldState); oldErr == nil {
			connections := make([]model.ConnectionInfo, len(oldState.ConnectionIDs))
			for i, id := range oldState.ConnectionIDs {
				connections[i] = model.ConnectionInfo{ConnectionID: id, SourceIP: ""}
			}
			return connections, nil
		}
		return nil, fmt.Errorf("unmarshal connections state: %w", err)
	}

	if connectionsState.Connections == nil {
		connectionsState.Connections = []model.ConnectionInfo{}
	}

	return connectionsState.Connections, nil
}

func (s *Storage) CountConnectionsPerIP(ctx context.Context, sourceIP string) (int, error) {
	connections, err := s.GetConnectionsWithIPs(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, conn := range connections {
		if conn.SourceIP == sourceIP {
			count++
		}
	}
	return count, nil
}

func (s *Storage) AddConnection(ctx context.Context, connectionID string, sourceIP string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, dynamoDBOperationTimeout)
	defer cancel()

	currentConnections, err := s.GetConnectionsWithIPs(ctx)
	if err != nil {
		return nil, fmt.Errorf("get current connections: %w", err)
	}

	for _, conn := range currentConnections {
		if conn.ConnectionID == connectionID {
			return s.GetConnections(ctx)
		}
	}

	newConnections := append(currentConnections, model.ConnectionInfo{
		ConnectionID: connectionID,
		SourceIP:     sourceIP,
	})

	newState := model.ConnectionsState{Connections: newConnections}
	item, err := attributevalue.MarshalMap(newState)
	if err != nil {
		return nil, fmt.Errorf("marshal connections state: %w", err)
	}
	item[partitionKey] = &types.AttributeValueMemberS{Value: documentKeyConnections}

	currentState := model.ConnectionsState{Connections: currentConnections}
	currentItem, err := attributevalue.MarshalMap(currentState)
	if err != nil {
		return nil, fmt.Errorf("marshal current connections state: %w", err)
	}

	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &s.tableName,
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(connections) OR connections = :currentConnections"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":currentConnections": currentItem["connections"],
		},
	})

	if err != nil {
		var condCheckErr *types.ConditionalCheckFailedException
		if errors.As(err, &condCheckErr) {
			const maxRetries = 3
			for attempt := 1; attempt <= maxRetries; attempt++ {
				currentConnections, retryErr := s.GetConnectionsWithIPs(ctx)
				if retryErr != nil {
					return nil, fmt.Errorf("get connections for retry: %w", retryErr)
				}

				for _, conn := range currentConnections {
					if conn.ConnectionID == connectionID {
						return s.GetConnections(ctx)
					}
				}

				newConnections := append(currentConnections, model.ConnectionInfo{
					ConnectionID: connectionID,
					SourceIP:     sourceIP,
				})

				newState := model.ConnectionsState{Connections: newConnections}
				item, retryErr := attributevalue.MarshalMap(newState)
				if retryErr != nil {
					return nil, fmt.Errorf("marshal connections state: %w", retryErr)
				}
				item[partitionKey] = &types.AttributeValueMemberS{Value: documentKeyConnections}

				currentState := model.ConnectionsState{Connections: currentConnections}
				currentItem, retryErr := attributevalue.MarshalMap(currentState)
				if retryErr != nil {
					return nil, fmt.Errorf("marshal current connections state: %w", retryErr)
				}

				_, retryErr = s.client.PutItem(ctx, &dynamodb.PutItemInput{
					TableName:           &s.tableName,
					Item:                item,
					ConditionExpression: aws.String("attribute_not_exists(connections) OR connections = :currentConnections"),
					ExpressionAttributeValues: map[string]types.AttributeValue{
						":currentConnections": currentItem["connections"],
					},
				})

				if retryErr == nil {
					return s.GetConnections(ctx)
				}

				if !errors.As(retryErr, &condCheckErr) {
					return nil, fmt.Errorf("put item: %w", retryErr)
				}
			}
			return nil, fmt.Errorf("failed to add connection after %d retries: concurrent updates", maxRetries)
		}
		return nil, fmt.Errorf("put item: %w", err)
	}

	return s.GetConnections(ctx)
}

func (s *Storage) DeleteConnection(ctx context.Context, connectionID string, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, dynamoDBOperationTimeout*time.Duration(maxDeleteRetries))
	defer cancel()

	for attempt := 0; attempt < maxDeleteRetries; attempt++ {
		connections, err := s.getConnectionsForUpdate(ctx)
		if err != nil {
			return fmt.Errorf("get connections for update: %w", err)
		}

		if len(connections) == 0 {
			return nil
		}

		index := s.findConnectionIndex(connections, connectionID)
		if index == -1 {
			return nil
		}

		newConnections := s.removeConnectionAtIndex(connections, index)

		if err := s.updateConnectionsWithCondition(ctx, connections, newConnections); err != nil {
			var condCheckErr *types.ConditionalCheckFailedException
			if errors.As(err, &condCheckErr) {
				if attempt < maxDeleteRetries-1 {
					continue
				}
				return fmt.Errorf("failed to delete connection after %d retries: state changed", maxDeleteRetries)
			}
			return fmt.Errorf("update item: %w", err)
		}

		return nil
	}

	return fmt.Errorf("failed to delete connection after %d attempts", maxDeleteRetries)
}

func (s *Storage) volumeKey() map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		partitionKey: &types.AttributeValueMemberS{Value: documentKeyVolume},
	}
}

func (s *Storage) connectionsKey() map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		partitionKey: &types.AttributeValueMemberS{Value: documentKeyConnections},
	}
}

func (s *Storage) getConnectionsForUpdate(ctx context.Context) ([]model.ConnectionInfo, error) {
	return s.GetConnectionsWithIPs(ctx)
}

func (s *Storage) findConnectionIndex(connections []model.ConnectionInfo, connectionID string) int {
	for i, conn := range connections {
		if conn.ConnectionID == connectionID {
			return i
		}
	}
	return -1
}

func (s *Storage) removeConnectionAtIndex(connections []model.ConnectionInfo, index int) []model.ConnectionInfo {
	newList := make([]model.ConnectionInfo, 0, len(connections)-1)
	newList = append(newList, connections[:index]...)
	newList = append(newList, connections[index+1:]...)
	return newList
}

func (s *Storage) updateConnectionsWithCondition(ctx context.Context, currentList, newList []model.ConnectionInfo) error {
	newState := model.ConnectionsState{Connections: newList}
	newItem, err := attributevalue.MarshalMap(newState)
	if err != nil {
		return fmt.Errorf("marshal new connections: %w", err)
	}
	newItem[partitionKey] = &types.AttributeValueMemberS{Value: documentKeyConnections}

	currentState := model.ConnectionsState{Connections: currentList}
	currentItem, err := attributevalue.MarshalMap(currentState)
	if err != nil {
		return fmt.Errorf("marshal current connections: %w", err)
	}

	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &s.tableName,
		Item:                newItem,
		ConditionExpression: aws.String("connections = :currentList"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":currentList": currentItem["connections"],
		},
	})

	return err
}
