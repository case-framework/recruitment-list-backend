package recruitmentlist

import (
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (dbService *RecruitmentListDBService) collectionPermissions() *mongo.Collection {
	return dbService.DBClient.Database(dbService.getDBName()).Collection(COL_NAME_PERMISSIONS)
}

func (dbService *RecruitmentListDBService) createIndexesForPermissions() error {
	ctx, cancel := dbService.getContext()
	defer cancel()
	_, err := dbService.collectionPermissions().Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys: bson.D{
				{Key: "userId", Value: 1},
				{Key: "action", Value: 1},
				{Key: "resourceId", Value: 1},
			},
		},
	)
	if err != nil {
		slog.Error("Error creating index for permissions: ", slog.String("error", err.Error()))
	}
	return nil
}

func (dbService *RecruitmentListDBService) CreatePermission(
	userID string,
	action string,
	resource string,
	createdBy string,
	limiter []map[string]string,
) (*Permission, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	permission := Permission{
		UserID:     userID,
		Action:     action,
		ResourceID: resource,
		CreatedAt:  time.Now(),
		CreatedBy:  createdBy,
		Limiter:    limiter,
	}
	res, err := dbService.collectionPermissions().InsertOne(ctx, permission)
	if err != nil {
		return nil, err
	}
	permission.ID = res.InsertedID.(primitive.ObjectID)
	return &permission, nil
}

func (dbService *RecruitmentListDBService) GetPermissionByID(permissionID string) (*Permission, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(permissionID)
	if err != nil {
		return nil, err
	}

	var permission Permission
	err = dbService.collectionPermissions().FindOne(ctx, bson.M{"_id": _id}).Decode(&permission)
	return &permission, err
}

func (dbService *RecruitmentListDBService) GetPermissionsByUserID(userID string) ([]Permission, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	var permissions []Permission
	cur, err := dbService.collectionPermissions().Find(ctx, bson.M{"userId": userID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	if err := cur.All(ctx, &permissions); err != nil {
		return nil, err
	}
	return permissions, err
}

func (dbService *RecruitmentListDBService) GetPermissionsByResourceID(resourceID string) ([]Permission, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	var permissions []Permission
	cur, err := dbService.collectionPermissions().Find(ctx, bson.M{"resourceId": resourceID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	if err := cur.All(ctx, &permissions); err != nil {
		return nil, err
	}
	return permissions, err
}

func (dbService *RecruitmentListDBService) GetSpecificPermissionsByUserID(userID string, actions []string, resources []string) ([]Permission, error) {
	ctx, cancel := dbService.getContext()
	defer cancel()

	filter := bson.M{"userId": userID}
	if len(actions) > 0 {
		filter["action"] = bson.M{"$in": actions}
	}
	if len(resources) > 0 {
		filter["resourceId"] = bson.M{"$in": resources}
	}
	var permissions []Permission
	cur, err := dbService.collectionPermissions().Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	if err := cur.All(ctx, &permissions); err != nil {
		return nil, err
	}
	return permissions, err
}

func (dbService *RecruitmentListDBService) DeletePermissionByID(permissionID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(permissionID)
	if err != nil {
		return err
	}
	_, err = dbService.collectionPermissions().DeleteOne(ctx, bson.M{"_id": _id})
	return err
}

func (dbService *RecruitmentListDBService) DeletePermissionsByUserID(userID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_, err := dbService.collectionPermissions().DeleteMany(ctx, bson.M{"userId": userID})
	return err
}

func (dbService *RecruitmentListDBService) DeletePermissionsByResourceID(resourceID string) error {
	ctx, cancel := dbService.getContext()
	defer cancel()

	_, err := dbService.collectionPermissions().DeleteMany(ctx, bson.M{"resourceId": resourceID})
	return err
}
