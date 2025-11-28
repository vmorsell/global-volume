package model

type VolumeState struct {
	Volume    int   `json:"volume" dynamodbav:"volume"`
	Timestamp int64 `json:"timestamp" dynamodbav:"timestamp"`
}

type ConnectionInfo struct {
	ConnectionID string `json:"connectionId" dynamodbav:"connectionId"`
	SourceIP     string `json:"sourceIP" dynamodbav:"sourceIP"`
}

type ConnectionsState struct {
	Connections []ConnectionInfo `json:"connections" dynamodbav:"connections"`
}

type State struct {
	ConnectionIDs []string
	Volume        int
}

type MessageType string

const (
	MessageTypeVolume           MessageType = "volume"
	MessageTypeConnectedClients MessageType = "clients"
)

type VolumeMessage struct {
	Type   MessageType `json:"type"`
	Volume int         `json:"volume"`
}

type ConnectedClientsMessage struct {
	Type    MessageType `json:"type"`
	Clients int         `json:"clients"`
}
