package recruitmentlist

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (dbService *RecruitmentListDBService) collectionParticipantNotes() *mongo.Collection {
	return dbService.DBClient.Database(dbService.getDBName()).Collection(COL_NAME_PARTICIPANT_NOTES)
}

type ParticipantNote struct {
	ID                primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	PID               string             `json:"pid,omitempty" bson:"pid,omitempty"`
	RecruitmentListID string             `json:"recruitmentListId,omitempty" bson:"recruitmentListId,omitempty"`
	Note              string             `json:"note,omitempty" bson:"note,omitempty"`
	CreatedAt         time.Time          `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
	CreatedByID       string             `json:"createdById,omitempty" bson:"createdById,omitempty"`
	CreatedBy         string             `json:"createdBy,omitempty" bson:"createdBy,omitempty"`
}

func (dbService *RecruitmentListDBService) CreateParticipantNote(
	pid string,
	recruitmentListID string,
	note string,
	createdByID string,
	createdBy string,
) (*ParticipantNote, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	participantNote := ParticipantNote{
		PID:               pid,
		RecruitmentListID: recruitmentListID,
		Note:              note,
		CreatedAt:         time.Now(),
		CreatedByID:       createdByID,
		CreatedBy:         createdBy,
	}
	_id, err := dbService.collectionParticipantNotes().InsertOne(ctx, participantNote)
	if err != nil {
		return nil, err
	}
	participantNote.ID = _id.InsertedID.(primitive.ObjectID)
	return &participantNote, nil
}

func (dbService *RecruitmentListDBService) GetParticipantNotes(
	pid string,
	recruitmentListID string,
) ([]ParticipantNote, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	var participantNotes []ParticipantNote
	cur, err := dbService.collectionParticipantNotes().Find(ctx, bson.M{"pid": pid, "recruitmentListId": recruitmentListID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	if err := cur.All(ctx, &participantNotes); err != nil {
		return nil, err
	}
	return participantNotes, nil
}

func (dbService *RecruitmentListDBService) GetParticipantNoteByID(noteID string) (*ParticipantNote, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(noteID)
	if err != nil {
		return nil, err
	}

	var participantNote ParticipantNote
	err = dbService.collectionParticipantNotes().FindOne(ctx, bson.M{"_id": _id}).Decode(&participantNote)
	return &participantNote, err
}

func (dbService *RecruitmentListDBService) DeleteParticipantNoteByID(noteID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(noteID)
	if err != nil {
		return err
	}

	_, err = dbService.collectionParticipantNotes().DeleteOne(ctx, bson.M{"_id": _id})
	return err
}

func (dbService *RecruitmentListDBService) DeleteParticipantNotesByRecruitmentListID(recruitmentListID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_, err := dbService.collectionParticipantNotes().DeleteMany(ctx, bson.M{"recruitmentListId": recruitmentListID})
	return err
}
