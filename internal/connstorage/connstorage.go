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

const pk = "connectionId"

type Connection struct {
	ConnectionID string `json:"connectionId" dynamodbav:"connectionId"`
}

type ConnectionStorage struct {
	Logger    *zap.Logger
	Client    *dynamodb.Client
	TableName string
}

func (s *ConnectionStorage) SaveConnection(ctx context.Context, id string) error {
	item, err := attributevalue.MarshalMap(Connection{
		ConnectionID: id,
	})
	if err != nil {
		return fmt.Errorf("marshal map: %w", err)
	}

	_, err = s.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &s.TableName,
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("put item: %w", err)
	}

	return nil
}

func (s *ConnectionStorage) DeleteConnection(ctx context.Context, id string) error {
	_, err := s.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &s.TableName,
		Key: map[string]types.AttributeValue{
			pk: &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return fmt.Errorf("delete item: %w", err)
	}

	return nil
}

func (s *ConnectionStorage) ListConnections(ctx context.Context) ([]string, error) {
	paginator := dynamodb.NewScanPaginator(s.Client, &dynamodb.ScanInput{
		TableName:            aws.String(s.TableName),
		ProjectionExpression: aws.String(pk),
	})

	var ids []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("next page: %w", err)
		}

		for _, item := range page.Items {
			var c Connection
			if err := attributevalue.UnmarshalMap(item, &c); err != nil {
				s.Logger.Warn("unmarshal map", zap.Error(err))
				continue
			}
			ids = append(ids, c.ConnectionID)
		}
	}

	return ids, nil
}
