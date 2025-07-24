# Recruitment List Backend

A Go-based backend service for managing recruitment lists and participant data synchronization.

## Configuration

The service uses a YAML configuration file and supports environment variable overrides for sensitive information.

### Configuration File

The service reads its configuration from a YAML file specified by the `CONFIG_FILE_PATH` environment variable.

#### Configuration Structure

```yaml
# Logging configuration
logging:
  log_level: "info"  # debug, info, warn, error
  include_src: false
  log_to_file: true
  filename: "logs/recruitment-list-api.log"
  max_size: 100  # MB
  max_age: 30    # days
  max_backups: 5
  compress_old_logs: true
  include_build_info: true

# Gin web server configuration
gin_config:
  debug_mode: true
  allow_origins:
    - "http://localhost:3002"
  port: 8045
  mtls:
    use: false
    certificate_paths: null

# User management configuration
user_management_config:
  researcher_user_jwt_config:
    sign_key: "your-jwt-sign-key"
    expires_in: "24h"  # Duration string (e.g., "1h", "24h", "7d")

# Study services connection
study_service_connection:
  instance_id: "your-instance-id"
  global_secret: "your-global-secret"
  external_services:
    - name: "service1"
      url: "https://service1.example.com"
      api_key: "service1-api-key"
    - name: "service2"
      url: "https://service2.example.com"
      api_key: "service2-api-key"

# Database configurations
db_configs:
  recruitment_list_db:
    connection_str: "<connection_str>"
    username: "<env var RECRUITMENT_LIST_DB_USERNAME>"
    password: "<env var RECRUITMENT_LIST_DB_PASSWORD>"
    connection_prefix: ""
    timeout: 30
    idle_conn_timeout: 45
    max_pool_size: 8
    use_no_cursor_timeout: false
    db_name_prefix: ""
    run_index_creation: false

  study_db:
    connection_str: "<connection_str>"
    username: "<env var STUDY_DB_USERNAME>"
    password: "<env var STUDY_DB_PASSWORD>"
    connection_prefix: ""
    timeout: 30
    idle_conn_timeout: 45
    max_pool_size: 8
    use_no_cursor_timeout: false
    db_name_prefix: ""
    run_index_creation: false

# File storage path for generated files
filestore_path: "/path/to/filestore"

```

### Environment Variable Overrides

The following environment variables can be used to override sensitive configuration values:

#### Required Environment Variables

- `CONFIG_FILE_PATH`: Path to the configuration YAML file

#### Database Credentials

- `RECRUITMENT_LIST_DB_USERNAME`: Overrides recruitment list database username
- `RECRUITMENT_LIST_DB_PASSWORD`: Overrides recruitment list database password
- `STUDY_DB_USERNAME`: Overrides study database username
- `STUDY_DB_PASSWORD`: Overrides study database password

#### Security

- `JWT_SIGN_KEY`: Overrides JWT signing key for user authentication
- `STUDY_GLOBAL_SECRET`: Overrides global secret for study services

#### External Service API Keys

For each external service defined in the configuration, the API key can be overridden using an environment variable with the pattern:
`EXTERNAL_SERVICE_{SERVICE_NAME}_API_KEY`

For example:
- `EXTERNAL_SERVICE_SERVICE1_API_KEY`: Overrides API key for service named "service1"
- `EXTERNAL_SERVICE_SERVICE2_API_KEY`: Overrides API key for service named "service2"

