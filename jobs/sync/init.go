package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/case-framework/case-backend/pkg/db"
	httpclient "github.com/case-framework/case-backend/pkg/http-client"
	"github.com/case-framework/case-backend/pkg/utils"
	"gopkg.in/yaml.v2"

	sdb "github.com/case-framework/case-backend/pkg/db/study"
	rdb "github.com/case-framework/recruitment-list-backend/pkg/db/recruitment-list"
	"github.com/case-framework/recruitment-list-backend/pkg/sync"
)

const (
	ENV_CONFIG_FILE_PATH = "CONFIG_FILE_PATH"

	ENV_RECRUITMENT_LIST_DB_USERNAME = "RECRUITMENT_LIST_DB_USERNAME"
	ENV_RECRUITMENT_LIST_DB_PASSWORD = "RECRUITMENT_LIST_DB_PASSWORD"

	ENV_STUDY_DB_USERNAME = "STUDY_DB_USERNAME"
	ENV_STUDY_DB_PASSWORD = "STUDY_DB_PASSWORD"

	ENV_STUDY_GLOBAL_SECRET = "STUDY_GLOBAL_SECRET"
)

type RecruitmentListApiConfig struct {
	// Logging configs
	Logging utils.LoggerConfig `json:"logging" yaml:"logging"`

	StudyServicesConnection struct {
		InstanceID   string `json:"instance_id" yaml:"instance_id"`
		GlobalSecret string `json:"global_secret" yaml:"global_secret"`
	} `json:"study_service_connection" yaml:"study_service_connection"`

	// DB configs
	DBConfigs struct {
		RecruitmentListDB db.DBConfigYaml `json:"recruitment_list_db" yaml:"recruitment_list_db"`
		StudyDB           db.DBConfigYaml `json:"study_db" yaml:"study_db"`
	} `json:"db_configs" yaml:"db_configs"`

	SmtpBridgeConfig *struct {
		URL            string        `json:"url" yaml:"url"`
		APIKey         string        `json:"api_key" yaml:"api_key"`
		RequestTimeout time.Duration `json:"request_timeout" yaml:"request_timeout"`
	} `json:"smtp_bridge_config" yaml:"smtp_bridge_config"`
}

var (
	conf                     RecruitmentListApiConfig
	recruitmentListDBService *rdb.RecruitmentListDBService
	studyDBService           *sdb.StudyDBService
)

func init() {
	// Read config from file
	yamlFile, err := os.ReadFile(os.Getenv(ENV_CONFIG_FILE_PATH))
	if err != nil {
		panic(err)
	}

	err = yaml.UnmarshalStrict(yamlFile, &conf)
	if err != nil {
		panic(err)
	}

	// Init logger:
	utils.InitLogger(
		conf.Logging.LogLevel,
		conf.Logging.IncludeSrc,
		conf.Logging.LogToFile,
		conf.Logging.Filename,
		conf.Logging.MaxSize,
		conf.Logging.MaxAge,
		conf.Logging.MaxBackups,
		conf.Logging.CompressOldLogs,
	)

	// Override secrets from environment variables
	secretsOverride()

	// Init DBs
	initDBs()

	if conf.SmtpBridgeConfig != nil && conf.SmtpBridgeConfig.URL != "" {
		slog.Info("SMTP bridge configured, will send emails to researchers")
		sync.HttpClient = loadEmailClientHTTPConfig()
	} else {
		slog.Debug("SMTP bridge not configured, will not send emails to researchers")
	}
}

func secretsOverride() {
	if dbUsername := os.Getenv(ENV_RECRUITMENT_LIST_DB_USERNAME); dbUsername != "" {
		conf.DBConfigs.RecruitmentListDB.Username = dbUsername
	}

	if dbPassword := os.Getenv(ENV_RECRUITMENT_LIST_DB_PASSWORD); dbPassword != "" {
		conf.DBConfigs.RecruitmentListDB.Password = dbPassword
	}

	if dbUsername := os.Getenv(ENV_STUDY_DB_USERNAME); dbUsername != "" {
		conf.DBConfigs.StudyDB.Username = dbUsername
	}

	if dbPassword := os.Getenv(ENV_STUDY_DB_PASSWORD); dbPassword != "" {
		conf.DBConfigs.StudyDB.Password = dbPassword
	}

	if studyGlobalSecret := os.Getenv(ENV_STUDY_GLOBAL_SECRET); studyGlobalSecret != "" {
		conf.StudyServicesConnection.GlobalSecret = studyGlobalSecret
	}
}

func initDBs() {
	var err error
	recruitmentListDBService, err = rdb.NewManagementUserDBService(db.DBConfigFromYamlObj(conf.DBConfigs.RecruitmentListDB, nil))
	if err != nil {
		slog.Error("Error connecting to recruitment list DB", slog.String("error", err.Error()))
	}

	studyDBService, err = sdb.NewStudyDBService(db.DBConfigFromYamlObj(conf.DBConfigs.StudyDB, nil))
	if err != nil {
		slog.Error("Error connecting to Study DB", slog.String("error", err.Error()))
	}
}

func loadEmailClientHTTPConfig() *httpclient.ClientConfig {
	return &httpclient.ClientConfig{
		RootURL: conf.SmtpBridgeConfig.URL,
		APIKey:  conf.SmtpBridgeConfig.APIKey,
		Timeout: conf.SmtpBridgeConfig.RequestTimeout,
	}
}
