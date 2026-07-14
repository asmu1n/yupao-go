package middleware

import (
	"net/http"

	"yupao-go/internal/shared/resp"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const SessionKeyUserID = "userID"

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		uid := session.Get(SessionKeyUserID)
		if uid == nil {
			c.JSON(http.StatusUnauthorized, resp.FailWithCode(resp.NotLogin, ""))
			c.Abort()
			return
		}
		c.Set(SessionKeyUserID, uid)
		c.Next()
	}
}

func GetLoginUserID(c *gin.Context) (int64, error) {
	uid, exists := c.Get(SessionKeyUserID)
	if !exists {
		return 0, resp.NewBizError(resp.NotLogin)
	}
	id, ok := uid.(int64)
	if !ok {
		return 0, resp.NewBizError(resp.NotLogin)
	}
	return id, nil
}
