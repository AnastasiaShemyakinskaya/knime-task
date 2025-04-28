package example

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/AnastasiaShemyakinskaya/knime-task/lib"
)

type mockPublisher struct {
	publishedMessages []*lib.Message
	shouldFail        bool
}

func (m *mockPublisher) Publish(ctx context.Context, msg *lib.Message) error {
	if m.shouldFail {
		return assert.AnError
	}
	m.publishedMessages = append(m.publishedMessages, msg)
	return nil
}

func (m *mockPublisher) Run(ctx context.Context) {
	// No-op
}

func TestService(t *testing.T) {
	ctx := context.Background()

	t.Run("AddMessage: successfully adds and publishes a message", func(t *testing.T) {
		repo := NewInMemoryRepo()
		logger := zap.NewNop()

		publisher := &mockPublisher{}
		svc := NewService(repo, publisher, logger)

		entity := &Entity{Text: "test message"}

		err := svc.AddMessage(ctx, entity)

		require.NoError(t, err)
		assert.NotEmpty(t, entity.Id)

		assert.Len(t, publisher.publishedMessages, 1)
		assert.Equal(t, entity.Id, publisher.publishedMessages[0].Id)

		notSent, err := repo.GetNotSent(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, notSent, 0)
	})

	t.Run("ProcessNotSent: publishes all pending messages", func(t *testing.T) {
		repo := NewInMemoryRepo()
		publisher := &mockPublisher{}
		logger := zap.NewNop()

		svc := NewService(repo, publisher, logger)

		msg1 := &Entity{Text: "message 1"}
		msg2 := &Entity{Text: "message 2"}

		_ = repo.AddMessage(ctx, msg1)
		_ = repo.AddMessage(ctx, msg2)

		svc.(*service).processNotSent(ctx)

		assert.Len(t, publisher.publishedMessages, 2)

		notSent, err := repo.GetNotSent(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, notSent, 0)
	})
}

type InMemoryRepo struct {
	mu       sync.RWMutex
	messages map[string]*Entity
	counter  int64
}

func NewInMemoryRepo() *InMemoryRepo {
	return &InMemoryRepo{
		messages: make(map[string]*Entity),
	}
}

func (r *InMemoryRepo) AddMessage(ctx context.Context, example *Entity) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.counter++
	id := generateID(r.counter)
	example.Id = id
	r.messages[id] = &Entity{
		Id:   id,
		Text: example.Text,
	}
	return nil
}

func (r *InMemoryRepo) MarkSent(ctx context.Context, ids []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, id := range ids {
		if msg, ok := r.messages[id]; ok {
			msg.IsSent = true
		} else {
			return errors.New("message not found: " + id)
		}
	}
	return nil
}

func (r *InMemoryRepo) GetNotSent(ctx context.Context, limit int64) ([]*Entity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Entity
	for _, msg := range r.messages {
		if !msg.IsSent {
			result = append(result, &Entity{
				Id:   msg.Id,
				Text: msg.Text,
			})
			if int64(len(result)) >= limit {
				break
			}
		}
	}
	return result, nil
}

func generateID(counter int64) string {
	return "msg-" + fmt.Sprint(counter)
}
