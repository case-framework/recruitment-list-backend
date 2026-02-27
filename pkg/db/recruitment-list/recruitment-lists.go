package recruitmentlist

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (dbService *RecruitmentListDBService) collectionRecruitmentLists() *mongo.Collection {
	return dbService.DBClient.Database(dbService.getDBName()).Collection(COL_NAME_RECRUITMENT_LIST)
}

func (dbService *RecruitmentListDBService) CreateRecruitmentList(
	recruitmentList RecruitmentList,
	by string,
) (*RecruitmentList, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	recruitmentList.CreatedAt = time.Now()
	recruitmentList.CreatedBy = by
	_id, err := dbService.collectionRecruitmentLists().InsertOne(ctx, recruitmentList)
	if err != nil {
		return nil, err
	}
	recruitmentList.ID = _id.InsertedID.(primitive.ObjectID)
	return &recruitmentList, nil
}

func (dbService *RecruitmentListDBService) SaveRecruitmentList(
	recruitmentList RecruitmentList,
) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_, err := dbService.collectionRecruitmentLists().ReplaceOne(ctx, bson.M{"_id": recruitmentList.ID}, recruitmentList)
	return err
}

func (dbService *RecruitmentListDBService) GetRecruitmentListByID(listID string) (*RecruitmentList, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(listID)
	if err != nil {
		return nil, err
	}

	var list RecruitmentList
	err = dbService.collectionRecruitmentLists().FindOne(ctx, bson.M{"_id": _id}).Decode(&list)
	return &list, err
}

func (dbService *RecruitmentListDBService) UpdateRecruitmentListStudyActions(listID string, studyActions []StudyAction) error {
	ctx, cancel := dbService.getContext()
	defer cancel()
	_id, err := primitive.ObjectIDFromHex(listID)
	if err != nil {
		return err
	}
	filter := bson.M{"_id": _id}
	update := bson.M{"$set": bson.M{"studyActions": studyActions}}
	_, err = dbService.collectionRecruitmentLists().UpdateOne(ctx, filter, update)
	return err
}

func (dbService *RecruitmentListDBService) GetRecruitmentListsInfos() ([]RecruitmentList, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	filter := bson.M{}
	projection := bson.M{
		"id":          1,
		"name":        1,
		"description": 1,
		"tags":        1,
		"createdAt":   1,
	}
	opts := options.Find()
	opts.SetProjection(projection)
	cur, err := dbService.collectionRecruitmentLists().Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var lists []RecruitmentList
	if err := cur.All(ctx, &lists); err != nil {
		return nil, err
	}
	return lists, err
}

func (dbService *RecruitmentListDBService) FindAndExecuteOnRecruitmentLists(
	filter bson.M,
	callback func(list *RecruitmentList) error,
) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	cur, err := dbService.collectionRecruitmentLists().Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var list RecruitmentList
		if err := cur.Decode(&list); err != nil {
			return err
		}
		if err := callback(&list); err != nil {
			return err
		}
	}
	return nil
}

func (dbService *RecruitmentListDBService) DeleteRecruitmentListByID(listID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(listID)
	if err != nil {
		return err
	}
	_, err = dbService.collectionRecruitmentLists().DeleteOne(ctx, bson.M{"_id": _id})
	return err
}
