package permissionchecker

import (
	"log/slog"

	rdb "github.com/case-framework/recruitement-list-backend/pkg/db/recruitment-list"
)

type DBConnector interface {
	GetSpecificPermissionsByUserID(userID string, actions []string, resources []string) ([]rdb.Permission, error)
}

var (
	RDBConn DBConnector
)

func IsAuthorized(userID string, requiredActions []string, requiredResources []string, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	if RDBConn == nil {
		slog.Error("dbConnector not set")
		return false
	}

	permissions, err := RDBConn.GetSpecificPermissionsByUserID(userID, requiredActions, requiredResources)
	if err != nil {
		slog.Error("could not get permissions", slog.String("error", err.Error()))
		return false
	}

	if len(permissions) == 0 {
		return false
	}

	return true
}
