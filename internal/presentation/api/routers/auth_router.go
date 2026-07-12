package routers

import "github.com/scienceandcode/nucleus-api/internal/infrastructure/api/middlewares"

func (r *Router) RegisterAuthRoutes() {
	group := r.group.Group("/auth")

	group.POST("/register", r.handlers.UserAuthHandler.Register)
	group.POST("/login", r.handlers.UserAuthHandler.Login)
	group.POST("/forgot-password", r.handlers.UserAuthHandler.ForgotPassword)

	group.POST("/confirm-email", middlewares.AuthenticationMiddleware(&middlewares.AuthMiddlewareCustomizer{
		SkipEmailConfirmedValidation: true,
	}), r.handlers.UserAuthHandler.ConfirmEmail)

	group.POST("/resend-confirmation", middlewares.AuthenticationMiddleware(&middlewares.AuthMiddlewareCustomizer{
		SkipEmailConfirmedValidation: true,
	}), r.handlers.UserAuthHandler.ResendConfirmationEmail)

	group.PATCH("/change-password", middlewares.AuthenticationMiddleware(&middlewares.AuthMiddlewareCustomizer{
		SkipPasswordUpdatedValidation: true,
		SkipEmailConfirmedValidation:  true,
	}), r.handlers.UserAuthHandler.UpdatePassword)

	group.POST("/logout", middlewares.AuthenticationMiddleware(&middlewares.AuthMiddlewareCustomizer{
		SkipPasswordUpdatedValidation: true,
		SkipEmailConfirmedValidation:  true,
	}), r.handlers.UserAuthHandler.Logout)
}
