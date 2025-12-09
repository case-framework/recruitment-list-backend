package recruitmentlist

import (
	"context"
	"log/slog"
	"time"

	"github.com/case-framework/case-backend/pkg/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	COL_NAME_RESEARCHER_USERS  = "researcher_users"
	COL_NAME_PERMISSIONS       = "permissions"
	COL_NAME_SESSIONS          = "researcher_user_sessions"
	COL_NAME_RECRUITMENT_LIST  = "recruitment_lists"
	COL_NAME_SYNC_INFOS        = "sync_infos"
	COL_NAME_PARTICIPANTS      = "participants"
	COL_NAME_PARTICIPANT_NOTES = "participant_notes"
	COL_NAME_RESEARCH_DATA     = "research_data"
	COL_NAME_DOWNLOADS         = "downloads"
)

const (
	REMOVE_SESSIONS_AFTER = 60 * 60 * 24 * 2 // 2 days
)

type RecruitmentListDBService struct {
	DBClient        *mongo.Client
	timeout         int
	noCursorTimeout bool
	DBNamePrefix    string
	InstanceIDs     []string
}

func NewRecruitmentListDBService(configs db.DBConfig) (*RecruitmentListDBService, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(configs.Timeout)*time.Second)
	defer cancel()

	dbClient, err := mongo.Connect(ctx,
		options.Client().ApplyURI(configs.URI),
		options.Client().SetMaxConnIdleTime(time.Duration(configs.IdleConnTimeout)*time.Second),
		options.Client().SetMaxPoolSize(configs.MaxPoolSize),
	)

	if err != nil {
		return nil, err
	}

	ctx, conCancel := context.WithTimeout(context.Background(), time.Duration(configs.Timeout)*time.Second)
	err = dbClient.Ping(ctx, nil)
	defer conCancel()

	if err != nil {
		return nil, err
	}

	rDBSc := &RecruitmentListDBService{
		DBClient:        dbClient,
		timeout:         configs.Timeout,
		noCursorTimeout: configs.NoCursorTimeout,
		DBNamePrefix:    configs.DBNamePrefix,
		InstanceIDs:     configs.InstanceIDs,
	}

	if configs.RunIndexCreation {
		if err := rDBSc.ensureIndexes(); err != nil {
			slog.Error("Error ensuring indexes for recruitment list DV", slog.String("error", err.Error()))
		}
	}

	return rDBSc, nil
}

func (dbService *RecruitmentListDBService) getDBName() string {
	return dbService.DBNamePrefix + "recruitment_lists"
}

func (dbService *RecruitmentListDBService) getContext() (ctx context.Context, cancel context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(dbService.timeout)*time.Second)
}

func (dbService *RecruitmentListDBService) ensureIndexes() error {
	slog.Debug("Ensuring indexes for recruitment list db")

	ctx, cancel := dbService.getContext()
	defer cancel()

	// create unique index for sub
	_, err := dbService.collectionResearcherUsers().Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys:    bson.D{{Key: "sub", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	)
	if err != nil {
		slog.Error("Error creating unique index for sub", slog.String("error", err.Error()))
	}

	// create index for sessions
	_, err = dbService.collectionSessions().Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys:    bson.D{{Key: "createdAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(REMOVE_SESSIONS_AFTER),
		},
	)
	if err != nil {
		slog.Error("Error creating index for createdAt sessions: ", slog.String("error", err.Error()))
	}

	// create index for permissions
	if err := dbService.createIndexesForPermissions(); err != nil {
		slog.Error("Error creating indexes for permissions: ", slog.String("error", err.Error()))
	}

	// create index for participants
	if err := dbService.createIndexesForParticipants(); err != nil {
		slog.Error("Error creating indexes for participants: ", slog.String("error", err.Error()))
	}

	// create index for research data
	if err := dbService.createIndexesForResearchData(); err != nil {
		slog.Error("Error creating indexes for research data: ", slog.String("error", err.Error()))
	}
	return nil
}
