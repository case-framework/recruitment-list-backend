package apihandlers

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	jwthandling "github.com/case-framework/case-backend/pkg/jwt-handling"
	"github.com/gin-gonic/gin"

	pc "github.com/case-framework/recruitment-list-backend/pkg/permission-checker"
)

type RequiredPermission struct {
	IgnoreId bool
	Actions  []string
}

func (h *HttpEndpoints) useAuthorisedHandler(
	requiredPermission RequiredPermission,
	handler gin.HandlerFunc,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

		requiredResourceIds := []string{c.Param("id")}
		if requiredPermission.IgnoreId {
			requiredResourceIds = []string{}
		}

		hasPermission := pc.IsAuthorized(
			token.Subject,
			requiredPermission.Actions,
			requiredResourceIds,
			token.IsAdmin,
		)
		if !hasPermission {
			slog.Warn("unauthorised access attempted",
				slog.String("userID", token.Subject),
				slog.String("resourceKeys", strings.Join(requiredResourceIds, ",")),
				slog.String("action", strings.Join(requiredPermission.Actions, ",")),
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorised access attempted"})
			return
		}

		handler(c)
	}
}

func (h *HttpEndpoints) middlewareHasPermissionForRL(
	requiredPermission RequiredPermission,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

		recruitmentListID := c.Param("id")
		if recruitmentListID == "" {
			slog.Warn("no recruitmentListID")
			c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
			return
		}

		hasPermission := pc.IsAuthorized(
			token.Subject,
			requiredPermission.Actions,
			[]string{recruitmentListID},
			token.IsAdmin,
		)
		if !hasPermission {
			slog.Warn("unauthorised access attempted",
				slog.String("userID", token.Subject),
				slog.String("resourceKeys", recruitmentListID),
				slog.String("action", strings.Join(requiredPermission.Actions, ",")),
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorised access attempted"})
			return
		}

		c.Next()
	}
}

func (h *HttpEndpoints) getFullFilePath(path string) string {
	return filepath.Join(h.filestorePath, path)
}

func (h *HttpEndpoints) deleteFileAtPath(path string) error {
	fullPath := h.getFullFilePath(path)
	err := os.Remove(fullPath)
	return err
}
