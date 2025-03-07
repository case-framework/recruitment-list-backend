package apihandlers

import (
	"log/slog"
	"net/http"
	"time"

	mw "github.com/case-framework/case-backend/pkg/apihelpers/middlewares"
	jwthandling "github.com/case-framework/case-backend/pkg/jwt-handling"
	rdb "github.com/case-framework/recruitment-list-backend/pkg/db/recruitment-list"
	"github.com/gin-gonic/gin"
)

func (h *HttpEndpoints) AddResearcherUserManagementAPI(rg *gin.RouterGroup) {
	authGroup := rg.Group("/auth")
	authGroup.POST("/signin-with-idp", h.signInWithIdP)
	authGroup.POST("/extend-session",
		mw.RequirePayload(),
		mw.GetAndValidateManagementUserJWT(h.tokenSignKey),
		h.extendSession)
	authGroup.GET("/renew-token/:sessionID",
		mw.GetAndValidateManagementUserJWT(h.tokenSignKey),
		h.getRenewTokenForSession)

	authGroup.GET("/permissions", mw.GetAndValidateManagementUserJWT(h.tokenSignKey), h.getMyPermissions)

	researcherUsersGroup := rg.Group("/researchers")
	researcherUsersGroup.Use(mw.GetAndValidateManagementUserJWT(h.tokenSignKey))
	researcherUsersGroup.GET("", h.getResearchers)
	researcherUsersGroup.GET("/:researcherID", mw.IsAdminUser(), h.getResearcher)
	researcherUsersGroup.PUT("/:researcherID/is-admin", mw.IsAdminUser(), h.updateResearcherIsAdmin)
	researcherUsersGroup.DELETE("/:researcherID", mw.IsAdminUser(), h.deleteResearcher)
	researcherUsersGroup.GET("/:researcherID/permissions", mw.IsAdminUser(), h.getPermissionsForResearcher)
	researcherUsersGroup.POST("/:researcherID/permissions", mw.IsAdminUser(), h.createPermissionForResearcher)
	researcherUsersGroup.DELETE("/:researcherID/permissions/:permissionID", mw.IsAdminUser(), h.deletePermissionForResearcher)
}

// SignInRequest is the request body for the signin-with-idp endpoint
type SignInRequest struct {
	Sub        string   `json:"sub"`
	Roles      []string `json:"roles"`
	Name       string   `json:"name"`
	Email      string   `json:"email"`
	ImageURL   string   `json:"imageUrl"`
	RenewToken string   `json:"renewToken"`
}

// singInWithIdP to generate a new token and update the user in the database
func (h *HttpEndpoints) signInWithIdP(c *gin.Context) {
	var req SignInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Sub == "" {
		slog.Error("sub is empty")
		c.JSON(http.StatusBadRequest, gin.H{"error": "sub is empty"})
		return
	}

	shouldBeAdmin := false
	for _, role := range req.Roles {
		if role == "ADMIN" {
			shouldBeAdmin = true
		}
	}
	if !shouldBeAdmin {
		count, err := h.recruitmentListDBConn.CountResearcherUsers()
		if err != nil {
			slog.Error("failed to count researcher users", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// if first user:
		shouldBeAdmin = count == 0
	}

	existingUser, err := h.recruitmentListDBConn.GetResearcherUserBySub(req.Sub)
	if err != nil || existingUser == nil {
		slog.Info("sign up with new user", slog.String("sub", req.Sub), slog.String("email", req.Email))
		existingUser, err = h.recruitmentListDBConn.CreateResearcherUser(
			req.Sub,
			req.Email,
			req.Name,
			req.ImageURL,
			shouldBeAdmin,
		)
		if err != nil {
			slog.Error("failed to create researcher user", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		slog.Info("sign in with existing user", slog.String("sub", req.Sub), slog.String("email", req.Email))
		if err := h.recruitmentListDBConn.UpdateLastLoginAt(existingUser.ID.Hex(), shouldBeAdmin); err != nil {
			slog.Error("failed to update last login at", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	sessionId := ""

	if req.RenewToken != "" {
		session, err := h.recruitmentListDBConn.CreateSession(existingUser.ID.Hex(), req.RenewToken)
		if err != nil {
			slog.Error("could not create session", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create session"})
			return
		}
		sessionId = session.ID.Hex()
	}

	// generate new JWT token
	token, err := jwthandling.GenerateNewManagementUserToken(
		h.ttls.AccessToken,
		existingUser.ID.Hex(),
		"",
		existingUser.IsAdmin,
		map[string]string{},
		h.tokenSignKey,
	)
	if err != nil {
		slog.Error("could not generate token", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accessToken": token,
		"sessionID":   sessionId,
		"expiresAt":   time.Now().Add(h.ttls.AccessToken).Unix(),
		"isAdmin":     existingUser.IsAdmin,
	})
}

type ExtendSessionRequest struct {
	RenewToken string `json:"renewToken"`
}

func (h *HttpEndpoints) extendSession(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	var req ExtendSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sessionId := ""

	// Create new session
	if req.RenewToken != "" {
		session, err := h.recruitmentListDBConn.CreateSession(token.Subject, req.RenewToken)
		if err != nil {
			slog.Error("could not create session", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create session"})
			return
		}
		sessionId = session.ID.Hex()
	}

	if err := h.recruitmentListDBConn.UpdateLastLoginAt(token.Subject, false); err != nil {
		slog.Error("failed to update last login at", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// generate new JWT token
	newAccessToken, err := jwthandling.GenerateNewManagementUserToken(
		h.ttls.AccessToken,
		token.Subject,
		token.InstanceID,
		token.IsAdmin,
		map[string]string{},
		h.tokenSignKey,
	)
	if err != nil {
		slog.Error("could not generate token", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
		return
	}

	slog.Info("extended session", slog.String("userID", token.Subject), slog.String("instanceID", token.InstanceID))

	c.JSON(http.StatusOK, gin.H{
		"accessToken": newAccessToken,
		"sessionID":   sessionId,
		"expiresAt":   time.Now().Add(h.ttls.AccessToken).Unix(),
		"isAdmin":     token.IsAdmin,
	})
}

// getRenewToken to get a the renew token for the user
func (h *HttpEndpoints) getRenewTokenForSession(c *gin.Context) {
	sessionID := c.Param("sessionID")
	if sessionID == "" {
		slog.Warn("no sessionID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no sessionID"})
		return
	}

	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	existingSession, err := h.recruitmentListDBConn.GetSession(sessionID)
	if err != nil {
		slog.Debug("could not get session", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get session"})
		return
	}
	if existingSession.UserID != token.Subject {
		slog.Warn("user not allowed to get renew token", slog.String("userID", token.Subject), slog.String("sessionUserID", existingSession.UserID))
		c.JSON(http.StatusForbidden, gin.H{"error": "user not allowed to get renew token"})
		return
	}

	slog.Info("got renew token", slog.String("userID", token.Subject), slog.String("instanceID", token.InstanceID))

	c.JSON(http.StatusOK, gin.H{"renewToken": existingSession.RenewToken})
}

func (h *HttpEndpoints) getMyPermissions(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	slog.Info("get my permissions", slog.String("userID", token.Subject))

	permissions, err := h.recruitmentListDBConn.GetPermissionsByUserID(token.Subject)
	if err != nil {
		slog.Error("could not get permissions", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"permissions": permissions})
}

func (h *HttpEndpoints) getResearchers(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	slog.Info("get researchers", slog.String("userID", token.Subject))

	researchers, err := h.recruitmentListDBConn.GetResearchers()
	if err != nil {
		slog.Error("could not get researchers", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get researchers"})
		return
	}

	resp := gin.H{
		"researchers": []rdb.ResearcherUser{},
	}
	if token.IsAdmin {
		resp["researchers"] = researchers
	} else {
		for _, researcher := range researchers {
			r := rdb.ResearcherUser{
				ID:       researcher.ID,
				Username: researcher.Username,
				Email:    researcher.Email,
				ImageURL: researcher.ImageURL,
			}
			resp["researchers"] = append(resp["researchers"].([]rdb.ResearcherUser), r)
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *HttpEndpoints) getResearcher(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	researcherID := c.Param("researcherID")
	if researcherID == "" {
		slog.Warn("no researcherID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no researcherID"})
		return
	}

	slog.Info("get researcher", slog.String("userID", token.Subject), slog.String("researcherID", researcherID))

	researcher, err := h.recruitmentListDBConn.GetResearcherUserByID(researcherID)
	if err != nil {
		slog.Error("could not get researcher", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get researcher"})
		return
	}

	c.JSON(http.StatusOK, researcher)
}

type UpdateResearcherIsAdminRequest struct {
	IsAdmin bool `json:"isAdmin"`
}

func (h *HttpEndpoints) updateResearcherIsAdmin(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	researcherID := c.Param("researcherID")
	if researcherID == "" {
		slog.Warn("no researcherID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no researcherID"})
		return
	}

	var req UpdateResearcherIsAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if token.Subject == researcherID {
		slog.Warn("user not allowed to update himself", slog.String("userID", token.Subject))
		c.JSON(http.StatusForbidden, gin.H{"error": "user not allowed to update himself"})
		return
	}

	slog.Info("update researcher isAdmin", slog.String("userID", token.Subject), slog.String("researcherID", researcherID))

	if err := h.recruitmentListDBConn.UpdateResearcherUserIsAdmin(researcherID, req.IsAdmin); err != nil {
		slog.Error("could not update researcher isAdmin", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update researcher isAdmin"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"isAdmin": req.IsAdmin})
}

func (h *HttpEndpoints) deleteResearcher(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	researcherID := c.Param("researcherID")
	if researcherID == "" {
		slog.Warn("no researcherID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no researcherID"})
		return
	}

	slog.Info("delete researcher", slog.String("userID", token.Subject), slog.String("researcherID", researcherID))

	if token.Subject == researcherID {
		slog.Warn("user not allowed to delete himself", slog.String("userID", token.Subject))
		c.JSON(http.StatusForbidden, gin.H{"error": "user not allowed to delete himself"})
		return
	}

	if err := h.recruitmentListDBConn.DeleteResearcherUser(researcherID); err != nil {
		slog.Error("could not delete researcher", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete researcher"})
		return
	}

	if err := h.recruitmentListDBConn.DeletePermissionsByUserID(researcherID); err != nil {
		slog.Error("could not delete permissions", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

func (h *HttpEndpoints) getPermissionsForResearcher(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	researcherID := c.Param("researcherID")
	if researcherID == "" {
		slog.Warn("no researcherID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no researcherID"})
		return
	}

	slog.Info("get permissions for researcher", slog.String("userID", token.Subject), slog.String("researcherID", researcherID))

	permissions, err := h.recruitmentListDBConn.GetPermissionsByUserID(researcherID)
	if err != nil {
		slog.Error("could not get permissions", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"permissions": permissions})
}

type CreatePermissionRequest struct {
	UserID     string              `json:"userId"`
	Action     string              `json:"action"`
	ResourceID string              `json:"resourceId"`
	Limiter    []map[string]string `json:"limiter"`
}

func (h *HttpEndpoints) createPermissionForResearcher(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	researcherID := c.Param("researcherID")
	if researcherID == "" {
		slog.Warn("no researcherID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no researcherID"})
		return
	}

	var req CreatePermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.UserID = researcherID
	if req.UserID == "" {
		slog.Warn("no userID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no userID"})
		return
	}
	if req.Action == "" {
		slog.Warn("no action")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no action"})
		return
	}

	slog.Info("create permission for researcher", slog.String("userID", token.Subject), slog.String("researcherID", researcherID))

	p, err := h.recruitmentListDBConn.CreatePermission(req.UserID, req.Action, req.ResourceID, token.Subject, req.Limiter)
	if err != nil {
		slog.Error("could not create permission", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create permission"})
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *HttpEndpoints) deletePermissionForResearcher(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	researcherID := c.Param("researcherID")
	if researcherID == "" {
		slog.Warn("no researcherID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no researcherID"})
		return
	}

	permissionID := c.Param("permissionID")
	if permissionID == "" {
		slog.Warn("no permissionID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no permissionID"})
		return
	}

	slog.Info("delete permission for researcher", slog.String("userID", token.Subject), slog.String("researcherID", researcherID), slog.String("permissionID", permissionID))

	perm, err := h.recruitmentListDBConn.GetPermissionByID(permissionID)
	if err != nil {
		slog.Error("could not get permission for researcher", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get permission for researcher"})
		return
	}

	if perm.UserID != researcherID {
		slog.Warn("unexpected userID", slog.String("userID", token.Subject), slog.String("researcherID", researcherID), slog.String("permissionID", permissionID))
		c.JSON(http.StatusForbidden, gin.H{"error": "unexpected userID"})
		return
	}

	if err := h.recruitmentListDBConn.DeletePermissionByID(permissionID); err != nil {
		slog.Error("could not delete permission for researcher", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete permission for researcher"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "permission deleted"})
}
