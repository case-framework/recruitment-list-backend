package apihandlers

import (
	"encoding/csv"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	mw "github.com/case-framework/case-backend/pkg/apihelpers/middlewares"
	jwthandling "github.com/case-framework/case-backend/pkg/jwt-handling"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"

	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
	rdb "github.com/case-framework/recruitment-list-backend/pkg/db/recruitment-list"
	pc "github.com/case-framework/recruitment-list-backend/pkg/permission-checker"
	"github.com/case-framework/recruitment-list-backend/pkg/sync"
)

func (h *HttpEndpoints) AddRecruitmentListsAPI(rg *gin.RouterGroup) {
	recruitmentListsGroup := rg.Group("/recruitment-lists")
	recruitmentListsGroup.Use(mw.GetAndValidateManagementUserJWT(h.tokenSignKey))
	{
		recruitmentListsGroup.GET("", h.getRecruitmentLists)
		recruitmentListsGroup.POST("",
			mw.RequirePayload(),
			h.useAuthorisedHandler(
				RequiredPermission{
					IgnoreId: true,
					Actions: []string{
						pc.ACTION_CREATE_RECRUITMENT_LIST,
					},
				},
				h.createRecruitmentList,
			),
		)

		recruitmentListsGroup.DELETE("/:id", h.useAuthorisedHandler(
			RequiredPermission{
				IgnoreId: true,
				Actions: []string{
					pc.ACTION_DELETE_RECRUITMENT_LIST,
				},
			},
			h.deleteRecruitmentList,
		))

		rlManageGroup := recruitmentListsGroup.Group("/:id")
		rlManageGroup.Use(h.middlewareHasPermissionForRL(
			RequiredPermission{
				IgnoreId: false,
				Actions: []string{
					pc.ACTION_MANAGE_RECRUITMENT_LIST,
				},
			}))
		{

			rlManageGroup.PUT("", h.updateRecruitmentList)
			rlManageGroup.POST("/import-participant", mw.RequirePayload(), h.importParticipant)
			rlManageGroup.GET("/permissions", h.getRecruitmentListPermissions)
			rlManageGroup.POST("/permissions", mw.RequirePayload(), h.createRecruitmentListPermission)
			rlManageGroup.DELETE("/permissions/:permissionID", h.deleteRecruitmentListPermission)
			rlManageGroup.GET("/sync-infos", h.getSyncInfos)
			rlManageGroup.POST("/sync-participants", h.syncParticipants)
			rlManageGroup.POST("/sync-responses", h.syncResponses)
			rlManageGroup.POST("/reset-participant-sync", h.resetParticipantSync)
			rlManageGroup.POST("/reset-data-sync", h.resetDataSync)
		}

		// Access recruitment list
		rlAccessGroup := recruitmentListsGroup.Group("/:id")
		rlAccessGroup.Use(h.middlewareHasPermissionForRL(
			RequiredPermission{
				IgnoreId: false,
				Actions: []string{
					pc.ACTION_ACCESS_RECRUITMENT_LIST,
					pc.ACTION_MANAGE_RECRUITMENT_LIST,
					pc.ACTION_DELETE_RECRUITMENT_LIST,
				},
			}))
		{
			rlAccessGroup.GET("", h.getRecruitmentList)

			participantGroup := rlAccessGroup.Group("/participants")
			{
				participantGroup.GET("", h.getParticipants)
				participantGroup.GET("/:participantID", h.getParticipant)
				participantGroup.POST("/:participantID/status", h.updateParticipantStatus)
				participantGroup.GET("/:participantID/notes", h.getParticipantNotes)
				participantGroup.POST("/:participantID/notes", h.addParticipantNote)
				participantGroup.DELETE("/:participantID/notes/:noteID", h.deleteParticipantNote)
			}

			rlAccessGroup.GET("/available-responses", h.getAvailableResponses)

			downloadGroup := rlAccessGroup.Group("/downloads")
			{
				downloadGroup.GET("", h.getDownloads)
				downloadGroup.POST("/prepare-response-file", mw.RequirePayload(), h.startResponseDownload)
				downloadGroup.POST("/prepare-participant-infos-file", mw.RequirePayload(), h.startParticipantInfosDownload)
				downloadGroup.GET("/:downloadID/status", h.getDownload)
				downloadGroup.GET("/:downloadID", h.serveDownloadFile)
				downloadGroup.DELETE("/:downloadID", h.deleteDownload)
			}
		}

	}
}

func (h *HttpEndpoints) createRecruitmentList(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	var req rdb.RecruitmentList
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Info("create recruitment list", slog.String("userID", token.Subject))

	rl, err := h.recruitmentListDBConn.CreateRecruitmentList(req, token.Subject)
	if err != nil {
		slog.Error("could not create recruitment list", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create recruitment list"})
		return
	}

	if !token.IsAdmin {
		_, err = h.recruitmentListDBConn.CreatePermission(token.Subject, pc.ACTION_MANAGE_RECRUITMENT_LIST, rl.ID.Hex(), token.Subject, nil)
		if err != nil {
			slog.Error("could not create permission", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create permission"})
			return
		}
		_, err = h.recruitmentListDBConn.CreatePermission(token.Subject, pc.ACTION_DELETE_RECRUITMENT_LIST, rl.ID.Hex(), token.Subject, nil)
		if err != nil {
			slog.Error("could not create permission", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create permission"})
			return
		}
	}

	c.JSON(http.StatusOK, rl)
}

func (h *HttpEndpoints) getRecruitmentLists(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	slog.Info("get recruitment lists", slog.String("userID", token.Subject))

	recruitmentLists, err := h.recruitmentListDBConn.GetRecruitmentListsInfos()
	if err != nil {
		slog.Error("could not get recruitment lists", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get recruitment lists"})
		return
	}
	var recruitmentListsForUser []rdb.RecruitmentList
	resp := gin.H{
		"recruitmentLists": recruitmentListsForUser,
	}
	if token.IsAdmin {
		resp["recruitmentLists"] = recruitmentLists
		c.JSON(http.StatusOK, resp)
		return
	}

	permissions, err := h.recruitmentListDBConn.GetPermissionsByUserID(token.Subject)
	if err != nil {
		slog.Error("could not get permissions", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get permissions"})
		return
	}

	for _, recruitmentList := range recruitmentLists {
		hasPermission := false
		for _, permission := range permissions {
			if permission.ResourceID == recruitmentList.ID.Hex() && (permission.Action == pc.ACTION_ACCESS_RECRUITMENT_LIST ||
				permission.Action == pc.ACTION_MANAGE_RECRUITMENT_LIST ||
				permission.Action == pc.ACTION_DELETE_RECRUITMENT_LIST) {
				hasPermission = true
				break
			}
		}
		if hasPermission {
			recruitmentListsForUser = append(recruitmentListsForUser, recruitmentList)
		}
	}

	resp["recruitmentLists"] = recruitmentListsForUser
	c.JSON(http.StatusOK, resp)
}

func (h *HttpEndpoints) getRecruitmentList(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	slog.Info("get recruitment list", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID))

	recruitmentList, err := h.recruitmentListDBConn.GetRecruitmentListByID(recruitmentListID)
	if err != nil {
		slog.Error("could not get recruitment list", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get recruitment list"})
		return
	}

	c.JSON(http.StatusOK, recruitmentList)
}

func (h *HttpEndpoints) updateRecruitmentList(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	var req rdb.RecruitmentList
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Info("update recruitment list", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID))

	if err := h.recruitmentListDBConn.SaveRecruitmentList(req); err != nil {
		slog.Error("could not update recruitment list", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update recruitment list"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "recruitment list updated",
	})
}

type ImportParticipantRequest struct {
	ParticipantID string `json:"participantId"`
}

func (h *HttpEndpoints) importParticipant(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	var req ImportParticipantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rl, err := h.recruitmentListDBConn.GetRecruitmentListByID(recruitmentListID)
	if err != nil {
		slog.Error("could not get recruitment list", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get recruitment list"})
		return
	}

	studyKey := rl.ParticipantInclusion.StudyKey

	slog.Info("import participant", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID), slog.String("studyKey", studyKey), slog.String("participantID", req.ParticipantID))

	participant, err := h.studyDBConn.GetParticipantByID(h.studyServiceConf.InstanceID, studyKey, req.ParticipantID)
	if err != nil {
		slog.Error("could not get participant", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get participant"})
		return
	}

	if participant.StudyStatus == studyTypes.PARTICIPANT_STUDY_STATUS_ACCOUNT_DELETED {
		slog.Error("participant has been deleted", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID), slog.String("studyKey", studyKey), slog.String("participantID", req.ParticipantID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "participant has been deleted"})
		return
	}

	if h.recruitmentListDBConn.ParticipantExists(req.ParticipantID, recruitmentListID) {
		slog.Error("participant already included", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID), slog.String("studyKey", studyKey), slog.String("participantID", req.ParticipantID))
		c.JSON(http.StatusOK, gin.H{"message": "participant already included"})
		return
	}

	_, err = h.recruitmentListDBConn.CreateParticipant(req.ParticipantID, recruitmentListID, token.Subject)
	if err != nil {
		slog.Error("could not create participant", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create participant"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "participant imported"})
}

func (h *HttpEndpoints) getRecruitmentListPermissions(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	slog.Info("get recruitment list permissions", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID))

	permissions, err := h.recruitmentListDBConn.GetPermissionsByResourceID(recruitmentListID)
	if err != nil {
		slog.Error("could not get permissions", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"permissions": permissions})
}

type CreateRecruitmentListPermissionRequest struct {
	UserID string `json:"userId"`
	Action string `json:"action"`
}

func (h *HttpEndpoints) createRecruitmentListPermission(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	var req CreateRecruitmentListPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Info("create recruitment list permission", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID))

	p, err := h.recruitmentListDBConn.CreatePermission(req.UserID, req.Action, recruitmentListID, token.Subject, nil)

	if err != nil {
		slog.Error("could not create permission", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create permission"})
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *HttpEndpoints) deleteRecruitmentListPermission(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	permissionID := c.Param("permissionID")
	if permissionID == "" {
		slog.Warn("no permissionID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no permissionID"})
		return
	}

	slog.Info("delete recruitment list permission", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID), slog.String("permissionID", permissionID))

	if err := h.recruitmentListDBConn.DeletePermissionByID(permissionID); err != nil {
		slog.Error("could not delete permission", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete permission"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "permission deleted"})
}

func (h *HttpEndpoints) getSyncInfos(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	slog.Info("getting sync infos", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID))

	syncInfos, err := h.recruitmentListDBConn.GetSyncInfoByRLID(recruitmentListID)
	if err != nil {
		slog.Error("could not get sync infos", slog.String("error", err.Error()))
		c.JSON(http.StatusOK, rdb.SyncInfo{
			RecruitmentListID: recruitmentListID,
		})
		return
	}

	c.JSON(http.StatusOK, syncInfos)
}

func (h *HttpEndpoints) syncParticipants(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	slog.Info("sync participants", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID))

	go func() {
		if err := sync.SyncParticipantsForRL(
			h.recruitmentListDBConn,
			h.studyDBConn,
			recruitmentListID,
			h.studyServiceConf.InstanceID,
		); err != nil {
			slog.Error("could not sync participants", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "participant sync started"})
}

func (h *HttpEndpoints) syncResponses(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	slog.Info("sync responses", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID))

	go func() {
		if err := sync.SyncResearchDataForRL(
			h.recruitmentListDBConn,
			h.studyDBConn,
			recruitmentListID,
			h.studyServiceConf.InstanceID,
			h.studyServiceConf.GlobalSecret,
		); err != nil {
			slog.Error("could not sync research data", slog.String("error", err.Error()))
			return
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "sync started"})
}

func (h *HttpEndpoints) resetParticipantSync(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	slog.Info("reset all data", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID))

	if err := h.recruitmentListDBConn.ResetParticipantSyncTime(recruitmentListID); err != nil {
		slog.Error("could not reset participant sync", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not reset participant sync"})
		return
	}

	if err := h.recruitmentListDBConn.DeleteAllParticipantsByRecruitmentListID(recruitmentListID); err != nil {
		slog.Error("could not delete all participants", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete all participants"})
		return
	}

	if err := h.recruitmentListDBConn.DeleteParticipantNotesByRecruitmentListID(recruitmentListID); err != nil {
		slog.Error("could not delete all participant notes", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete all participant notes"})
		return
	}

	if err := h.recruitmentListDBConn.DeleteResearchDataByRecruitmentListID(recruitmentListID); err != nil {
		slog.Error("could not delete all responses", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete all responses"})
		return
	}

	// reset sync info
	if err := h.recruitmentListDBConn.ResetDataSyncTime(recruitmentListID); err != nil {
		slog.Error("could not reset data sync", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not reset data sync"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "participant sync reset"})
}

func (h *HttpEndpoints) resetDataSync(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	slog.Info("reset data sync", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID))

	// remove all responses
	if err := h.recruitmentListDBConn.DeleteResearchDataByRecruitmentListID(recruitmentListID); err != nil {
		slog.Error("could not delete all responses", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete all responses"})
		return
	}

	// reset sync info
	if err := h.recruitmentListDBConn.ResetDataSyncTime(recruitmentListID); err != nil {
		slog.Error("could not reset data sync", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not reset data sync"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "data sync reset"})
}

func (h *HttpEndpoints) getParticipants(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "50")
	pageInt, err := strconv.ParseInt(page, 10, 64)
	if err != nil {
		slog.Error("could not parse page", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not parse page"})
		return
	}
	limitInt, err := strconv.ParseInt(limit, 10, 64)
	if err != nil {
		slog.Error("could not parse limit", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not parse limit"})
		return
	}

	// filters:
	includedSinceFilter := c.DefaultQuery("includedSince", "")
	var includedSince *time.Time
	if includedSinceFilter != "" {
		t, err := time.Parse(time.RFC3339, includedSinceFilter)
		if err != nil {
			slog.Error("could not parse includedSince", slog.String("error", err.Error()))
		} else {
			includedSince = &t
		}
	}
	includedUntilFilter := c.DefaultQuery("includedUntil", "")
	var includedUntil *time.Time
	if includedUntilFilter != "" {
		t, err := time.Parse(time.RFC3339, includedUntilFilter)
		if err != nil {
			slog.Error("could not parse includedUntil", slog.String("error", err.Error()))
		} else {
			includedUntil = &t
		}
	}
	participantIDFilter := c.DefaultQuery("participantId", "")
	recruitmentStatusFilter := c.DefaultQuery("recruitmentStatus", "")

	// sort config
	sortBy := c.DefaultQuery("sortBy", "includedAt")
	sortOrder := c.DefaultQuery("sortDir", "asc")

	pFilter := rdb.ParticipantFilter{
		IncludedSince:     includedSince,
		IncludedUntil:     includedUntil,
		ParticipantID:     participantIDFilter,
		RecruitmentStatus: recruitmentStatusFilter,
	}

	sort := rdb.ParticipantSort{
		Field: sortBy,
		Order: sortOrder,
	}

	slog.Info("get participants", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID), slog.String("page", page), slog.String("limit", limit), slog.Any("filter", pFilter), slog.Any("sort", sort))

	participants, paginationInfo, err := h.recruitmentListDBConn.GetParticipantsByRecruitmentListID(recruitmentListID, pageInt, limitInt, pFilter, sort)
	if err != nil {
		slog.Error("could not get participants", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get participants"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"participants": participants,
		"pagination":   paginationInfo,
	})
}

func (h *HttpEndpoints) getParticipant(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	participantID := c.Param("participantID")
	if participantID == "" {
		slog.Warn("no participantID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no participantID"})
		return
	}

	slog.Info("get participant", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID), slog.String("participantID", participantID))

	participant, err := h.recruitmentListDBConn.GetParticipantByID(participantID, recruitmentListID)
	if err != nil {
		slog.Error("could not get participant", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get participant"})
		return
	}

	c.JSON(http.StatusOK, participant)
}

type UpdateParticipantStatusRequest struct {
	Status string `json:"status"`
}

func (h *HttpEndpoints) updateParticipantStatus(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}
	participantID := c.Param("participantID")
	if participantID == "" {
		slog.Warn("no participantID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no participantID"})
		return
	}

	var req UpdateParticipantStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Info("update participant status", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID), slog.String("participantID", participantID))

	if err := h.recruitmentListDBConn.UpdateParticipantStatus(participantID, recruitmentListID, req.Status); err != nil {
		slog.Error("could not update participant status", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update participant status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "participant status updated"})
}

func (h *HttpEndpoints) getParticipantNotes(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	participantID := c.Param("participantID")
	if participantID == "" {
		slog.Warn("no participantID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no participantID"})
		return
	}

	slog.Info("get participant notes", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID), slog.String("participantID", participantID))

	notes, err := h.recruitmentListDBConn.GetParticipantNotes(participantID, recruitmentListID)
	if err != nil {
		slog.Error("could not get participant notes", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get participant notes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"notes": notes})
}

type AddParticipantNoteRequest struct {
	Note string `json:"note"`
}

func (h *HttpEndpoints) addParticipantNote(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	participantID := c.Param("participantID")
	if participantID == "" {
		slog.Warn("no participantID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no participantID"})
		return
	}

	var req AddParticipantNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Info("add participant note", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID), slog.String("participantID", participantID))

	if req.Note == "" {
		slog.Warn("no note")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no note"})
		return
	}

	_, err := h.recruitmentListDBConn.GetParticipantByID(participantID, recruitmentListID)
	if err != nil {
		slog.Error("could not get participant", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get participant"})
		return
	}

	creator, err := h.recruitmentListDBConn.GetResearcherUserByID(token.Subject)
	if err != nil {
		slog.Error("could not get creator", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get creator"})
		return
	}

	creatorName := creator.Username + " (" + creator.Email + ")"

	if _, err := h.recruitmentListDBConn.CreateParticipantNote(participantID, recruitmentListID, req.Note, token.Subject, creatorName); err != nil {
		slog.Error("could not create participant note", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create participant note"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "participant note added"})
}

func (h *HttpEndpoints) deleteParticipantNote(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	participantID := c.Param("participantID")
	if participantID == "" {
		slog.Warn("no participantID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no participantID"})
		return
	}

	noteID := c.Param("noteID")
	if noteID == "" {
		slog.Warn("no noteID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no noteID"})
		return
	}

	slog.Info("delete participant note", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID), slog.String("participantID", participantID), slog.String("noteID", noteID))

	note, err := h.recruitmentListDBConn.GetParticipantNoteByID(noteID)
	if err != nil {
		slog.Error("could not get participant note", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get participant note"})
		return
	}

	if note.CreatedByID != token.Subject || !token.IsAdmin {
		permissions, err := h.recruitmentListDBConn.GetSpecificPermissionsByUserID(token.Subject, []string{pc.ACTION_MANAGE_RECRUITMENT_LIST}, []string{recruitmentListID})
		if err != nil {
			slog.Error("could not get permissions", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get permissions"})
			return
		}

		if len(permissions) == 0 {
			slog.Error("user not allowed to delete participant note", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID), slog.String("participantID", participantID), slog.String("noteID", noteID))
			c.JSON(http.StatusForbidden, gin.H{"error": "user not allowed to delete participant note"})
			return
		}
	}

	err = h.recruitmentListDBConn.DeleteParticipantNoteByID(noteID)
	if err != nil {
		slog.Error("could not delete participant note", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete participant note"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "participant note deleted"})
}

func (h *HttpEndpoints) getAvailableResponses(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	pidFilter := c.DefaultQuery("pid", "")
	startDateQuery := c.DefaultQuery("startDate", "")
	endDateQuery := c.DefaultQuery("endDate", "")

	slog.Info("get available responses", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID))

	var startDate *time.Time
	if startDateQuery != "" {
		t, err := time.Parse(time.RFC3339, startDateQuery)
		if err != nil {
			slog.Error("could not parse startDate", slog.String("error", err.Error()))
		} else {
			startDate = &t
		}
	}

	var endDate *time.Time
	if endDateQuery != "" {

		t, err := time.Parse(time.RFC3339, endDateQuery)
		if err != nil {
			slog.Error("could not parse endDate", slog.String("error", err.Error()))
		} else {
			endDate = &t
		}
	}

	infos, err := h.recruitmentListDBConn.GetAvailableResponseDataInfos(recruitmentListID, pidFilter, startDate, endDate)
	if err != nil {
		slog.Error("could not get available responses", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get available responses"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"infos": infos})
}

func (h *HttpEndpoints) getDownloads(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}
	slog.Info("get downloads", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID))

	downloads, err := h.recruitmentListDBConn.GetDownloadsForRecruitmentList(recruitmentListID)
	if err != nil {
		slog.Error("could not get downloads", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get downloads"})
		return
	}

	if err := h.recruitmentListDBConn.MarkPendingDownloadsAsError(); err != nil {
		slog.Error("could not mark pending downloads as error", slog.String("error", err.Error()))
	}

	c.JSON(http.StatusOK, gin.H{"downloads": downloads})
}

type StartResponseDownloadRequest struct {
	SurveyKey     string     `json:"surveyKey"`
	ParticipantID string     `json:"participantId"`
	StartDate     *time.Time `json:"startDate"`
	EndDate       *time.Time `json:"endDate"`
	Format        string     `json:"format"`
}

func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

func (h *HttpEndpoints) startResponseDownload(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	slog.Info("start response download", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID))

	var req StartResponseDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ext := ".csv"
	if req.Format == rdb.FILE_TYPE_JSON {
		ext = ".json"
	}
	filename := "responses_" + req.SurveyKey + "_" + time.Now().Format("2006-01-02-15-04-05") + ext
	exportFolder := ""
	path := filepath.Join(exportFolder, filename)

	filterInfo := req.SurveyKey + " "
	if req.StartDate != nil {
		filterInfo += "from " + req.StartDate.Format("2006-01-02") + " "
	}
	if req.EndDate != nil {
		filterInfo += "to " + req.EndDate.Format("2006-01-02") + " "
	}
	if req.ParticipantID != "" {
		filterInfo += "for participant " + req.ParticipantID + " "
	}

	downloadInfo, err := h.recruitmentListDBConn.CreateDownload(
		recruitmentListID,
		filterInfo,
		req.Format,
		filename,
		path,
		token.Subject,
	)
	if err != nil {
		slog.Error("could not create download", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create download"})
		return
	}

	go func() {
		fullpath := h.getFullFilePath(path)
		file, err := os.Create(fullpath)
		if err != nil {
			slog.Error("could not create file", slog.String("file path", fullpath), slog.String("error", err.Error()))
			if err := h.recruitmentListDBConn.UpdateDownloadStatus(downloadInfo.ID.Hex(), rdb.DOWNLOAD_STATUS_ERROR); err != nil {
				slog.Error("could not update download status", slog.String("error", err.Error()))
			}
			return
		}
		defer file.Close()

		filter := bson.M{
			"recruitmentListId": recruitmentListID,
			"surveyKey":         req.SurveyKey,
		}
		if req.ParticipantID != "" {
			filter["participantId"] = req.ParticipantID
		}
		if req.StartDate != nil && req.EndDate != nil {
			filter["arrivedAt"] = bson.M{"$gte": req.StartDate.Unix(), "$lte": req.EndDate.Unix()}
		}

		if req.Format == rdb.FILE_TYPE_CSV {
			firstCols := []string{
				"ID", "participantID", "submitted",
			}

			headerSet := make(map[string]struct{})

			if err := h.recruitmentListDBConn.IterateOnResponseData(
				filter,
				func(responseData *rdb.ResponseData) error {
					for key := range responseData.Response {
						if !contains(firstCols, key) {
							headerSet[key] = struct{}{}
						}
					}
					return nil
				},
			); err != nil {
				slog.Error("unexpected error", slog.String("error", err.Error()))
			}

			// Convert the remaining set to a slice and sort it
			var otherHeaders []string
			for key := range headerSet {
				otherHeaders = append(otherHeaders, key)
			}
			sort.Slice(otherHeaders, func(i, j int) bool {
				return strings.ToLower(otherHeaders[i]) < strings.ToLower(otherHeaders[j])
			})

			// Combine firstColumns with otherHeaders
			headers := append(firstCols, otherHeaders...)

			writer := csv.NewWriter(file)
			defer writer.Flush()

			err = writer.Write(headers)
			if err != nil {
				slog.Error("failed to write header", slog.String("error", err.Error()))
				if err := h.recruitmentListDBConn.UpdateDownloadStatus(downloadInfo.ID.Hex(), rdb.DOWNLOAD_STATUS_ERROR); err != nil {
					slog.Error("could not update download status", slog.String("error", err.Error()))
				}
				return
			}

			if err := h.recruitmentListDBConn.IterateOnResponseData(
				filter,
				func(responseData *rdb.ResponseData) error {
					record := []string{}
					record = append(record, responseData.ResponseID)
					record = append(record, responseData.ParticipantID)
					record = append(record, strconv.FormatInt(responseData.Response["submitted"].(int64), 10))

					for _, key := range otherHeaders {
						valueStr := ""
						value, ok := responseData.Response[key]
						if !ok {
							valueStr = ""
						} else {
							switch typedValue := value.(type) {
							case string:
								valueStr = typedValue
							case float64:
								valueStr = strconv.FormatFloat(typedValue, 'f', -1, 64)
							case int64:
								valueStr = strconv.FormatInt(typedValue, 10)
							case bool:
								valueStr = strconv.FormatBool(typedValue)
							}
						}
						record = append(record, valueStr)
					}
					err = writer.Write(record)
					if err != nil {
						slog.Error("failed to write to export file", slog.String("error", err.Error()))
						return err
					}
					return nil
				},
			); err != nil {
				slog.Error("unexpected error", slog.String("error", err.Error()))
			}
		} else if req.Format == rdb.FILE_TYPE_JSON {
			_, err = file.WriteString("{\"responses\": [")
			if err != nil {
				slog.Error("failed to write header", slog.String("error", err.Error()))
				if err := h.recruitmentListDBConn.UpdateDownloadStatus(downloadInfo.ID.Hex(), rdb.DOWNLOAD_STATUS_ERROR); err != nil {
					slog.Error("could not update download status", slog.String("error", err.Error()))
				}
				return
			}

			counter := 0

			if err := h.recruitmentListDBConn.IterateOnResponseData(
				filter,
				func(responseData *rdb.ResponseData) error {
					if counter > 0 {
						_, err = file.WriteString(",")
						if err != nil {
							slog.Error("failed to write to export file", slog.String("error", err.Error()))
							return err
						}
					}

					rJSON, err := json.Marshal(responseData.Response)
					if err != nil {
						slog.Error("failed to marshal report", slog.String("error", err.Error()))
						return nil
					}
					_, err = file.Write(rJSON)
					if err != nil {
						slog.Error("failed to write to export file", slog.String("error", err.Error()))
						return err
					}
					counter++

					return nil
				},
			); err != nil {
				slog.Error("could not iterate on response data", slog.String("error", err.Error()))
			}
			_, err = file.WriteString("]}")
			if err != nil {
				slog.Error("failed to write footer", slog.String("error", err.Error()))
				_, err = file.WriteString("]}")
				if err != nil {
					slog.Error("failed to write footer", slog.String("error", err.Error()))
				}
			}
		}

		if err := h.recruitmentListDBConn.UpdateDownloadStatus(downloadInfo.ID.Hex(), rdb.DONWLOAD_STATUS_AVAILABLE); err != nil {
			slog.Error("could not update download status", slog.String("error", err.Error()))
		}
	}()

	c.JSON(http.StatusOK, downloadInfo)
}

type StartParticipantInfosDownloadRequest struct {
	Format string `json:"format"`
}

func (h *HttpEndpoints) startParticipantInfosDownload(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	var req StartParticipantInfosDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Info("start participant infos download", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID), slog.String("format", req.Format))

	ext := ".csv"
	if req.Format == rdb.FILE_TYPE_JSON {
		ext = ".json"
	} else {
		req.Format = rdb.FILE_TYPE_CSV
	}

	filename := "participant-infos_" + time.Now().Format("2006-01-02-15-04-05") + ext
	exportFolder := ""
	path := filepath.Join(exportFolder, filename)

	filterInfo := "Participant infos"

	downloadInfo, err := h.recruitmentListDBConn.CreateDownload(
		recruitmentListID,
		filterInfo,
		req.Format,
		filename,
		path,
		token.Subject,
	)
	if err != nil {
		slog.Error("could not create download", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create download"})
		return
	}

	go func() {
		fullpath := h.getFullFilePath(path)
		file, err := os.Create(fullpath)
		if err != nil {
			slog.Error("could not create file", slog.String("file path", fullpath), slog.String("error", err.Error()))
			if err := h.recruitmentListDBConn.UpdateDownloadStatus(downloadInfo.ID.Hex(), rdb.DOWNLOAD_STATUS_ERROR); err != nil {
				slog.Error("could not update download status", slog.String("error", err.Error()))
			}
			return
		}
		defer file.Close()

		if req.Format == rdb.FILE_TYPE_CSV {
			rlInfos, err := h.recruitmentListDBConn.GetRecruitmentListByID(recruitmentListID)
			if err != nil {
				slog.Error("could not get recruitment list", slog.String("error", err.Error()))
				if err := h.recruitmentListDBConn.UpdateDownloadStatus(downloadInfo.ID.Hex(), rdb.DOWNLOAD_STATUS_ERROR); err != nil {
					slog.Error("could not update download status", slog.String("error", err.Error()))
				}
				return
			}

			writer := csv.NewWriter(file)
			defer writer.Flush()

			// Prepare header line
			record := []string{}
			record = append(record, "Participant ID")
			record = append(record, "Recruitment Status")
			record = append(record, "Imported At")
			record = append(record, "Deleted At")
			for _, infoDef := range rlInfos.ParticipantData.ParticipantInfos {
				record = append(record, infoDef.Label)
			}
			err = writer.Write(record)
			if err != nil {
				slog.Error("failed to write header", slog.String("error", err.Error()))
				if err := h.recruitmentListDBConn.UpdateDownloadStatus(downloadInfo.ID.Hex(), rdb.DOWNLOAD_STATUS_ERROR); err != nil {
					slog.Error("could not update download status", slog.String("error", err.Error()))
				}
				return
			}

			// Content
			if err := h.recruitmentListDBConn.IterateParticipantsByRecruitmentListID(
				recruitmentListID,
				func(participant *rdb.Participant) error {
					record := []string{}
					record = append(record, participant.ParticipantID)
					record = append(record, participant.RecruitmentStatus)
					record = append(record, participant.IncludedAt.Format(time.RFC3339))

					if participant.DeletedAt != nil {
						record = append(record, participant.DeletedAt.Format(time.RFC3339))
					} else {
						record = append(record, "")
					}

					for _, infoDef := range rlInfos.ParticipantData.ParticipantInfos {
						valueStr := ""
						value, ok := participant.Infos[infoDef.Label]
						if !ok {
							valueStr = ""
						} else {
							switch typedValue := value.(type) {
							case string:
								valueStr = typedValue
							case float64:
								valueStr = strconv.FormatFloat(typedValue, 'f', -1, 64)
							case int64:
								valueStr = strconv.FormatInt(typedValue, 10)
							case bool:
								valueStr = strconv.FormatBool(typedValue)
							}
						}
						record = append(record, valueStr)
					}
					err = writer.Write(record)
					if err != nil {
						slog.Error("failed to write to export file", slog.String("error", err.Error()))
						return err
					}
					return nil
				},
			); err != nil {
				slog.Error("could not iterate on response data", slog.String("error", err.Error()))
			}
		} else if req.Format == rdb.FILE_TYPE_JSON {
			_, err = file.WriteString("{\"participantInfos\": [")
			if err != nil {
				slog.Error("failed to write header", slog.String("error", err.Error()))
				if err := h.recruitmentListDBConn.UpdateDownloadStatus(downloadInfo.ID.Hex(), rdb.DOWNLOAD_STATUS_ERROR); err != nil {
					slog.Error("could not update download status", slog.String("error", err.Error()))
				}
				return
			}

			counter := 0

			if err := h.recruitmentListDBConn.IterateParticipantsByRecruitmentListID(
				recruitmentListID,
				func(participant *rdb.Participant) error {
					if counter > 0 {
						_, err = file.WriteString(",")
						if err != nil {
							slog.Error("failed to write to export file", slog.String("error", err.Error()))
							return err
						}
					}

					rJSON, err := json.Marshal(participant)
					if err != nil {
						slog.Error("failed to marshal report", slog.String("error", err.Error()))
						return nil
					}
					_, err = file.Write(rJSON)
					if err != nil {
						slog.Error("failed to write to export file", slog.String("error", err.Error()))
						return err
					}
					counter++

					return nil
				},
			); err != nil {
				slog.Error("could not iterate on response data", slog.String("error", err.Error()))
			}
			_, err = file.WriteString("]}")
			if err != nil {
				slog.Error("failed to write footer", slog.String("error", err.Error()))
				_, err = file.WriteString("]}")
				if err != nil {
					slog.Error("failed to write footer", slog.String("error", err.Error()))
				}
			}
		}

		if err := h.recruitmentListDBConn.UpdateDownloadStatus(downloadInfo.ID.Hex(), rdb.DONWLOAD_STATUS_AVAILABLE); err != nil {
			slog.Error("could not update download status", slog.String("error", err.Error()))
		}
	}()

	c.JSON(http.StatusOK, downloadInfo)

}

func (h *HttpEndpoints) getDownload(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	downloadID := c.Param("downloadID")
	if downloadID == "" {
		slog.Warn("no downloadID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no downloadID"})
		return
	}

	slog.Info("get download", slog.String("userID", token.Subject), slog.String("downloadID", downloadID))

	download, err := h.recruitmentListDBConn.GetDownloadByID(downloadID)
	if err != nil {
		slog.Error("could not get download", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get download"})
		return
	}

	c.JSON(http.StatusOK, download)
}

func (h *HttpEndpoints) serveDownloadFile(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	downloadID := c.Param("downloadID")
	if downloadID == "" {
		slog.Warn("no downloadID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no downloadID"})
		return
	}

	slog.Info("serve download file", slog.String("userID", token.Subject), slog.String("downloadID", downloadID))

	download, err := h.recruitmentListDBConn.GetDownloadByID(downloadID)
	if err != nil {
		slog.Error("could not get download", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get download"})
		return
	}

	if download.Status == rdb.DOWNLOAD_STATUS_PREPARING {
		slog.Error("download is preparing", slog.String("userID", token.Subject), slog.String("downloadID", downloadID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "download is preparing"})
		return
	}

	fullpath := h.getFullFilePath(download.Path)

	// Check if file exists
	if _, err := os.Stat(fullpath); os.IsNotExist(err) {
		slog.Error("file does not exist", slog.String("path", fullpath))
		c.JSON(http.StatusNotFound, gin.H{"error": "file does not exist"})
		return
	}

	// Return file from file system
	c.Header("Content-Disposition", "attachment; filename="+download.FileName)
	c.Header("Content-Type", download.FileType)
	c.File(fullpath)
}

func (h *HttpEndpoints) deleteDownload(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	downloadID := c.Param("downloadID")
	if downloadID == "" {
		slog.Warn("no downloadID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no downloadID"})
		return
	}

	slog.Info("delete download", slog.String("userID", token.Subject), slog.String("downloadID", downloadID))

	download, err := h.recruitmentListDBConn.GetDownloadByID(downloadID)
	if err != nil {
		slog.Error("could not get download", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get download"})
		return
	}
	if download.Status == rdb.DOWNLOAD_STATUS_PREPARING {
		slog.Error("download is preparing", slog.String("userID", token.Subject), slog.String("downloadID", downloadID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "download is preparing"})
		return
	}

	if err := h.deleteFileAtPath(download.Path); err != nil {
		slog.Error("could not delete file", slog.String("file path", download.Path), slog.String("error", err.Error()))
	}

	if err := h.recruitmentListDBConn.DeleteDownload(downloadID); err != nil {
		slog.Error("could not delete download", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete download"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "download deleted"})
}

func (h *HttpEndpoints) deleteRecruitmentList(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	recruitmentListID := c.Param("id")
	if recruitmentListID == "" {
		slog.Warn("no recruitmentListID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no recruitmentListID"})
		return
	}

	slog.Info("delete recruitment list", slog.String("userID", token.Subject), slog.String("recruitmentListID", recruitmentListID))

	if err := h.recruitmentListDBConn.DeleteRecruitmentListByID(recruitmentListID); err != nil {
		slog.Error("could not delete recruitment list", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete recruitment list"})
		return
	}
	if err := h.recruitmentListDBConn.DeletePermissionsByResourceID(recruitmentListID); err != nil {
		slog.Error("could not delete permissions", slog.String("error", err.Error()))
	}

	if err := h.recruitmentListDBConn.DeleteAllParticipantsByRecruitmentListID(recruitmentListID); err != nil {
		slog.Error("could not delete participants", slog.String("error", err.Error()))
	}

	if err := h.recruitmentListDBConn.DeleteSyncInfosByRecruitmentListID(recruitmentListID); err != nil {
		slog.Error("could not delete sync infos", slog.String("error", err.Error()))
	}

	if err := h.recruitmentListDBConn.DeleteResearchDataByRecruitmentListID(recruitmentListID); err != nil {
		slog.Error("could not delete research data", slog.String("error", err.Error()))
	}

	if err := h.recruitmentListDBConn.DeleteParticipantNotesByRecruitmentListID(recruitmentListID); err != nil {
		slog.Error("could not delete participant notes", slog.String("error", err.Error()))
	}

	downloads, err := h.recruitmentListDBConn.GetDownloadsForRecruitmentList(recruitmentListID)
	if err == nil {
		for _, download := range downloads {
			// remove file at full path
			if err := h.deleteFileAtPath(download.Path); err != nil {
				slog.Error("could not delete file", slog.String("file path", download.Path), slog.String("error", err.Error()))
			}

			if err := h.recruitmentListDBConn.DeleteDownload(download.ID.Hex()); err != nil {
				slog.Error("could not delete download", slog.String("error", err.Error()))
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "recruitment list deleted",
	})
}
