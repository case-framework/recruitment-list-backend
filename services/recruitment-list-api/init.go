package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/case-framework/case-backend/pkg/apihelpers"
	"github.com/case-framework/case-backend/pkg/db"
	"github.com/case-framework/case-backend/pkg/study"
	"github.com/case-framework/case-backend/pkg/study/studyengine"
	"github.com/case-framework/case-backend/pkg/utils"
	"github.com/gin-gonic/gin"

	"gopkg.in/yaml.v2"

	sdb "github.com/case-framework/case-backend/pkg/db/study"
	rdb "github.com/case-framework/recruitment-list-backend/pkg/db/recruitment-list"
)

const (
	ENV_CONFIG_FILE_PATH = "CONFIG_FILE_PATH"

	ENV_JWT_SIGN_KEY = "JWT_SIGN_KEY"

	ENV_RECRUITMENT_LIST_DB_USERNAME = "RECRUITMENT_LIST_DB_USERNAME"
	ENV_RECRUITMENT_LIST_DB_PASSWORD = "RECRUITMENT_LIST_DB_PASSWORD"

	ENV_STUDY_DB_USERNAME = "STUDY_DB_USERNAME"
	ENV_STUDY_DB_PASSWORD = "STUDY_DB_PASSWORD"

	ENV_STUDY_GLOBAL_SECRET = "STUDY_GLOBAL_SECRET"
)

type RecruitmentListApiConfig struct {
	// Logging configs
	Logging utils.LoggerConfig `json:"logging" yaml:"logging"`

	// Gin configs
	GinConfig struct {
		DebugMode    bool     `json:"debug_mode" yaml:"debug_mode"`
		AllowOrigins []string `json:"allow_origins" yaml:"allow_origins"`
		Port         string   `json:"port" yaml:"port"`

		// Mutual TLS configs
		MTLS struct {
			Use              bool                        `json:"use" yaml:"use"`
			CertificatePaths apihelpers.CertificatePaths `json:"certificate_paths" yaml:"certificate_paths"`
		} `json:"mtls" yaml:"mtls"`
	} `json:"gin_config" yaml:"gin_config"`

	// user management configs
	UserManagementConfig struct {
		ResearcherUserJWTConfig struct {
			SignKey   string        `json:"sign_key" yaml:"sign_key"`
			ExpiresIn time.Duration `json:"expires_in" yaml:"expires_in"`
		} `json:"researcher_user_jwt_config" yaml:"researcher_user_jwt_config"`
	} `json:"user_management_config" yaml:"user_management_config"`

	StudyServicesConnection struct {
		InstanceID       string                        `json:"instance_id" yaml:"instance_id"`
		GlobalSecret     string                        `json:"global_secret" yaml:"global_secret"`
		ExternalServices []studyengine.ExternalService `json:"external_services" yaml:"external_services"`
	} `json:"study_service_connection" yaml:"study_service_connection"`

	// DB configs
	DBConfigs struct {
		RecruitmentListDB db.DBConfigYaml `json:"recruitment_list_db" yaml:"recruitment_list_db"`
		StudyDB           db.DBConfigYaml `json:"study_db" yaml:"study_db"`
	} `json:"db_configs" yaml:"db_configs"`

	FilestorePath string `json:"filestore_path" yaml:"filestore_path"`
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
		conf.Logging.IncludeBuildInfo,
	)

	// Override secrets from environment variables
	secretsOverride()

	// Init DBs
	initDBs()

	initStudyService()

	if !conf.GinConfig.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}

	checkRecruitmentListFilestorePath()
}

func initStudyService() {
	study.Init(
		studyDBService,
		conf.StudyServicesConnection.GlobalSecret,
		conf.StudyServicesConnection.ExternalServices,
	)
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

	if signKey := os.Getenv(ENV_JWT_SIGN_KEY); signKey != "" {
		conf.UserManagementConfig.ResearcherUserJWTConfig.SignKey = signKey
	}

	if studyGlobalSecret := os.Getenv(ENV_STUDY_GLOBAL_SECRET); studyGlobalSecret != "" {
		conf.StudyServicesConnection.GlobalSecret = studyGlobalSecret
	}

	// Override API keys for external services
	for i := range conf.StudyServicesConnection.ExternalServices {
		service := &conf.StudyServicesConnection.ExternalServices[i]

		// Skip if name is not defined
		if service.Name == "" {
			continue
		}

		// Generate environment variable name from service name
		envVarName := utils.GenerateExternalServiceAPIKeyEnvVarName(service.Name)

		// Override if environment variable exists
		if apiKey := os.Getenv(envVarName); apiKey != "" {
			service.APIKey = apiKey
		}
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

func checkRecruitmentListFilestorePath() {
	// To store dynamically generated files
	fsPath := conf.FilestorePath
	if fsPath == "" {
		slog.Error("Filestore path not set")
		panic("Filestore path not set")
	}

	if _, err := os.Stat(fsPath); os.IsNotExist(err) {
		slog.Error("Filestore path does not exist", slog.String("path", fsPath))
		panic("Filestore path does not exist")
	}
}
