package sync

import (
	"encoding/json"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"maps"

	sDB "github.com/case-framework/case-backend/pkg/db/study"
	surveydefinition "github.com/case-framework/case-backend/pkg/study/exporter/survey-definition"
	surveyresponses "github.com/case-framework/case-backend/pkg/study/exporter/survey-responses"
	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
	studyutils "github.com/case-framework/case-backend/pkg/study/utils"
	rDB "github.com/case-framework/recruitment-list-backend/pkg/db/recruitment-list"
	"go.mongodb.org/mongo-driver/bson"
)

var (
	responseExporterCache          = make(map[string]*surveyresponses.ResponseParser)
	responseExporterCacheForPInfos = make(map[string]*surveyresponses.ResponseParser)
)

func SyncResearchDataForRL(
	rdb *rDB.RecruitmentListDBService,
	studyDB *sDB.StudyDBService,
	recruitmentListID string,
	instanceID string,
	globalStudySecret string,
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

	if err := rdb.StartDataSync(recruitmentListID); err != nil {
		slog.Error("could not start data sync", slog.String("error", err.Error()))
		return err
	}

	studyKey := recruitmentList.ParticipantInclusion.StudyKey

	resetResponseParserCache()

	if err := rdb.IterateParticipantsByRecruitmentListID(recruitmentListID, func(participant *rDB.Participant) error {
		return SyncDataForParticipant(rdb, studyDB, recruitmentList, participant, instanceID, studyKey, lastDataSyncInfo, globalStudySecret, false)
	}); err != nil {
		slog.Error("could not iterate participants", slog.String("error", err.Error()))
	}

	if err := rdb.FinishDataSync(recruitmentListID); err != nil {
		slog.Error("could not finish data sync", slog.String("error", err.Error()))
		return err
	}

	return nil
}

func SyncDataForParticipant(
	rdb *rDB.RecruitmentListDBService,
	studyDB *sDB.StudyDBService,
	recruitmentList *rDB.RecruitmentList,
	participant *rDB.Participant,
	instanceID string,
	studyKey string,
	lastDataSyncInfo *rDB.SyncInfo,
	globalStudySecret string,
	skipResponseSync bool,
) error {
	if participant.DeletedAt != nil && !participant.DeletedAt.IsZero() {
		slog.Debug("skip deleted participant", slog.String("participantID", participant.ParticipantID))
		return nil
	}

	// fetch participant info from study DB:
	studyParticipant, err := studyDB.GetParticipantByID(instanceID, studyKey, participant.ParticipantID)
	if err != nil {
		slog.Error("could not retrieve participant from study DB", slog.String("error", err.Error()))
		return err
	}

	// check if participant is deleted in study DB:
	if studyParticipant.StudyStatus == studyTypes.PARTICIPANT_STUDY_STATUS_ACCOUNT_DELETED {
		if err := rdb.OnParticipantDeleted(participant, recruitmentList.ID.Hex(), "deleted in study DB"); err != nil {
			slog.Error("could not delete participant", slog.String("error", err.Error()))
			return err
		}
		return nil
	}

	// update participant infos:
	updatedParticipantInfos, err := updateAndSaveParticipantInfos(rdb, studyDB, recruitmentList, instanceID, participant, studyParticipant, lastDataSyncInfo.DataSyncStartedAt, globalStudySecret)
	if err != nil {
		slog.Error("could not update participant infos", slog.String("error", err.Error()))
		return err
	}

	// check and if needed apply exclusion conditions
	if toExclude := CheckExclusionConditions(recruitmentList, updatedParticipantInfos); toExclude {
		if err := rdb.OnParticipantDeleted(participant, recruitmentList.ID.Hex(), "excluded by exclusion conditions"); err != nil {
			slog.Error("could not exclude participant", slog.String("error", err.Error()))
			return err
		}
		slog.Info("excluded participant", slog.String("pid", participant.ParticipantID), slog.String("rlID", recruitmentList.ID.Hex()))
		return nil
	}

	// update participant responses:
	if !skipResponseSync {
		syncNewResponses(rdb, studyDB, recruitmentList, instanceID, participant, lastDataSyncInfo)
	}

	return nil
}

func updateAndSaveParticipantInfos(
	rdb *rDB.RecruitmentListDBService,
	studyDB *sDB.StudyDBService,
	recruitmentList *rDB.RecruitmentList,
	instanceID string,
	participant *rDB.Participant,
	studyParticipant studyTypes.Participant,
	lastDataSync *time.Time,
	globalStudySecret string,
) (map[string]interface{}, error) {
	updatedParticipantInfo := make(map[string]any)

	// populate participant info from current study participant
	maps.Copy(updatedParticipantInfo, participant.Infos)

	// delete keys that not in the expected participant info
	for key := range updatedParticipantInfo {
		deleteEntry := true
		for _, pInfoDef := range recruitmentList.ParticipantData.ParticipantInfos {
			if pInfoDef.Label == key {
				deleteEntry = false
				break
			}
		}
		if deleteEntry {
			delete(updatedParticipantInfo, key)
		}
	}

	lastResponseCache := make(map[string]map[string]interface{})
	lastConfidentialDataCache := make(map[string]studyTypes.SurveyResponse)

	studyKey := recruitmentList.ParticipantInclusion.StudyKey
	study, err := studyDB.GetStudy(instanceID, studyKey)
	if err != nil {
		slog.Error("could not get study", slog.String("error", err.Error()))
		return nil, err
	}

	for _, pInfoDef := range recruitmentList.ParticipantData.ParticipantInfos {
		if pInfoDef.SourceKey == "" {
			slog.Error("sourceKey is empty", slog.String("recruitmentListID", recruitmentList.ID.Hex()), slog.String("label", pInfoDef.Label))
			continue
		}
		if pInfoDef.Label == "" {
			slog.Error("label is empty", slog.String("recruitmentListID", recruitmentList.ID.Hex()), slog.String("sourceKey", pInfoDef.SourceKey))
			continue
		}

		surveyKey := getSurveyKeyFromSourceKey(pInfoDef.SourceKey)

		switch pInfoDef.SourceType {
		case "flagValue":
			val := applyMapping(pInfoDef, studyParticipant.Flags[pInfoDef.SourceKey])
			updatedParticipantInfo[pInfoDef.Label] = val
		case "confidentialData":
			keyParts := strings.Split(pInfoDef.SourceKey, "-")
			if len(keyParts) < 2 {
				slog.Error("invalid source key", slog.String("sourceKey", pInfoDef.SourceKey))
				continue
			}

			itemKey := keyParts[0]
			slotKey := keyParts[1]

			latestConfidentialData, ok := lastConfidentialDataCache[itemKey]
			if !ok {

				confidentialID, err := studyutils.ProfileIDtoParticipantID(participant.ParticipantID, globalStudySecret, study.SecretKey, study.Configs.IdMappingMethod)
				if err != nil {
					slog.Error("failed to get confidential participantID", slog.String("error", err.Error()))
					continue
				}

				newConfidentialData, err := studyDB.FindConfidentialResponses(
					instanceID,
					studyKey,
					confidentialID,
					itemKey,
				)
				if err != nil {

					slog.Debug("could not get confidential data", slog.String("error", err.Error()))
					continue
				}

				if len(newConfidentialData) == 0 {
					slog.Debug("no confidential data found", slog.String("itemKey", itemKey))
					continue
				}

				// Sort by timestamp - latest first
				sort.Slice(newConfidentialData, func(i, j int) bool {
					return newConfidentialData[i].ID.Timestamp().After(newConfidentialData[j].ID.Timestamp())
				})

				latestConfidentialData = newConfidentialData[0]
				lastConfidentialDataCache[itemKey] = latestConfidentialData
			}

			// find item with key
			itemResp, ok := findSurveyItemResponseByItemKey(latestConfidentialData, itemKey)
			if !ok || itemResp == nil {
				slog.Debug("no item response found", slog.String("itemKey", itemKey))
				continue
			}

			root := &studyTypes.ResponseItem{
				Items: []*studyTypes.ResponseItem{
					itemResp.Response,
				},
			}
			val, ok := extractSlotValue(root, slotKey, pInfoDef)
			if !ok {
				slog.Debug("no slot value found", slog.String("slotKey", slotKey))
				continue
			}

			updatedParticipantInfo[pInfoDef.Label] = val
		case "responseData":
			if lastSubmissionForSurveyLaterThan(studyParticipant.LastSubmissions, surveyKey, lastDataSync) {
				responsesFromFilter := int64(0)
				if lastDataSync != nil {
					responsesFromFilter = lastDataSync.Unix()
				}

				lastResponse, ok := lastResponseCache[surveyKey]
				if !ok {
					// fetch latest response data from study DB
					filter := bson.M{
						"participantID": participant.ParticipantID,
						"key":           surveyKey,
						"arrivedAt":     bson.M{"$gte": responsesFromFilter},
					}

					responses, _, err := studyDB.GetResponses(
						instanceID,
						recruitmentList.ParticipantInclusion.StudyKey,
						filter,
						bson.M{"arrivedAt": -1},
						1,
						1,
					)
					if err != nil {
						slog.Error("could not get responses", slog.String("error", err.Error()))
						continue
					}
					if len(responses) == 0 {
						slog.Debug("no responses found", slog.String("participantID", participant.ParticipantID), slog.String("surveyKey", surveyKey))
						continue
					}

					respDef := rDB.ResearchData{
						SurveyKey:       surveyKey,
						ExcludedColumns: []string{},
					}

					parsedResponses, err := responsesToResearchData(responses, studyDB, instanceID, recruitmentList, respDef, participant.ParticipantID, responseExporterCacheForPInfos)
					if err != nil {
						slog.Error("failed to convert responses to research data entries", slog.String("error", err.Error()))
						continue
					}
					lastResponse = parsedResponses[0].Response

					lastResponseCache[surveyKey] = lastResponse
				}

				findResponseEntry, ok := lastResponse[pInfoDef.SourceKey]
				if !ok {
					slog.Debug("no response found", slog.String("surveyKey", surveyKey), slog.String("sourceKey", pInfoDef.SourceKey))
					continue
				}

				switch typedValue := findResponseEntry.(type) {
				case string:
					updatedParticipantInfo[pInfoDef.Label] = typedValue
				case float64:
					updatedParticipantInfo[pInfoDef.Label] = strconv.FormatFloat(typedValue, 'f', -1, 64)
				case int64:
					updatedParticipantInfo[pInfoDef.Label] = strconv.FormatInt(typedValue, 10)
				case bool:
					updatedParticipantInfo[pInfoDef.Label] = strconv.FormatBool(typedValue)
				default:
					jsonStr, err := json.Marshal(findResponseEntry)
					if err != nil {
						slog.Error("failed to marshal response", slog.String("error", err.Error()))
						continue
					}
					updatedParticipantInfo[pInfoDef.Label] = string(jsonStr)
				}
				if pInfoDef.Mapping != nil {
					updatedParticipantInfo[pInfoDef.Label] = applyMapping(pInfoDef, updatedParticipantInfo[pInfoDef.Label].(string))
				}
			}
		default:
			slog.Error("unknown source type", slog.String("sourceType", pInfoDef.SourceType))
		}
	}

	if err := rdb.UpdateParticipantInfos(participant.ParticipantID, recruitmentList.ID.Hex(), updatedParticipantInfo); err != nil {
		slog.Error("could not update participant infos", slog.String("error", err.Error()))
		return nil, err
	}
	return updatedParticipantInfo, nil
}

func CheckExclusionConditions(recruitmentList *rDB.RecruitmentList, updatedParticipantInfo map[string]interface{}) bool {
	for _, cond := range recruitmentList.ExclusionConditions {
		val, ok := updatedParticipantInfo[cond.Key]
		if !ok {
			continue
		}
		if val == cond.Value {
			return true
		}
	}
	return false
}

func findSurveyItemResponseByItemKey(response studyTypes.SurveyResponse, itemKey string) (*studyTypes.SurveyItemResponse, bool) {
	for _, itemResp := range response.Responses {
		if itemResp.Key == itemKey {
			return &itemResp, true
		}
	}
	return nil, false
}

func extractSlotValue(item *studyTypes.ResponseItem, slotKey string, pInfoDef rDB.ParticipantInfo) (string, bool) {
	slotParts := strings.Split(slotKey, ".")

	var currentSlot *studyTypes.ResponseItem
	for _, slot := range item.Items {
		if slot.Key == slotParts[0] {
			currentSlot = slot
			break
		}
	}

	if currentSlot == nil {
		return "", false
	}

	if len(slotParts) == 1 {
		if len(currentSlot.Items) > 0 {
			if pInfoDef.MappingType == rDB.MappingTypeJSON {
				jsonVal, err := json.Marshal(currentSlot.Items)
				if err != nil {
					slog.Error("failed to marshal slot value", slog.String("error", err.Error()))
					return "", false
				}
				return string(jsonVal), true
			}

			vals := make([]string, len(currentSlot.Items))
			for i, item := range currentSlot.Items {
				vals[i] = applyMapping(pInfoDef, item.Key)
			}
			return strings.Join(vals, ","), true

		}
		return applyMapping(pInfoDef, currentSlot.Value), true
	} else {
		return extractSlotValue(currentSlot, strings.Join(slotParts[1:], "."), pInfoDef)
	}
}

func lastSubmissionForSurveyLaterThan(lastSubmissions map[string]int64, surveyKey string, lastSync *time.Time) bool {
	if lastSync == nil {
		return true
	}
	lastSubmission, ok := lastSubmissions[surveyKey]
	if !ok {
		return false
	}
	return lastSubmission > lastSync.Unix()
}

func getSurveyKeyFromSourceKey(sourceKey string) string {
	parts := strings.Split(sourceKey, ".")
	if len(parts) < 1 {
		return ""
	}
	return parts[0]
}

func syncNewResponses(
	rdb *rDB.RecruitmentListDBService,
	studyDB *sDB.StudyDBService,
	recruitmentList *rDB.RecruitmentList,
	instanceID string,
	participant *rDB.Participant,
	lastDataSyncInfo *rDB.SyncInfo,
) {
	if participant == nil {
		slog.Error("participant should not be nil")
		return
	}
	for _, respDef := range recruitmentList.ParticipantData.ResearchData {
		checkResponsesSince := int64(0)
		checkResponsesUntil := time.Now().Unix()

		if respDef.StartDate != nil {
			checkResponsesSince = respDef.StartDate.Unix()
		}
		if respDef.EndDate != nil {
			checkResponsesUntil = respDef.EndDate.Unix()
		}

		lastDataSyncStarted := int64(0)
		if lastDataSyncInfo != nil && lastDataSyncInfo.DataSyncStartedAt != nil {
			lastDataSyncStarted = lastDataSyncInfo.DataSyncStartedAt.Unix()
		}

		applySince := !participant.IncludedAt.After(time.Unix(lastDataSyncStarted, 0))
		if applySince {
			checkResponsesSince = max(checkResponsesSince, lastDataSyncStarted)
		}

		filter := bson.M{
			"participantID": participant.ParticipantID,
			"key":           respDef.SurveyKey,
			"$and": bson.A{
				bson.M{"arrivedAt": bson.M{"$lte": checkResponsesUntil}},
				bson.M{"arrivedAt": bson.M{"$gte": checkResponsesSince}},
			},
		}

		// get responses
		responses, _, err := studyDB.GetResponses(
			instanceID,
			recruitmentList.ParticipantInclusion.StudyKey,
			filter,
			bson.M{
				"arrivedAt": 1,
			},
			1,
			1000,
		)
		if err != nil {
			slog.Error("could not get responses", slog.String("error", err.Error()))
			return
		}

		if len(responses) == 0 {
			slog.Debug("no responses found", slog.String("participantID", participant.ParticipantID), slog.String("surveyKey", respDef.SurveyKey))
			continue
		}

		researchData, err := responsesToResearchData(
			responses,
			studyDB,
			instanceID,
			recruitmentList,
			respDef,
			participant.ParticipantID,
			responseExporterCache,
		)
		if err != nil {
			slog.Error("failed to convert responses to research data entries", slog.String("error", err.Error()))
			continue
		}

		if err := rdb.SaveResearchData(recruitmentList.ID.Hex(), participant.ParticipantID, researchData); err != nil {
			slog.Error("could not save research data", slog.String("error", err.Error()))
			continue
		}
	}
}

func responsesToResearchData(
	responses []studyTypes.SurveyResponse,
	studyDB *sDB.StudyDBService,
	instanceID string,
	recruitmentList *rDB.RecruitmentList,
	respDef rDB.ResearchData,
	participantID string,
	exporterCache map[string]*surveyresponses.ResponseParser,
) (researchData []rDB.ResponseData, err error) {
	studyKey := recruitmentList.ParticipantInclusion.StudyKey
	surveyKey := respDef.SurveyKey

	respParser, ok := exporterCache[respDef.SurveyKey]
	if !ok {
		respParser, err = initResponseParser(
			studyDB,
			instanceID,
			studyKey,
			surveyKey,
			respDef.ExcludedColumns,
		)
		if err != nil {
			slog.Error("failed to create response parser", slog.String("error", err.Error()))
			return
		}
		exporterCache[respDef.SurveyKey] = respParser
	}

	if respParser == nil {
		slog.Error("response parser is nil")
		return
	}

	researchData = make([]rDB.ResponseData, len(responses))

	for i, rawResp := range responses {
		resp, err := respParser.ParseResponse(&rawResp)
		if err != nil {
			slog.Error("failed to parse response", slog.String("error", err.Error()))
			continue
		}
		output, err := respParser.ResponseToFlatObj(resp)
		if err != nil {
			slog.Error("failed to convert response to flat object", slog.String("error", err.Error()))
			continue
		}
		researchData[i] = rDB.ResponseData{
			ResponseID:        rawResp.ID.Hex(),
			ParticipantID:     participantID,
			SurveyKey:         surveyKey,
			ArrivedAt:         rawResp.ArrivedAt,
			RecruitmentListID: recruitmentList.ID.Hex(),
			Response:          output,
		}
	}

	return
}

func resetResponseParserCache() {
	responseExporterCache = make(map[string]*surveyresponses.ResponseParser)
	responseExporterCacheForPInfos = make(map[string]*surveyresponses.ResponseParser)
}

func initResponseParser(
	studyDB *sDB.StudyDBService,
	instanceID string,
	studyKey string,
	surveyKey string,
	excludedCols []string,
) (respParser *surveyresponses.ResponseParser, err error) {

	surveyVersions, err := surveydefinition.PrepareSurveyInfosFromDB(
		studyDB,
		instanceID,
		studyKey,
		surveyKey,
		&surveydefinition.ExtractOptions{
			UseLabelLang: "",
			IncludeItems: nil,
			ExcludeItems: excludedCols,
		},
	)
	if err != nil {
		slog.Error("failed to get survey versions", slog.String("error", err.Error()))
		return
	}

	useShortKeys := false
	extraCols := []string{}
	respParser, err = surveyresponses.NewResponseParser(
		surveyKey,
		surveyVersions,
		useShortKeys,
		nil,
		"-",
		&extraCols,
	)
	if err != nil {
		slog.Error("failed to create response parser", slog.String("error", err.Error()))
		return
	}
	return
}

func applyMapping(pInfoDef rDB.ParticipantInfo, value string) string {
	switch pInfoDef.MappingType {
	case rDB.MappingTypeTs2Date:
		value = strings.Split(value, ".")[0]
		ts, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			slog.Error("failed to parse date", slog.String("error", err.Error()))
			return value
		}
		return time.Unix(ts, 0).Format("2006-Jan-02")
	case rDB.MappingTypeKey2Value:
		for _, m := range pInfoDef.Mapping {
			if m.Key == value {
				return m.Value
			}
		}
	}
	// if no mapping found, return original value
	return value
}
