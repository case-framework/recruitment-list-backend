package recruitmentlist

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (dbService *RecruitmentListDBService) UpdateRecruitmentListTags(listID string, tags []string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(listID)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id}
	update := bson.M{"$set": bson.M{"tags": tags}}
	_, err = dbService.collectionRecruitmentLists().UpdateOne(ctx, filter, update)
	return err
}

func (dbService *RecruitmentListDBService) GetRecruitmentListTags() ([]string, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$project", Value: bson.M{"tags": 1}}},
		bson.D{{Key: "$unwind", Value: "$tags"}},
		bson.D{{Key: "$group", Value: bson.M{"_id": "$tags"}}},
		bson.D{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}

	cursor, err := dbService.collectionRecruitmentLists().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		Tag string `bson:"_id"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	tags := make([]string, 0, len(results))
	for _, result := range results {
		if result.Tag == "" {
			continue
		}
		tags = append(tags, result.Tag)
	}
	return tags, nil
}
