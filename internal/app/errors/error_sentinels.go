package errors

var (
	ErrNotFound   = NewError("Recurso não encontrado.", nil)
	InternalError = NewError("Ocorreu um erro interno. Por favor, tente novamente mais tarde.", nil)
)
