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

	for _, vol := range []string{"postgres_data", "redis_data", "rabbitmq_data", "minio_data"} {
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

	minio, ok := compose.Services["minio"]
	if !ok {
		t.Fatal("compose.foundation.yml missing minio service")
	}
	if !slices.Contains(minio.Volumes, "minio_data:/data") {
		t.Errorf("minio volumes = %v, missing minio_data:/data", minio.Volumes)
	}
	if !slices.Contains(minio.Ports, "9000:9000") || !slices.Contains(minio.Ports, "127.0.0.1:9001:9001") {
		t.Errorf("minio ports = %v, want 9000:9000 and 127.0.0.1:9001:9001", minio.Ports)
	}
	if minio.Environment["MINIO_API_CORS_ALLOW_ORIGIN"] != "http://localhost:5173,http://localhost:5174" {
		t.Errorf("minio MINIO_API_CORS_ALLOW_ORIGIN = %q, want http://localhost:5173,http://localhost:5174", minio.Environment["MINIO_API_CORS_ALLOW_ORIGIN"])
	}
	if len(minio.Healthcheck) == 0 {
		t.Error("minio missing healthcheck")
	}
}
