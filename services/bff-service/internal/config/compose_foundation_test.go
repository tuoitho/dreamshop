package config

import (
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFoundationComposePersistsStatefulDataWithNamedVolumes(t *testing.T) {
	root := repoRoot(t)
	raw := readRepoFile(t, root, "deployments", "docker", "compose.foundation.yml")

	var compose composeManifest
	if err := yaml.Unmarshal([]byte(raw), &compose); err != nil {
		t.Fatalf("parse compose.foundation.yml: %v", err)
	}

	for _, vol := range []string{"postgres_data", "redis_data", "rabbitmq_data"} {
		if _, ok := compose.Volumes[vol]; !ok {
			t.Errorf("compose.foundation.yml missing named volume %q", vol)
		}
	}

	postgres := compose.Services["postgres"]
	if !slices.Contains(postgres.Volumes, "postgres_data:/var/lib/postgresql/data") {
		t.Errorf("postgres volumes = %v, missing postgres_data:/var/lib/postgresql/data", postgres.Volumes)
	}
	if !slices.Contains(postgres.Volumes, "../postgres/init/01-create-databases.sql:/docker-entrypoint-initdb.d/01-create-databases.sql:ro") {
		t.Errorf("postgres volumes = %v, missing init SQL mount", postgres.Volumes)
	}

	redis := compose.Services["redis"]
	if !slices.Contains(redis.Volumes, "redis_data:/data") {
		t.Errorf("redis volumes = %v, missing redis_data:/data", redis.Volumes)
	}

	rabbitmq := compose.Services["rabbitmq"]
	if !slices.Contains(rabbitmq.Volumes, "rabbitmq_data:/var/lib/rabbitmq") {
		t.Errorf("rabbitmq volumes = %v, missing rabbitmq_data:/var/lib/rabbitmq", rabbitmq.Volumes)
	}
}
