package db

import (
	"context"
	"errors"
	"strings"

	commonInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/common/interfaces"

	"gorm.io/gorm"
)

type transactionKey struct{}

func WithTransaction(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, transactionKey{}, tx)
}

func GetTransactionFromContext(ctx context.Context) *gorm.DB {
	if ctx == nil {
		return nil
	}
	tx, ok := ctx.Value(transactionKey{}).(*gorm.DB)
	if !ok {
		return nil
	}
	return tx
}

type TransactionManager struct {
	db *gorm.DB
}

func NewTransactionManager(db *gorm.DB) commonInterfaces.TransactionManagerInterface {
	return &TransactionManager{db: db}
}

type txOwnerKey struct{}

func (tm *TransactionManager) BeginTransaction(ctx context.Context) (context.Context, error) {
	existingTx := GetTransactionFromContext(ctx)
	if existingTx != nil {
		return ctx, nil
	}

	tx := tm.db.Begin()
	if tx.Error != nil {
		return ctx, tx.Error
	}

	ctx = WithTransaction(ctx, tx)
	ctx = context.WithValue(ctx, txOwnerKey{}, true)
	return ctx, nil
}

func (tm *TransactionManager) IsTransactionOwner(ctx context.Context) bool {
	owner, _ := ctx.Value(txOwnerKey{}).(bool)
	return owner
}

func (tm *TransactionManager) CommitTransaction(ctx context.Context) error {
	tx := GetTransactionFromContext(ctx)
	if tx == nil {
		return nil
	}

	err := tx.Commit().Error
	if err != nil && isTransactionFinalizedError(err) {
		return nil
	}
	return err
}

func (tm *TransactionManager) RollbackTransaction(ctx context.Context) error {
	tx := GetTransactionFromContext(ctx)
	if tx == nil {
		return nil
	}

	err := tx.Rollback().Error
	if err != nil && isTransactionFinalizedError(err) {
		return nil
	}
	return err
}

func isTransactionFinalizedError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "already been committed") ||
		strings.Contains(errStr, "already been rolled back") ||
		errors.Is(err, gorm.ErrInvalidTransaction)
}
