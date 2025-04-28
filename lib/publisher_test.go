package lib

import (
	"context"
	"encoding/json"
	"go.uber.org/zap"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockNatsConn struct {
	publishedMessages []publishedMessage
	failPublish       bool
}

type publishedMessage struct {
	subject string
	payload []byte
}

func (m *mockNatsConn) Publish(subject string, payload []byte) error {
	if m.failPublish {
		return assert.AnError
	}
	m.publishedMessages = append(m.publishedMessages, publishedMessage{subject, payload})
	return nil
}

func TestPublisher_Publish(t *testing.T) {
	ctx := context.Background()

	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	mockNATS := &mockNatsConn{}
	logger := zap.NewNop()

	publisher := NewPublisher(mockNATS, mockDB, logger)

	message := &Message{
		Id:      "id",
		Message: []byte("test"),
		Topic:   "topic",
	}

	mockDB.ExpectBegin()
	mockDB.ExpectExec("INSERT INTO messages").
		WithArgs(message.Id, message.Message, message.MessageType, message.Topic).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mockDB.ExpectCommit()

	err = publisher.Publish(ctx, message)

	require.NoError(t, err)
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestPublisher_SendMessages(t *testing.T) {
	ctx := context.Background()

	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	mockNATS := &mockNatsConn{}
	logger := zap.NewNop()

	pub := NewPublisher(mockNATS, mockDB, logger)

	msg := &Message{
		Id:      "id",
		Message: []byte("test"),
		Topic:   "test.topic",
	}
	payload, _ := json.Marshal(msg)

	mockDB.ExpectQuery("SELECT id, message, message_type, topic FROM messages").
		WillReturnRows(pgxmock.NewRows([]string{"id", "message", "message_type", "topic"}).
			AddRow(msg.Id, msg.Message, msg.MessageType, msg.Topic),
		)

	mockDB.ExpectBegin()
	mockDB.ExpectExec("UPDATE messages SET processed_at = NOW()").
		WithArgs([]string{msg.Id}).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mockDB.ExpectCommit()

	err = pub.(*publisher).sendMessages(ctx)

	require.NoError(t, err)
	assert.Len(t, mockNATS.publishedMessages, 1)
	assert.Equal(t, "test.topic", mockNATS.publishedMessages[0].subject)
	assert.Equal(t, payload, mockNATS.publishedMessages[0].payload)
	assert.NoError(t, mockDB.ExpectationsWereMet())
}
