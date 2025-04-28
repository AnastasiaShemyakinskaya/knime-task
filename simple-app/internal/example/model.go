package example

import "github.com/AnastasiaShemyakinskaya/knime-task/lib"

// Entity is an example model, stored in database
type Entity struct {
	Id     string
	Text   string
	IsSent bool
}

// ToMessage converts entity to publish message
func (e Entity) ToMessage() *lib.Message {
	return &lib.Message{
		Id:      e.Id,
		Message: []byte(e.Text),
		Topic:   "app.updates",
	}
}
