package server

type Message struct {
	Type string `json:"type"`
	ToUserId int64 `json:"to_user_id"`
	RoomId string `json:"room_id"`
	Data string `json:"data"`
}