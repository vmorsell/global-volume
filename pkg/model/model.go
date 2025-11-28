package model

type State struct {
	ConnectionIDs []string `json:"connectionIds" dynamodbav:"connectionIds"`
	Volume        int      `json:"volume" dynamodbav:"volume"`
}

type StateMessage struct {
	Users  int `json:"users"`
	Volume int `json:"volume"`
}
