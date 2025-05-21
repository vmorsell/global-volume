package connstorage

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

const (
	pk = "pk"

	connsDocPK = "connections"
)

type ConnectionStorage struct {
	Logger    *zap.Logger
	Client    *dynamodb.Client
	TableName string
}

func (s *ConnectionStorage) SaveConnection(ctx context.Context, id string) error {
	_, err := s.Client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.TableName,
		Key: map[string]types.AttributeValue{
			pk: &types.AttributeValueMemberS{Value: connsDocPK},
		},
		UpdateExpression: aws.String("ADD connectionIds :id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":id": &types.AttributeValueMemberSS{Value: []string{id}},
		},
	})
	if err != nil {
		return fmt.Errorf("update item: %w", err)
	}
	return nil
}

func (s *ConnectionStorage) DeleteConnection(ctx context.Context, id string) error {
	_, err := s.Client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.TableName,
		Key: map[string]types.AttributeValue{
			pk: &types.AttributeValueMemberS{Value: connsDocPK},
		},
		UpdateExpression: aws.String("DELETE connectionIds :id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":id": &types.AttributeValueMemberSS{Value: []string{id}},
		},
	})
	if err != nil {
		return fmt.Errorf("update item: %w", err)
	}
	return nil
}

func (s *ConnectionStorage) ListConnections(ctx context.Context) ([]string, error) {
	result, err := s.Client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.TableName,
		Key: map[string]types.AttributeValue{
			pk: &types.AttributeValueMemberS{Value: connsDocPK},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}

	var connections struct {
		ConnectionIds []string `dynamodbav:"connectionIds"`
	}
	if err := attributevalue.UnmarshalMap(result.Item, &connections); err != nil {
		return nil, fmt.Errorf("unmarshal map: %w", err)
	}

	return connections.ConnectionIds, nil
}
