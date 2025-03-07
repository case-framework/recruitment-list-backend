package apihandlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	sdb "github.com/case-framework/case-backend/pkg/db/study"
	rdb "github.com/case-framework/recruitment-list-backend/pkg/db/recruitment-list"
	pc "github.com/case-framework/recruitment-list-backend/pkg/permission-checker"
)

func HealthCheckHandle(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type TTLs struct {
	AccessToken time.Duration
}

type HttpEndpoints struct {
	recruitmentListDBConn *rdb.RecruitmentListDBService
	studyDBConn           *sdb.StudyDBService
	tokenSignKey          string
	filestorePath         string
	ttls                  TTLs
	studyServiceConf      struct {
		GlobalSecret string
		InstanceID   string
	}
}

func NewHTTPHandler(
	tokenSignKey string,
	recruitmentListDBConn *rdb.RecruitmentListDBService,
	studyDBConn *sdb.StudyDBService,
	filestorePath string,
	ttls TTLs,
	studyGlobalSecret string,
	studyInstanceID string,
) *HttpEndpoints {
	pc.RDBConn = recruitmentListDBConn

	return &HttpEndpoints{
		tokenSignKey:          tokenSignKey,
		recruitmentListDBConn: recruitmentListDBConn,
		studyDBConn:           studyDBConn,
		filestorePath:         filestorePath,
		ttls:                  ttls,
		studyServiceConf: struct {
			GlobalSecret string
			InstanceID   string
		}{
			GlobalSecret: studyGlobalSecret,
			InstanceID:   studyInstanceID,
		},
	}
}
