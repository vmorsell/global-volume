package storage

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/vmorsell/global-volume/pkg/model"
	"go.uber.org/zap"
)

const (
	pk = "pk"

	stateDocPK = "state"
)

type Storage struct {
	Logger    *zap.Logger
	Client    *dynamodb.Client
	TableName string
}

func (s *Storage) SaveState(ctx context.Context, state model.State) error {
	item, err := attributevalue.MarshalMap(state)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	item[pk] = &types.AttributeValueMemberS{Value: stateDocPK}

	_, err = s.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &s.TableName,
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("put item: %w", err)
	}
	return nil
}

func (s *Storage) GetState(ctx context.Context) (model.State, error) {
	result, err := s.Client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.TableName,
		Key: map[string]types.AttributeValue{
			pk: &types.AttributeValueMemberS{Value: stateDocPK},
		},
	})
	if err != nil {
		return model.State{}, fmt.Errorf("get item: %w", err)
	}

	var state model.State
	if err := attributevalue.UnmarshalMap(result.Item, &state); err != nil {
		return model.State{}, fmt.Errorf("unmarshal map: %w", err)
	}

	return state, nil
}

func (s *Storage) DeleteConnection(ctx context.Context, connectionID string) error {
	state, err := s.GetState(ctx)
	if err != nil {
		return fmt.Errorf("get state: %w", err)
	}

	for i, id := range state.ConnectionIDs {
		if id == connectionID {
			state.ConnectionIDs = append(state.ConnectionIDs[:i], state.ConnectionIDs[i+1:]...)
			break
		}
	}

	if err := s.SaveState(ctx, state); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	return nil
}
