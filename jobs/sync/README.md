# Recruitment List Sync Job

A Go-based background job for synchronizing participant data and research data between recruitment lists and study databases.

## Overview

The sync job is responsible for:
- Synchronizing participant data from study databases to recruitment lists
- Synchronizing research data (survey responses) from study databases to recruitment lists
- Sending email notifications to researchers when new participants are added
- Maintaining data consistency between different database instances

## Configuration

The sync job uses the same YAML configuration file as the main API service but with a subset of configuration options.

### Configuration File

The sync job reads its configuration from a YAML file specified by the `CONFIG_FILE_PATH` environment variable.

#### Configuration Structure

```yaml
# Logging configuration
logging:
  log_level: "info"  # debug, info, warn, error
  include_src: false
  log_to_file: true
  filename: "logs/sync-job.log"
  max_size: 100  # MB
  max_age: 30    # days
  max_backups: 5
  compress_old_logs: true
  include_build_info: true

# Study services connection
study_service_connection:
  instance_id: "your-instance-id"
  global_secret: "your-global-secret"  # REQUIRED

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

# SMTP Bridge configuration (optional)
smtp_bridge_config:
  url: "https://smtp-bridge.example.com"
  api_key: "smtp-api-key"
  request_timeout: "30s"
```

### Environment Variable Overrides

The following environment variables can be used to override sensitive configuration values:

#### Required Environment Variables

- `CONFIG_FILE_PATH`: Path to the configuration YAML file
- `STUDY_GLOBAL_SECRET`: **REQUIRED** - Global secret for study services (will panic if not set)

#### Database Credentials

- `RECRUITMENT_LIST_DB_USERNAME`: Overrides recruitment list database username
- `RECRUITMENT_LIST_DB_PASSWORD`: Overrides recruitment list database password
- `STUDY_DB_USERNAME`: Overrides study database username
- `STUDY_DB_PASSWORD`: Overrides study database password

#### SMTP Bridge Configuration

- `SMTP_BRIDGE_API_KEY`: Overrides SMTP bridge API key for email notifications
