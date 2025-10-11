package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	// Header Key
	HeaderUserID = "X-User-ID"
	HeaderOrgID  = "X-Org-ID"

	// Context Key
	UserID = "user_id"
	OrgID  = "org_id"
)

func Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {

		header := c.Request.Header
		if header.Get(UserID) == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set(UserID, header.Get(UserID))

		if header.Get(OrgID) == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set(OrgID, header.Get(OrgID))

		c.Next()
	}
}
