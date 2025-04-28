package lib

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// Database interface for db used in Publisher
type Database interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// MQ interface for message queue used in Publisher
type MQ interface {
	Publish(topic string, data []byte) error
}

// lockNum is the advisory lock ID used for leader election via PostgreSQL.
const lockNum = 42

// Publisher defines the interface for publishing messages.
type Publisher interface {
	Publish(context.Context, *Message) error
	Run(context.Context)
}

// publisher implements the Publisher interface.
// It sends messages from PostgreSQL to a MQ server.
type publisher struct {
	db     Database
	nats   MQ
	logger *zap.Logger
}

// NewPublisher constructs a new publisher instance.
// If no logger is provided, it uses a no-op logger.
func NewPublisher(nats MQ, db Database, logger *zap.Logger) Publisher {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &publisher{nats: nats, db: db, logger: logger}
}

// Run tries to acquire leadership and starts the publishing loop.
func (p *publisher) Run(ctx context.Context) {
	leader, err := tryAcquireLeadership(ctx, p.db, lockNum)
	if err != nil {
		p.logger.Error("failed to acquire leadership", zap.Error(err))
		return
	}

	if !leader {
		p.logger.Info("this instance is not the leader, skipping consuming")
		return
	}

	p.logger.Info("this instance became the leader, starting consuming")
	go p.startPublish(ctx)
}

// Publish stores a message into the database for later publishing to MQ.
func (p *publisher) Publish(ctx context.Context, message *Message) error {
	tx, err := p.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func(tx pgx.Tx, ctx context.Context) {
		if err == nil {
			return
		}
		err = tx.Rollback(ctx)
		if err != nil {
			p.logger.Error("rollback failed", zap.Error(err))
		}
	}(tx, ctx)

	_, err = tx.Exec(ctx, "INSERT INTO messages (message_id, message, message_type, topic) VALUES ($1, $2, $3, $4) ON CONFLICT (message_id) DO NOTHING", message.Id, message.Message, message.MessageType, message.Topic)
	if err != nil {
		return err
	}
	err = tx.Commit(ctx)
	if err != nil {
		return err
	}
	return nil
}

// startPublish periodically sends pending messages to MQ.
func (p *publisher) startPublish(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			p.logger.Warn("consumer stopped")
			return
		case <-ticker.C:
			err := p.sendMessages(ctx)
			if err != nil {
				p.logger.Error("failed to send messages", zap.Error(err))
			}
		}
	}
}

// sendMessages gets unsent messages from the database and publishes them to MQ.
func (p *publisher) sendMessages(ctx context.Context) error {
	rows, err := p.db.Query(ctx, "SELECT id, message, message_type, topic FROM messages WHERE processed_at IS NULL LIMIT 100")
	if err != nil {
		return err
	}
	defer rows.Close()

	var processed []string
	for rows.Next() {
		var msg Message
		if err = rows.Scan(&msg.Id, &msg.Message, &msg.MessageType, &msg.Topic); err != nil {
			p.logger.Error("scan failed", zap.Error(err))
			continue
		}

		payload, err := json.Marshal(msg)
		if err != nil {
			p.logger.Error("marshal failed", zap.Error(err))
			continue
		}

		if err = p.nats.Publish(msg.Topic, payload); err != nil {
			p.logger.Error("publish failed", zap.Error(err))
			continue
		}

		processed = append(processed, msg.Id)
	}

	if len(processed) == 0 {
		return nil
	}
	return p.markMessageAsProcessed(ctx, processed)
}

// markMessageAsProcessed updates the database and mark messages as processed.
func (p *publisher) markMessageAsProcessed(ctx context.Context, id []string) error {
	tx, err := p.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx, "UPDATE messages SET processed_at = NOW() WHERE id = ANY($1)", id)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// tryAcquireLeadership attempts to acquire a PostgreSQL advisory lock. Only one instance can hold the lock
func tryAcquireLeadership(ctx context.Context, pool Database, lockID int64) (bool, error) {
	var success bool
	err := pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&success)
	return success, err
}
