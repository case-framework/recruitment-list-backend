package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	sDB "github.com/case-framework/case-backend/pkg/db/study"
	httpclient "github.com/case-framework/case-backend/pkg/http-client"
	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
	rDB "github.com/case-framework/recruitment-list-backend/pkg/db/recruitment-list"
	"go.mongodb.org/mongo-driver/bson"
)

var (
	HttpClient *httpclient.ClientConfig
)

func SyncParticipantsForRL(
	rdb *rDB.RecruitmentListDBService,
	studyDB *sDB.StudyDBService,
	recruitmentListID string,
	instanceID string,
) error {
	recruitmentList, err := rdb.GetRecruitmentListByID(recruitmentListID)
	if err != nil {
		slog.Error("could not get recruitment list", slog.String("error", err.Error()))
		return err
	}

	lastDataSyncInfo, err := rdb.GetSyncInfoByRLID(recruitmentListID)
	if err != nil {
		slog.Debug("could not get sync info", slog.String("error", err.Error()))
		old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		lastDataSyncInfo = &rDB.SyncInfo{
			DataSyncStartedAt: &old,
		}
	}

	if lastDataSyncInfo.ParticipantSyncStartedAt != nil && time.Now().Add(-5*time.Minute).Before(*lastDataSyncInfo.ParticipantSyncStartedAt) {
		slog.Warn("last sync started less than 5 minutes ago, skipping sync")
		return errors.New("last sync started less than 5 minutes ago, skipping sync")
	}

	if err := rdb.StartParticipantSync(recruitmentListID); err != nil {
		slog.Error("could not start participant sync", slog.String("error", err.Error()))
		return err
	}

	if recruitmentList.ParticipantInclusion.Type != rDB.PARTICIPANT_INCLUSION_TYPE_AUTO {
		slog.Info("participant inclusion type is manual, skipping sync", slog.String("recruitmentListID", recruitmentListID))
		if err := rdb.FinishParticipantSync(recruitmentListID); err != nil {
			slog.Error("could not finish participant sync", slog.String("error", err.Error()))
			return err
		}
		return nil
	}

	studyKey := recruitmentList.ParticipantInclusion.StudyKey

	filter := bson.M{
		"studyStatus": bson.M{"$nin": []string{studyTypes.PARTICIPANT_STUDY_STATUS_ACCOUNT_DELETED}},
	}

	if recruitmentList.ParticipantInclusion.AutoConfig != nil {
		if recruitmentList.ParticipantInclusion.AutoConfig.StartDate != nil &&
			recruitmentList.ParticipantInclusion.AutoConfig.EndDate != nil {

			filter["$and"] = bson.A{
				bson.M{"enteredAt": bson.M{"$lte": recruitmentList.ParticipantInclusion.AutoConfig.EndDate.Unix()}},
				bson.M{"enteredAt": bson.M{"$gte": recruitmentList.ParticipantInclusion.AutoConfig.StartDate.Unix()}},
			}
		}
	}

	useInclusionCriteria := false
	var inclusionCriteria *CriteriaGroup
	if recruitmentList.ParticipantInclusion.AutoConfig != nil && recruitmentList.ParticipantInclusion.AutoConfig.Criteria != "" {
		useInclusionCriteria = true
		inclusionCriteria, err = NewCriteriaGroupFromJSON(recruitmentList.ParticipantInclusion.AutoConfig.Criteria)
		if err != nil {
			slog.Error("could not parse inclusion criteria", slog.String("error", err.Error()))
			return err
		}
	}

	sort := bson.M{}

	newParticipantCounter := 0

	if err := studyDB.FindAndExecuteOnParticipantsStates(
		context.Background(),
		instanceID,
		studyKey,
		filter,
		sort,
		false,
		func(dbService *sDB.StudyDBService, p studyTypes.Participant, instanceID string, studyKey string, args ...interface{}) error {
			if rdb.ParticipantExists(p.ParticipantID, recruitmentListID) {
				slog.Debug("participant already included", slog.String("pid", p.ParticipantID), slog.String("recruitmentListID", recruitmentListID))
				return nil
			}
			if useInclusionCriteria && inclusionCriteria != nil {
				if !checkCriteria(inclusionCriteria, p) {
					return nil
				}
			}
			_, err = rdb.CreateParticipant(p.ParticipantID, recruitmentListID, "auto")
			if err != nil {
				slog.Error("could not create participant", slog.String("error", err.Error()))
				return err
			}
			newParticipantCounter++
			return nil
		},
	); err != nil {
		slog.Error("unexpected error", slog.String("error", err.Error()))
	}

	if newParticipantCounter > 0 && len(recruitmentList.ParticipantInclusion.NotificationEmails) > 0 {
		subject := fmt.Sprintf("[%s] - New participants", recruitmentList.Name)
		message := fmt.Sprintf("One new participant has been added to recruitment list '%s'", recruitmentList.Name)
		if newParticipantCounter > 1 {
			message = fmt.Sprintf("%d new participants have been added to recruitment list '%s'", newParticipantCounter, recruitmentList.Name)
		}
		err := sendEmail(recruitmentList.ParticipantInclusion.NotificationEmails, subject, message)
		if err != nil {
			slog.Error("could not send email", slog.String("error", err.Error()))
		}
	}

	if err := rdb.FinishParticipantSync(recruitmentListID); err != nil {
		slog.Error("could not finish participant sync", slog.String("error", err.Error()))
		return err
	}

	return nil
}

func checkCriteria(criteria *CriteriaGroup, participant studyTypes.Participant) bool {
	val := criteria.Operator == AND

	for _, cond := range criteria.Conditions {
		switch cond := cond.(type) {
		case Condition:
			val = evaluateCondition(cond, participant)
		case *CriteriaGroup:
			val = checkCriteria(cond, participant)
		}
		if criteria.Operator == AND {
			if !val {
				return false
			}
		} else if criteria.Operator == OR {
			if val {
				return true
			}
		}
	}
	return val
}

func evaluateCondition(condition Condition, participant studyTypes.Participant) bool {
	switch condition.Type {
	case FlagExists:
		_, ok := participant.Flags[condition.Key]
		return ok
	case FlagHasValue:
		val, ok := participant.Flags[condition.Key]
		return ok && condition.Value != nil && val == *condition.Value
	case FlagNotExists:
		_, ok := participant.Flags[condition.Key]
		return !ok
	case FlagNotHasValue:
		val, ok := participant.Flags[condition.Key]
		return !ok && condition.Value != nil && val != *condition.Value
	case HasStatus:
		return condition.Value != nil && participant.StudyStatus == *condition.Value
	default:
		return false
	}
}

type SendEmailReq struct {
	To       []string `json:"to"`
	Subject  string   `json:"subject"`
	Content  string   `json:"content"`
	HighPrio bool     `json:"highPrio"`
}

func sendEmail(recipients []string, subject string, message string) error {
	if HttpClient == nil || HttpClient.RootURL == "" {
		return errors.New("connection to smtp bridge not initialized")
	}

	sendEmailReq := SendEmailReq{
		To:       recipients,
		Subject:  subject,
		Content:  message,
		HighPrio: false,
	}
	resp, err := HttpClient.RunHTTPcall("/send-email", sendEmailReq)
	if err == nil && resp != nil {
		errMsg, hasError := resp["error"]
		if hasError {
			return errors.New(errMsg.(string))
		}
	}
	return err
}
