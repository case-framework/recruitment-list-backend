package recruitmentlist

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (dbService *RecruitmentListDBService) collectionSyncInfos() *mongo.Collection {
	return dbService.DBClient.Database(dbService.getDBName()).Collection(COL_NAME_SYNC_INFOS)
}

type SyncInfo struct {
	ID                       string     `json:"id,omitempty" bson:"_id,omitempty"`
	RecruitmentListID        string     `json:"recruitmentListId,omitempty" bson:"recruitmentListId,omitempty"`
	ParticipantSyncStatus    string     `json:"participantSyncStatus,omitempty" bson:"participantSyncStatus,omitempty"`
	ParticipantSyncStartedAt *time.Time `json:"participantSyncStartedAt,omitempty" bson:"participantSyncStartedAt,omitempty"`

	DataSyncStatus    string     `json:"dataSyncStatus,omitempty" bson:"dataSyncStatus,omitempty"`
	DataSyncStartedAt *time.Time `json:"dataSyncStartedAt,omitempty" bson:"dataSyncStartedAt,omitempty"`
}

const (
	SYNC_STATUS_IDLE    = "idle"
	SYNC_STATUS_RUNNING = "running"
)

func (dbService *RecruitmentListDBService) GetSyncInfoByRLID(recruitmentListID string) (*SyncInfo, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	var syncInfo SyncInfo
	err := dbService.collectionSyncInfos().FindOne(ctx, bson.M{"recruitmentListId": recruitmentListID}).Decode(&syncInfo)
	return &syncInfo, err
}

func (dbService *RecruitmentListDBService) StartParticipantSync(recruitmentListID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	filter := bson.M{"recruitmentListId": recruitmentListID}
	update := bson.M{"$set": bson.M{
		"participantSyncStatus":    SYNC_STATUS_RUNNING,
		"participantSyncStartedAt": time.Now(),
	}}
	opts := options.Update()
	opts.SetUpsert(true)
	_, err := dbService.collectionSyncInfos().UpdateOne(ctx, filter, update, opts)
	return err
}

func (dbService *RecruitmentListDBService) FinishParticipantSync(recruitmentListID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	filter := bson.M{"recruitmentListId": recruitmentListID}
	update := bson.M{"$set": bson.M{
		"participantSyncStatus": SYNC_STATUS_IDLE,
	}}
	_, err := dbService.collectionSyncInfos().UpdateOne(ctx, filter, update)
	return err
}

func (dbService *RecruitmentListDBService) ResetParticipantSyncTime(recruitmentListID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	filter := bson.M{"recruitmentListId": recruitmentListID, "participantSyncStatus": SYNC_STATUS_IDLE}
	update := bson.M{"$set": bson.M{
		"participantSyncStartedAt": nil,
	}}
	_, err := dbService.collectionSyncInfos().UpdateOne(ctx, filter, update)
	return err
}

func (dbService *RecruitmentListDBService) StartDataSync(recruitmentListID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	filter := bson.M{"recruitmentListId": recruitmentListID}
	update := bson.M{"$set": bson.M{
		"dataSyncStatus":    SYNC_STATUS_RUNNING,
		"dataSyncStartedAt": time.Now(),
	}}
	opts := options.Update().SetUpsert(true)
	_, err := dbService.collectionSyncInfos().UpdateOne(ctx, filter, update, opts)
	return err
}

func (dbService *RecruitmentListDBService) FinishDataSync(recruitmentListID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	filter := bson.M{"recruitmentListId": recruitmentListID}
	update := bson.M{"$set": bson.M{
		"dataSyncStatus": SYNC_STATUS_IDLE,
	}}
	_, err := dbService.collectionSyncInfos().UpdateOne(ctx, filter, update)
	return err
}

func (dbService *RecruitmentListDBService) ResetDataSyncTime(recruitmentListID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	filter := bson.M{"recruitmentListId": recruitmentListID, "dataSyncStatus": SYNC_STATUS_IDLE}
	update := bson.M{"$set": bson.M{
		"dataSyncStartedAt": nil,
	}}
	_, err := dbService.collectionSyncInfos().UpdateOne(ctx, filter, update)
	return err
}

func (dbService *RecruitmentListDBService) DeleteSyncInfosByRecruitmentListID(recruitmentListID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_, err := dbService.collectionSyncInfos().DeleteMany(ctx, bson.M{"recruitmentListId": recruitmentListID})
	return err
}
