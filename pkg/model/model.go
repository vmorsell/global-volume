package model

type BroadcastMessage struct {
	Users  int `json:"users,omitempty"`
	Volume int `json:"volume,omitempty"`
}
