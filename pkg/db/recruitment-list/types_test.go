package recruitmentlist

import (
	"testing"

	"gopkg.in/yaml.v2"
)

func TestDBConfigYamlKeepsRecruitmentListIndexCreationSetting(t *testing.T) {
	var config DBConfigYaml

	err := yaml.UnmarshalStrict([]byte(`
connection_str: localhost:27017
username: user
password: password
timeout: 30
run_index_creation: true
`), &config)
	if err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if config.ConnectionStr != "localhost:27017" {
		t.Fatalf("connection string = %q, want %q", config.ConnectionStr, "localhost:27017")
	}
	if !config.RunIndexCreation {
		t.Fatal("RunIndexCreation = false, want true")
	}
}

func TestDBConfigYamlRejectsUnknownSetting(t *testing.T) {
	var config DBConfigYaml

	err := yaml.UnmarshalStrict([]byte(`unknown_setting: true`), &config)
	if err == nil {
		t.Fatal("expected unknown setting to be rejected")
	}
}
