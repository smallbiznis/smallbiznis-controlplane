package middleware

import (
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Authorize(e *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString(UserID)
		orgID := c.GetString(OrgID)

		if userID == "" && orgID == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		ok, err := e.Enforce(userID, orgID, c.FullPath())
		if err != nil {
			zap.L().Error("Failed enforce subject", zap.String("user_id", userID), zap.String("org_id", orgID), zap.String("path", c.FullPath()))
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		if !ok {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}
