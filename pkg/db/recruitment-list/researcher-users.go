package recruitmentlist

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (dbService *RecruitmentListDBService) collectionResearcherUsers() *mongo.Collection {
	return dbService.DBClient.Database(dbService.getDBName()).Collection(COL_NAME_RESEARCHER_USERS)
}

func (dbService *RecruitmentListDBService) CountResearcherUsers() (int, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	count, err := dbService.collectionResearcherUsers().CountDocuments(ctx, bson.M{})
	return int(count), err
}

func (dbService *RecruitmentListDBService) CreateResearcherUser(
	sub string,
	email string,
	username string,
	imageURL string,
	isAdmin bool,
) (*ResearcherUser, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	user := ResearcherUser{
		Sub:         sub,
		Email:       email,
		Username:    username,
		ImageURL:    imageURL,
		IsAdmin:     isAdmin,
		LastLoginAt: time.Now(),
		CreatedAt:   time.Now(),
	}
	_, err := dbService.collectionResearcherUsers().InsertOne(ctx, user)
	return &user, err
}

func (dbService *RecruitmentListDBService) UpdateLastLoginAt(userID string, shouldBeAdmin bool) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}
	filte := bson.M{"_id": _id}
	update := bson.M{"$set": bson.M{"lastLoginAt": time.Now()}}
	if shouldBeAdmin {
		update["$set"] = bson.M{"isAdmin": true}
	}
	_, err = dbService.collectionResearcherUsers().UpdateOne(ctx, filte, update)
	return err
}

func (dbService *RecruitmentListDBService) UpdateResearcherUserIsAdmin(userID string, isAdmin bool) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}
	filte := bson.M{"_id": _id}
	update := bson.M{"$set": bson.M{"isAdmin": isAdmin}}
	_, err = dbService.collectionResearcherUsers().UpdateOne(ctx, filte, update)
	return err
}

func (dbService *RecruitmentListDBService) GetResearcherUserBySub(sub string) (*ResearcherUser, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	var user ResearcherUser
	err := dbService.collectionResearcherUsers().FindOne(ctx, bson.M{"sub": sub}).Decode(&user)
	return &user, err
}

func (dbService *RecruitmentListDBService) GetResearcherUserByID(userID string) (*ResearcherUser, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	var user ResearcherUser
	err = dbService.collectionResearcherUsers().FindOne(ctx, bson.M{"_id": _id}).Decode(&user)
	return &user, err
}

func (dbService *RecruitmentListDBService) GetResearchers() ([]ResearcherUser, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	var users []ResearcherUser
	cur, err := dbService.collectionResearcherUsers().Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	if err := cur.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, err
}

func (dbService *RecruitmentListDBService) DeleteResearcherUser(userID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}
	_, err = dbService.collectionResearcherUsers().DeleteOne(ctx, bson.M{"_id": _id})
	return err
}
