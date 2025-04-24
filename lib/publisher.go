package lib

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type Publisher interface {
	Publish(context.Context, *Message) error
	Run()
	Shutdown()
}

type publisher struct {
	pool   *pgxpool.Pool
	nats   *nats.Conn
	cancel context.CancelFunc
	logger *zap.Logger
}

func NewPublisher(nats *nats.Conn, pool *pgxpool.Pool, logger *zap.Logger) Publisher {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &publisher{nats: nats, pool: pool, logger: logger}
}

func (p *publisher) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go p.startConsume(ctx)
}

func (p *publisher) Shutdown() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *publisher) Publish(ctx context.Context, message *Message) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func(tx pgx.Tx, ctx context.Context) {
		if err == nil {
			return
		}
		err = tx.Rollback(ctx)
		if err != nil {
			log.Printf("Rollback failed: %v", err)
		}
	}(tx, ctx)

	_, err = tx.Exec(ctx, "INSERT INTO outbox (id, message, message_type, topic) VALUES (?, ?, ?) ON CONFLICT DO NOTHING", message.Id, message.Message, message.MessageType, message.Topic)
	if err != nil {
		return err
	}
	err = tx.Commit(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (p *publisher) startConsume(ctx context.Context) {
	ticker := time.NewTicker(time.Millisecond * 100)
	for {
		select {
		case <-ctx.Done():
			log.Println("consumer stopped")
			return
		case <-ticker.C:
			err := p.sendMessages(ctx)
			if err != nil {
				log.Printf("failed to send messages: %v", err)
			}
		}
	}
}

func (p *publisher) sendMessages(ctx context.Context) error {
	rows, err := p.pool.Query(ctx, "SELECT id, message, message_type FROM outbox WHERE processed_at IS NULL LIMIT 100")
	if err != nil {
		return err
	}
	defer rows.Close()

	var processed []string
	for rows.Next() {
		var msg Message
		if err = rows.Scan(&msg.Id, &msg.Message, &msg.MessageType); err != nil {
			log.Printf("scan failed: %v", err)
			continue
		}

		payload, err := json.Marshal(msg)
		if err != nil {
			log.Printf("marshal failed: %v", err)
			continue
		}

		if err = p.nats.Publish(msg.Topic, payload); err != nil {
			log.Printf("publish failed: %v", err)
			continue
		}

		processed = append(processed, msg.Id)
	}

	return p.markMessageAsProcessed(ctx, processed)
}

func (p *publisher) markMessageAsProcessed(ctx context.Context, id []string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx, "UPDATE outbox SET processed_at = NOW() WHERE id = ANY(?)", id)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
