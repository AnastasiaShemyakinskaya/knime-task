package example

import (
	"context"
	"go.uber.org/zap"
	"time"

	"github.com/AnastasiaShemyakinskaya/knime-task/lib"
)

const messageLimit = 100 // Limit on how many unsent messages to fetch at once

// Service defines the main business logic interface.
type Service interface {
	AddMessage(ctx context.Context, example *Entity) error
	Run(ctx context.Context)
}

// service is the concrete implementation of the Service interface.
type service struct {
	repo      Repo
	publisher lib.Publisher
	logger    *zap.Logger
}

// NewService constructs a new service with injected dependencies.
func NewService(repo Repo, publisher lib.Publisher, logger *zap.Logger) Service {
	return &service{repo: repo, publisher: publisher, logger: logger}
}

// AddMessage stores a new message in the database publishes it and marks it as sent if publishing succeeds.
func (a *service) AddMessage(ctx context.Context, example *Entity) error {
	err := a.repo.AddMessage(ctx, example)
	if err != nil {
		return err
	}
	err = a.publisher.Publish(ctx, example.ToMessage())
	if err != nil {
		return err
	}
	return a.repo.MarkSent(ctx, []string{example.Id})
}

// Run starts a background goroutine that periodically tries to publish unsent messages from the database.
func (a *service) Run(ctx context.Context) {
	go func() {
		a.processNotSent(ctx)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.processNotSent(ctx)
			}
		}
	}()
}

// processNotSent fetches unsent messages, attempts to publish and marks them as sent upon success.
func (a *service) processNotSent(ctx context.Context) {
	notSent, err := a.repo.GetNotSent(ctx, messageLimit)
	if err != nil {
		a.logger.Error("failed to get notSent", zap.Error(err))
		return
	}
	var processedIds []string
	for _, entity := range notSent {
		err = a.publisher.Publish(ctx, entity.ToMessage())
		if err != nil {
			a.logger.Error("failed to publish message", zap.Error(err))
			continue
		}
		processedIds = append(processedIds, entity.Id)
	}
	err = a.repo.MarkSent(ctx, processedIds)
	if err != nil {
		a.logger.Error("failed to markSent", zap.Error(err))
	}
}
