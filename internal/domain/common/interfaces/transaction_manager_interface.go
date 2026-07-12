package interfaces

import (
	"context"
)

type TransactionManagerInterface interface {
	BeginTransaction(ctx context.Context) (context.Context, error)
	CommitTransaction(ctx context.Context) error
	RollbackTransaction(ctx context.Context) error
	IsTransactionOwner(ctx context.Context) bool
}
