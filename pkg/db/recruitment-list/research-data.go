package recruitmentlist

import (
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ResponseData struct {
	ID                primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	ResponseID        string                 `json:"responseId,omitempty" bson:"responseId,omitempty"`
	ParticipantID     string                 `json:"participantId,omitempty" bson:"participantId,omitempty"`
	RecruitmentListID string                 `json:"recruitmentListId,omitempty" bson:"recruitmentListId,omitempty"`
	SurveyKey         string                 `json:"surveyKey,omitempty" bson:"surveyKey,omitempty"`
	ArrivedAt         int64                  `json:"arrivedAt,omitempty" bson:"arrivedAt,omitempty"`
	Response          map[string]interface{} `json:"response,omitempty" bson:"response,omitempty"`
}

func (dbService *RecruitmentListDBService) collectionResearchData() *mongo.Collection {
	return dbService.DBClient.Database(dbService.getDBName()).Collection(COL_NAME_RESEARCH_DATA)
}

func (dbService *RecruitmentListDBService) createIndexesForResearchData() error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_, err := dbService.collectionResearchData().Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys: bson.D{
				{Key: "participantId", Value: 1},
				{Key: "responseId", Value: 1},
				{Key: "recruitmentListId", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	)
	if err != nil {
		slog.Error("Error creating index for research data: ", slog.String("error", err.Error()))
	}
	return nil
}

func (dbService *RecruitmentListDBService) SaveResearchData(
	recruitmentListID string,
	participantID string,
	researchData []ResponseData,
) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	generic := make([]interface{}, len(researchData))
	for i, rd := range researchData {
		generic[i] = rd
	}

	_, err := dbService.collectionResearchData().InsertMany(ctx, generic)
	return err
}

func (dbService *RecruitmentListDBService) DeleteResearchDataByRecruitmentListID(recruitmentListID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_, err := dbService.collectionResearchData().DeleteMany(ctx, bson.M{"recruitmentListId": recruitmentListID})
	return err
}

type ResponseDataInfo struct {
	SurveyKey      string `json:"surveyKey,omitempty" bson:"_id,omitempty"`
	Count          int64  `json:"count,omitempty" bson:"count,omitempty"`
	FirstArrivedAt int64  `json:"firstArrivedAt,omitempty" bson:"firstArrivedAt,omitempty"`
	LastArrivedAt  int64  `json:"lastArrivedAt,omitempty" bson:"lastArrivedAt,omitempty"`
}

func (dbService *RecruitmentListDBService) GetAvailableResponseDataInfos(
	recruitmentListID string,
	pidFilter string,
	startDateFilter *time.Time,
	endDateFilter *time.Time,
) ([]ResponseDataInfo, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	filter := bson.M{"recruitmentListId": recruitmentListID}
	if pidFilter != "" {
		filter["participantId"] = pidFilter
	}
	if startDateFilter != nil && endDateFilter != nil {
		filter["arrivedAt"] = bson.M{"$gte": startDateFilter.Unix(), "$lte": endDateFilter.Unix()}
	}

	pipeline := []bson.M{
		{"$match": filter},

		{"$group": bson.M{
			"_id":            "$surveyKey",
			"count":          bson.M{"$sum": 1},
			"firstArrivedAt": bson.M{"$min": "$arrivedAt"},
			"lastArrivedAt":  bson.M{"$max": "$arrivedAt"},
		}},
	}
	cursor, err := dbService.collectionResearchData().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []ResponseDataInfo
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

func (dbService *RecruitmentListDBService) IterateOnResponseData(
	filter bson.M,
	callback func(responseData *ResponseData) error,
) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	cur, err := dbService.collectionResearchData().Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var responseData ResponseData
		if err := cur.Decode(&responseData); err != nil {
			slog.Error("could not decode response data", slog.String("error", err.Error()))
			continue
		}
		if err := callback(&responseData); err != nil {
			return err
		}
	}
	return nil
}
