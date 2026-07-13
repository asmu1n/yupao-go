package core

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RespondOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, OK(data))
}

func RespondError(c *gin.Context, err error) {
	c.JSON(HTTPCodeFromErr(err), Fail(err))
}

func RespondBindingError(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, FailWithCode(ParamsError, err.Error()))
}
