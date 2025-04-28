package example

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo defines the interface for database operations.
type Repo interface {
	AddMessage(context context.Context, example *Entity) error
	MarkSent(context context.Context, ids []string) error
	GetNotSent(ctx context.Context, limit int64) ([]*Entity, error)
}

// repo is the concrete implementation of the Repo interface. It uses a pgxpool.Pool to interact with the PostgreSQL database.
type repo struct {
	pool *pgxpool.Pool
}

// NewRepo creates a new repository instance.
func NewRepo(pool *pgxpool.Pool) Repo {
	return &repo{pool: pool}
}

// AddMessage inserts a new message into the example table  and updates the entity id field with the generated primary key.
func (r *repo) AddMessage(ctx context.Context, example *Entity) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	err = tx.QueryRow(ctx, "INSERT INTO example (text) VALUES ($1) RETURNING id", example.Text).Scan(&example.Id)
	if err != nil {
		return err
	}
	err = tx.Commit(ctx)
	if err != nil {
		return tx.Rollback(ctx)
	}
	return nil
}

// MarkSent updates an existing messages isSent flag to true, marking them as sent.
func (r *repo) MarkSent(ctx context.Context, ids []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE example SET isSent = true WHERE id = ANY($1)", ids)
	if err != nil {
		return err
	}
	err = tx.Commit(ctx)
	if err != nil {
		return tx.Rollback(ctx)
	}
	return nil
}

// GetNotSent retrieves messages from the example table with given limit
func (r *repo) GetNotSent(ctx context.Context, limit int64) ([]*Entity, error) {
	rows, err := r.pool.Query(ctx, "SELECT id, text FROM example WHERE isSent = false LIMIT ($1)", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entities := make([]*Entity, 0)
	for rows.Next() {
		var example Entity
		err := rows.Scan(&example.Id, &example.Text)
		if err != nil {
			return nil, err
		}
		entities = append(entities, &example)
	}
	return entities, nil
}
