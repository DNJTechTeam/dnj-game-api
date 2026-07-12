package services

import (
	"context"

	commonInterfaces "github.com/scienceandcode/nucleus-api/internal/domain/common/interfaces"
)

type BaseService struct {
	TransactionManager commonInterfaces.TransactionManagerInterface
}

func NewBaseService(transactionManager commonInterfaces.TransactionManagerInterface) *BaseService {
	return &BaseService{
		TransactionManager: transactionManager,
	}
}

func (bs *BaseService) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	ownerBefore := bs.TransactionManager.IsTransactionOwner(ctx)
	ctx, err := bs.TransactionManager.BeginTransaction(ctx)
	if err != nil {
		return err
	}

	isOwner := !ownerBefore && bs.TransactionManager.IsTransactionOwner(ctx)

	shouldRollback := true
	defer func() {
		if shouldRollback && isOwner {
			bs.TransactionManager.RollbackTransaction(ctx)
		}
	}()

	err = fn(ctx)
	if err != nil {
		return err
	}

	if isOwner {
		err = bs.TransactionManager.CommitTransaction(ctx)
		if err != nil {
			return err
		}
	}

	shouldRollback = false
	return nil
}
