package lib

type Message struct {
	Id          string `json:"id"`
	Message     []byte `json:"message"`
	MessageType int    `json:"message_type"`
	Topic       string `json:"topic"`
}
