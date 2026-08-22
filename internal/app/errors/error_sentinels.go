package errors

var (
	ErrNotFound   = NewError("Recurso não encontrado.", nil)
	ErrConflict   = NewError("Recurso já existe.", nil)
	InternalError = NewError("Ocorreu um erro interno. Por favor, tente novamente mais tarde.", nil)
)
