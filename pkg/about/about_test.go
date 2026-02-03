package about

import (
	"encoding/json"
	"testing"
)

func TestAboutStructFields(t *testing.T) {
	// Verify the About struct has expected values from package variables
	if About.Version != Version {
		t.Errorf("expected About.Version=%q, got %q", Version, About.Version)
	}
	if About.Epoch != Epoch {
		t.Errorf("expected About.Epoch=%q, got %q", Epoch, About.Epoch)
	}
	if About.Timestamp != Timestamp {
		t.Errorf("expected About.Timestamp=%q, got %q", Timestamp, About.Timestamp)
	}
	if About.Build != Build {
		t.Errorf("expected About.Build=%q, got %q", Build, About.Build)
	}
}

func TestAboutJSONMarshal(t *testing.T) {
	data, err := json.Marshal(About)
	if err != nil {
		t.Fatalf("failed to marshal About: %v", err)
	}

	// Verify we can unmarshal it back
	var unmarshaled Ab
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal About: %v", err)
	}

	if unmarshaled.Version != About.Version {
		t.Errorf("expected Version=%q, got %q", About.Version, unmarshaled.Version)
	}
	if unmarshaled.Epoch != About.Epoch {
		t.Errorf("expected Epoch=%q, got %q", About.Epoch, unmarshaled.Epoch)
	}
	if unmarshaled.Timestamp != About.Timestamp {
		t.Errorf("expected Timestamp=%q, got %q", About.Timestamp, unmarshaled.Timestamp)
	}
	if unmarshaled.Build != About.Build {
		t.Errorf("expected Build=%q, got %q", About.Build, unmarshaled.Build)
	}
}

func TestAboutJSONFieldNames(t *testing.T) {
	data, err := json.Marshal(About)
	if err != nil {
		t.Fatalf("failed to marshal About: %v", err)
	}

	// Parse as map to verify JSON field names
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	// Check that expected fields are present with correct names
	expectedFields := []string{"version", "epoch", "timestamp", "build"}
	for _, field := range expectedFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("expected field %q in JSON, but not found", field)
		}
	}

	// Verify no extra fields
	if len(raw) != len(expectedFields) {
		t.Errorf("expected %d fields, got %d", len(expectedFields), len(raw))
	}
}

func TestAbStruct(t *testing.T) {
	ab := Ab{
		Version:   "1.0.0",
		Epoch:     "123",
		Timestamp: "2024-01-01T00:00:00Z",
		Build:     "abc123",
	}

	if ab.Version != "1.0.0" {
		t.Errorf("expected Version=1.0.0, got %s", ab.Version)
	}
	if ab.Epoch != "123" {
		t.Errorf("expected Epoch=123, got %s", ab.Epoch)
	}
	if ab.Timestamp != "2024-01-01T00:00:00Z" {
		t.Errorf("expected Timestamp=2024-01-01T00:00:00Z, got %s", ab.Timestamp)
	}
	if ab.Build != "abc123" {
		t.Errorf("expected Build=abc123, got %s", ab.Build)
	}
}

func TestPackageVariables(t *testing.T) {
	// Verify package variables are set (even if to default "dev" values)
	if Version == "" {
		t.Error("Version should not be empty")
	}
	if Epoch == "" {
		t.Error("Epoch should not be empty")
	}
	if Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}
	if Build == "" {
		t.Error("Build should not be empty")
	}
}

func TestDefaultValues(t *testing.T) {
	// In dev builds, these should be default values
	// Note: In production builds, these are overridden via ldflags
	if Version != "dev" && Version == "" {
		t.Error("Version should be 'dev' or a valid version")
	}
}
