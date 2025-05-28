package recruitmentlist

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	PARTICIPANT_INCLUSION_TYPE_MANUAL = "manual"
	PARTICIPANT_INCLUSION_TYPE_AUTO   = "auto"
)

type ResearcherUser struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Sub         string             `json:"sub,omitempty" bson:"sub,omitempty"`
	Email       string             `json:"email,omitempty" bson:"email,omitempty"`
	Username    string             `json:"username,omitempty" bson:"username,omitempty"`
	ImageURL    string             `json:"imageUrl,omitempty" bson:"imageUrl,omitempty"`
	IsAdmin     bool               `json:"isAdmin,omitempty" bson:"isAdmin,omitempty"`
	LastLoginAt time.Time          `json:"lastLoginAt,omitempty" bson:"lastLoginAt,omitempty"`
	CreatedAt   time.Time          `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
}

type Session struct {
	ID         primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UserID     string             `json:"userId,omitempty" bson:"userId,omitempty"`
	RenewToken string             `json:"renewToken,omitempty" bson:"renewToken,omitempty"`
	CreatedAt  time.Time          `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
}

type Permission struct {
	ID         primitive.ObjectID  `json:"id,omitempty" bson:"_id,omitempty"`
	UserID     string              `json:"userId,omitempty" bson:"userId,omitempty"`
	ResourceID string              `json:"resourceId,omitempty" bson:"resourceId,omitempty"`
	Action     string              `json:"action,omitempty" bson:"action,omitempty"`
	Limiter    []map[string]string `json:"limiter,omitempty" bson:"limiter,omitempty"`
	CreatedAt  time.Time           `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
	CreatedBy  string              `json:"createdBy,omitempty" bson:"createdBy,omitempty"`
}

// RecruitmentList

type InclusionAutoConfig struct {
	Criteria  string     `json:"criteria,omitempty" bson:"criteria,omitempty"`
	StartDate *time.Time `json:"startDate,omitempty" bson:"startDate,omitempty"`
	EndDate   *time.Time `json:"endDate,omitempty" bson:"endDate,omitempty"`
}

type ParticipantInclusion struct {
	StudyKey           string               `json:"studyKey,omitempty" bson:"studyKey,omitempty"`
	Type               string               `json:"type,omitempty" bson:"type,omitempty"`
	AutoConfig         *InclusionAutoConfig `json:"autoConfig,omitempty" bson:"autoConfig,omitempty"`
	NotificationEmails []string             `json:"notificationEmails,omitempty" bson:"notificationEmails,omitempty"`
}

// if any of the Participant info fields with the given key equals the given value, the participant will be excluded
type ExclusionCondition struct {
	Key   string `json:"key,omitempty" bson:"key,omitempty"`
	Value string `json:"value,omitempty" bson:"value,omitempty"`
}

type MappingType string

const (
	MappingTypeDefault   MappingType = "default"   // Use value as is
	MappingTypeJSON      MappingType = "json"      // Parse as JSON
	MappingTypeKey2Value MappingType = "key2value" // Use key-value mapping
	MappingTypeTs2Date   MappingType = "ts2date"   // Parse as date
)

type Mapping struct {
	Key   string `json:"key,omitempty" bson:"key,omitempty"`
	Value string `json:"value,omitempty" bson:"value,omitempty"`
}

type ParticipantInfo struct {
	ID            string      `json:"id,omitempty" bson:"id,omitempty"`
	Label         string      `json:"label,omitempty" bson:"label,omitempty"`
	SourceType    string      `json:"sourceType,omitempty" bson:"sourceType,omitempty"`
	SourceKey     string      `json:"sourceKey,omitempty" bson:"sourceKey,omitempty"`
	ShowInPreview bool        `json:"showInPreview,omitempty" bson:"showInPreview,omitempty"`
	MappingType   MappingType `json:"mappingType,omitempty" bson:"mappingType,omitempty"`
	Mapping       []Mapping   `json:"mapping,omitempty" bson:"mapping,omitempty"`
}

type ResearchData struct {
	ID              string     `json:"id,omitempty" bson:"id,omitempty"`
	SurveyKey       string     `json:"surveyKey,omitempty" bson:"surveyKey,omitempty"`
	StartDate       *time.Time `json:"startDate,omitempty" bson:"startDate,omitempty"`
	EndDate         *time.Time `json:"endDate,omitempty" bson:"endDate,omitempty"`
	ExcludedColumns []string   `json:"excludedColumns,omitempty" bson:"excludedColumns,omitempty"`
}

type ParticipantDataConfig struct {
	ParticipantInfos []ParticipantInfo `json:"participantInfos" bson:"participantInfos"`
	ResearchData     []ResearchData    `json:"researchData" bson:"researchData"`
}

type Customization struct {
	RecruitmentStatusValues []string `json:"recruitmentStatusValues,omitempty" bson:"recruitmentStatusValues,omitempty"`
}

type StudyAction struct {
	ID            string `json:"id,omitempty" bson:"id,omitempty"`
	EncodedAction string `json:"encodedAction,omitempty" bson:"encodedAction,omitempty"`
	Label         string `json:"label,omitempty" bson:"label,omitempty"`
	Description   string `json:"description,omitempty" bson:"description,omitempty"`
}

type RecruitmentList struct {
	ID                   primitive.ObjectID    `json:"id,omitempty" bson:"_id,omitempty"`
	Name                 string                `json:"name,omitempty" bson:"name,omitempty"`
	Description          string                `json:"description,omitempty" bson:"description,omitempty"`
	CreatedAt            time.Time             `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
	CreatedBy            string                `json:"createdBy,omitempty" bson:"createdBy,omitempty"`
	ParticipantInclusion ParticipantInclusion  `json:"participantInclusion,omitempty" bson:"participantInclusion,omitempty"`
	ExclusionConditions  []ExclusionCondition  `json:"exclusionConditions,omitempty" bson:"exclusionConditions,omitempty"`
	ParticipantData      ParticipantDataConfig `json:"participantData,omitempty" bson:"participantData,omitempty"`
	Customization        Customization         `json:"customization,omitempty" bson:"customization,omitempty"`
	StudyActions         []StudyAction         `json:"studyActions,omitempty" bson:"studyActions,omitempty"`
}
