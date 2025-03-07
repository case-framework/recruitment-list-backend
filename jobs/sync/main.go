package main

import (
	"log/slog"

	"github.com/case-framework/recruitment-list-backend/pkg/sync"
)

func main() {
	slog.Info("Sync job started")

	rls, err := recruitmentListDBService.GetRecruitmentListsInfos()
	if err != nil {
		slog.Error("could not retrieve recruitment lists", slog.String("error", err.Error()))
		return
	}

	for _, rl := range rls {
		slog.Info("start sync for recruitment list", slog.String("id", rl.ID.Hex()), slog.String("name", rl.Name))

		// sync participants
		if err := sync.SyncParticipantsForRL(
			recruitmentListDBService,
			studyDBService,
			rl.ID.Hex(),
			conf.StudyServicesConnection.InstanceID,
		); err != nil {
			slog.Error("could not sync participants", slog.String("id", rl.ID.Hex()), slog.String("name", rl.Name), slog.String("error", err.Error()))
		} else {
			slog.Info("participant sync finished", slog.String("id", rl.ID.Hex()), slog.String("name", rl.Name))
		}

		// sync responses
		if err := sync.SyncResearchDataForRL(
			recruitmentListDBService,
			studyDBService,
			rl.ID.Hex(),
			conf.StudyServicesConnection.InstanceID,
			conf.StudyServicesConnection.GlobalSecret,
		); err != nil {
			slog.Error("could not sync research data", slog.String("id", rl.ID.Hex()), slog.String("name", rl.Name), slog.String("error", err.Error()))
		} else {
			slog.Info("response sync finished", slog.String("id", rl.ID.Hex()), slog.String("name", rl.Name))
		}
	}

	slog.Info("Sync job finished")
}
