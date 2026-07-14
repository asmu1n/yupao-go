package router

import (
	"yupao-go/ent"

	"github.com/gin-gonic/gin"

	_ "yupao-go/api/swagger"
)

func RegisterRouter(r *gin.Engine, client *ent.Client) {
	api := r.Group("/api")
	registerUser(api, client)
}
