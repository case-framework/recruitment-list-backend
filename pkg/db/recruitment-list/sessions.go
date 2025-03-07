package recruitmentlist

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (dbService *RecruitmentListDBService) collectionSessions() *mongo.Collection {
	return dbService.DBClient.Database(dbService.getDBName()).Collection(COL_NAME_SESSIONS)
}

// Session represents a user session, created when a user logs in
func (dbService *RecruitmentListDBService) CreateSession(
	userID string,
	renewToken string,
) (*Session, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()
	session := &Session{
		UserID:     userID,
		RenewToken: renewToken,
		CreatedAt:  time.Now(),
	}

	res, err := dbService.collectionSessions().InsertOne(ctx, session)
	if err != nil {
		return nil, err
	}
	session.ID = res.InsertedID.(primitive.ObjectID)
	return session, nil
}

// GetSession returns the session with the given ID
func (dbService *RecruitmentListDBService) GetSession(
	sessionID string,
) (*Session, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	var session Session
	objID, err := primitive.ObjectIDFromHex(sessionID)
	if err != nil {
		return nil, err
	}
	err = dbService.collectionSessions().FindOne(ctx, primitive.M{"_id": objID}).Decode(&session)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// DeleteSession deletes the session with the given ID
func (dbService *RecruitmentListDBService) DeleteSession(
	sessionID string,
) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(sessionID)
	if err != nil {
		return err
	}
	_, err = dbService.collectionSessions().DeleteOne(ctx, primitive.M{"_id": objID})
	return err
}

// DeleteSessionsByUserID deletes all sessions for the given user
func (dbService *RecruitmentListDBService) DeleteSessionsByUserID(
	userID string,
) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_, err := dbService.collectionSessions().DeleteMany(ctx, primitive.M{"userId": userID})
	return err
}
