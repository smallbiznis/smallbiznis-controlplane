package middleware

import (
	"smallbiznis-controlplane/pkg/errutil"

	"github.com/gin-gonic/gin"
)

func Error() gin.HandlerFunc {
	return func(c *gin.Context) {
		err := c.Errors.Last()

		if v, ok := err.Err.(errutil.BaseError); ok {
			c.JSON(v.Code.HTTPStatus(), v)
			return
		}
	}
}
