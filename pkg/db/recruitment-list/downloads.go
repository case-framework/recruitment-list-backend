package recruitmentlist

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	DOWNLOAD_STATUS_PREPARING = "preparing"
	DONWLOAD_STATUS_AVAILABLE = "available"
	DOWNLOAD_STATUS_ERROR     = "error"

	FILE_TYPE_CSV  = "csv"
	FILE_TYPE_JSON = "json"
)

type Download struct {
	ID                primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	RecruitmentListID string             `json:"recruitmentListId,omitempty" bson:"recruitmentListId,omitempty"`
	FilterInfo        string             `json:"filterInfo,omitempty" bson:"filterInfo,omitempty"`
	Status            string             `json:"status,omitempty" bson:"status,omitempty"`
	CreatedAt         time.Time          `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
	CreatedBy         string             `json:"createdBy,omitempty" bson:"createdBy,omitempty"`
	FileType          string             `json:"fileType,omitempty" bson:"fileType,omitempty"`
	FileName          string             `json:"fileName,omitempty" bson:"fileName,omitempty"`
	Path              string             `json:"path,omitempty" bson:"path,omitempty"`
}

func (dbService *RecruitmentListDBService) collectionDownloads() *mongo.Collection {
	return dbService.DBClient.Database(dbService.getDBName()).Collection(COL_NAME_DOWNLOADS)
}

func (dbService *RecruitmentListDBService) CreateDownload(
	recruitmentListID string,
	filterInfo string,
	fileType string,
	fileName string,
	path string,
	by string,
) (*Download, error) {
	download := Download{
		RecruitmentListID: recruitmentListID,
		FilterInfo:        filterInfo,
		Status:            DOWNLOAD_STATUS_PREPARING,
		CreatedAt:         time.Now(),
		CreatedBy:         by,
		FileType:          fileType,
		FileName:          fileName,
		Path:              path,
	}

	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := dbService.collectionDownloads().InsertOne(ctx, download)
	if err != nil {
		return nil, err
	}
	download.ID = _id.InsertedID.(primitive.ObjectID)
	return &download, nil
}

func (dbService *RecruitmentListDBService) UpdateDownloadStatus(downloadID string, status string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(downloadID)
	if err != nil {
		return err
	}

	_, err = dbService.collectionDownloads().UpdateOne(
		ctx,
		bson.M{"_id": _id},
		bson.M{"$set": bson.M{"status": status}},
	)
	if err != nil {
		return err
	}
	return nil
}

func (dbService *RecruitmentListDBService) GetDownloadByID(downloadID string) (*Download, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(downloadID)
	if err != nil {
		return nil, err
	}

	var download Download
	err = dbService.collectionDownloads().FindOne(ctx, bson.M{"_id": _id}).Decode(&download)
	if err != nil {
		return nil, err
	}
	return &download, nil
}

func (dbService *RecruitmentListDBService) GetDownloadsForRecruitmentList(recruitmentListID string) ([]Download, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	var downloads []Download
	cur, err := dbService.collectionDownloads().Find(ctx, bson.M{"recruitmentListId": recruitmentListID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	if err := cur.All(ctx, &downloads); err != nil {
		return nil, err
	}
	return downloads, nil
}

func (dbService *RecruitmentListDBService) DeleteDownload(downloadID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(downloadID)
	if err != nil {
		return err
	}

	_, err = dbService.collectionDownloads().DeleteOne(ctx, bson.M{"_id": _id})
	if err != nil {
		return err
	}
	return nil
}

func (dbService *RecruitmentListDBService) MarkPendingDownloadsAsError() error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	filter := bson.M{
		"status": DOWNLOAD_STATUS_PREPARING,
		"createdAt": bson.M{
			"$lt": time.Now().Add(-time.Minute * 40),
		},
	}

	_, err := dbService.collectionDownloads().UpdateMany(
		ctx,
		filter,
		bson.M{"$set": bson.M{"status": DOWNLOAD_STATUS_ERROR}},
	)
	if err != nil {
		return err
	}
	return nil
}
