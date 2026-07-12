package api

import (
	"context"
	"log"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"

	"github.com/gin-gonic/gin"
)

var ginLambda *ginadapter.GinLambdaV2

func Run(app *gin.Engine) {
	key := common.GetEnv("SERVER_RUNNER")
	runners := map[string]func(*gin.Engine){
		"default": func(app *gin.Engine) {
			err := app.Run(":" + common.GetEnv("SERVER_PORT"))
			if err != nil {
				log.Fatal(err.Error())
			}
		},
		"lambda": func(app *gin.Engine) {
			ginLambda = ginadapter.NewV2(app)
			lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
				return ginLambda.ProxyWithContext(ctx, req)
			})
		},
	}

	runner, exists := runners[key]

	if !exists {
		log.Fatalf("(%s) HTTP server runner key does not exist", key)
	}

	runner(app)
}
