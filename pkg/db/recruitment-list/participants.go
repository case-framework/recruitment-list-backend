package recruitmentlist

import (
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (dbService *RecruitmentListDBService) collectionParticipants() *mongo.Collection {
	return dbService.DBClient.Database(dbService.getDBName()).Collection(COL_NAME_PARTICIPANTS)
}

type Participant struct {
	ID                primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	ParticipantID     string                 `json:"participantId,omitempty" bson:"participantId,omitempty"`
	RecruitmentListID string                 `json:"recruitmentListId,omitempty" bson:"recruitmentListId,omitempty"`
	IncludedAt        time.Time              `json:"includedAt,omitempty" bson:"includedAt,omitempty"`
	IncludedBy        string                 `json:"includedBy,omitempty" bson:"includedBy,omitempty"`
	DeletedAt         *time.Time             `json:"deletedAt,omitempty" bson:"deletedAt,omitempty"`
	RecruitmentStatus string                 `json:"recruitmentStatus" bson:"recruitmentStatus"`
	Infos             map[string]interface{} `json:"infos,omitempty" bson:"infos,omitempty"`
}

func (dbService *RecruitmentListDBService) createIndexesForParticipants() error {
	ctx, cancel := dbService.getContext()
	defer cancel()
	_, err := dbService.collectionParticipants().Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys: bson.D{
				{Key: "participantId", Value: 1},
				{Key: "recruitmentListId", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	)

	return err
}

func (dbService *RecruitmentListDBService) CreateParticipant(
	pid string,
	rlID string,
	by string,
) (*Participant, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	participant := Participant{
		ParticipantID:     pid,
		RecruitmentListID: rlID,
		IncludedAt:        time.Now(),
		IncludedBy:        by,
	}
	_id, err := dbService.collectionParticipants().InsertOne(ctx, participant)
	if err != nil {
		return nil, err
	}
	participant.ID = _id.InsertedID.(primitive.ObjectID)
	return &participant, nil
}

func (dbService *RecruitmentListDBService) ParticipantExists(pid string, rlID string) bool {
	ctx, cancel := dbService.getContext()
	defer cancel()

	var participant Participant
	err := dbService.collectionParticipants().FindOne(ctx, bson.M{"participantId": pid, "recruitmentListId": rlID}).Decode(&participant)
	return err == nil
}

func (dbService *RecruitmentListDBService) GetParticipantByID(pid string, rlID string) (*Participant, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(pid)
	if err != nil {
		return nil, err
	}
	var participant Participant
	err = dbService.collectionParticipants().FindOne(ctx, bson.M{"_id": _id, "recruitmentListId": rlID}).Decode(&participant)
	return &participant, err
}

func (dbService *RecruitmentListDBService) OnParticipantDeleted(p *Participant, rlID string, reason string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	filter := bson.M{"participantId": p.ParticipantID, "recruitmentListId": rlID}
	update := bson.M{"$set": bson.M{"deletedAt": time.Now()}, "$unset": bson.M{"infos": 1}}
	_, err := dbService.collectionParticipants().UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	// Also delete all research data for this participant
	filter = bson.M{"participantId": p.ParticipantID, "recruitmentListId": rlID}
	_, err = dbService.collectionResearchData().DeleteMany(ctx, filter)
	if err != nil {
		slog.Error("could not delete research data", slog.String("error", err.Error()))
	}

	// Add particpant note
	_, err = dbService.CreateParticipantNote(
		p.ID.Hex(),
		rlID,
		fmt.Sprintf("Participant deleted: %s", reason),
		"",
		"<system>",
	)
	if err != nil {
		slog.Error("could not create participant note", slog.String("error", err.Error()))
	}

	return nil
}

func (dbService *RecruitmentListDBService) UpdateParticipantStatus(pid string, rlID string, status string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(pid)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "recruitmentListId": rlID}
	update := bson.M{"$set": bson.M{"recruitmentStatus": status}}
	_, err = dbService.collectionParticipants().UpdateOne(ctx, filter, update)
	return err
}

func (dbService *RecruitmentListDBService) UpdateParticipantInfos(
	pid string,
	rlID string,
	infos map[string]interface{},
) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	filter := bson.M{"participantId": pid, "recruitmentListId": rlID}
	update := bson.M{"$set": bson.M{"infos": infos}}
	_, err := dbService.collectionParticipants().UpdateOne(ctx, filter, update)
	return err
}

func (dbService *RecruitmentListDBService) IterateParticipantsByRecruitmentListID(
	rlID string,
	callback func(participant *Participant) error,
) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	cur, err := dbService.collectionParticipants().Find(ctx, bson.M{"recruitmentListId": rlID})
	if err != nil {
		return err
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var participant Participant
		if err := cur.Decode(&participant); err != nil {
			return err
		}
		if err := callback(&participant); err != nil {
			return err
		}
	}
	return nil
}

func (dbService *RecruitmentListDBService) DeleteAllParticipantsByRecruitmentListID(rlID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_, err := dbService.collectionParticipants().DeleteMany(ctx, bson.M{"recruitmentListId": rlID})
	return err
}

func (dbService *RecruitmentListDBService) CountParticipantsByRecruitmentListID(rlID string) (int, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	count, err := dbService.collectionParticipants().CountDocuments(ctx, bson.M{"recruitmentListId": rlID})
	return int(count), err
}

type PaginationInfos struct {
	TotalCount  int64 `json:"totalCount"`
	CurrentPage int64 `json:"currentPage"`
	TotalPages  int64 `json:"totalPages"`
	PageSize    int64 `json:"pageSize"`
}

type ParticipantFilter struct {
	IncludedSince     *time.Time
	IncludedUntil     *time.Time
	ParticipantID     string
	RecruitmentStatus string
}

type ParticipantSort struct {
	Field string
	Order string
}

func (dbService *RecruitmentListDBService) GetParticipantsByRecruitmentListID(rlID string, page int64, limit int64,
	pFilter ParticipantFilter,
	sort ParticipantSort,
) (participants []Participant, paginationInfo PaginationInfos, err error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	filter := bson.M{"recruitmentListId": rlID}
	if pFilter.IncludedSince != nil && pFilter.IncludedUntil != nil {
		filter["includedAt"] = bson.M{"$gte": pFilter.IncludedSince, "$lte": pFilter.IncludedUntil}
	} else if pFilter.IncludedSince != nil {
		filter["includedAt"] = bson.M{"$gte": pFilter.IncludedSince}
	} else if pFilter.IncludedUntil != nil {
		filter["includedAt"] = bson.M{"$lte": pFilter.IncludedUntil}
	}

	if pFilter.ParticipantID != "" {
		filter["participantId"] = pFilter.ParticipantID
	}
	if pFilter.RecruitmentStatus != "" {
		filter["recruitmentStatus"] = pFilter.RecruitmentStatus
	}

	count, err := dbService.collectionParticipants().CountDocuments(ctx, filter)
	if err != nil {
		return participants, paginationInfo, err
	}

	paginationInfo = prepPaginationInfos(
		count,
		page,
		limit,
	)

	sortBy := bson.D{}
	if sort.Order == "desc" {
		sortBy = append(sortBy, bson.E{Key: sort.Field, Value: -1})
	} else {
		sortBy = append(sortBy, bson.E{Key: sort.Field, Value: 1})
	}
	sortBy = append(sortBy, bson.E{Key: "_id", Value: 1})

	opts := options.Find()
	opts.SetLimit(limit)
	opts.SetSkip((page - 1) * limit)
	opts.SetSort(sortBy)
	cur, err := dbService.collectionParticipants().Find(ctx, filter, opts)
	if err != nil {
		return nil, paginationInfo, err
	}
	defer cur.Close(ctx)

	if err := cur.All(ctx, &participants); err != nil {
		return nil, paginationInfo, err
	}
	return participants, paginationInfo, nil
}
