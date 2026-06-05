package versions

import (
	"testing"
)

func TestCalculateAutoPrerelease(t *testing.T) {
	tests := []struct {
		name              string
		major             string
		currentStable     string
		currentPrerelease string
		expected          string
	}{
		{
			name:              "no current versions - first RC",
			major:             "v1",
			currentStable:     "",
			currentPrerelease: "",
			expected:          "v1.0.0-rc.1",
		},
		{
			name:              "stable only - bump patch RC",
			major:             "v1",
			currentStable:     "v1.0.0",
			currentPrerelease: "",
			expected:          "v1.0.1-rc.1",
		},
		{
			name:              "prerelease for stable - bump RC",
			major:             "v1",
			currentStable:     "v1.0.0",
			currentPrerelease: "v1.0.0-rc.1",
			expected:          "v1.0.0-rc.2",
		},
		{
			name:              "prerelease for next version - bump RC",
			major:             "v1",
			currentStable:     "v1.0.0",
			currentPrerelease: "v1.0.1-rc.1",
			expected:          "v1.0.1-rc.2",
		},
		{
			name:              "prerelease RC increment",
			major:             "v1",
			currentStable:     "v1.1.0",
			currentPrerelease: "v1.1.1-rc.3",
			expected:          "v1.1.1-rc.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calculateAutoPrerelease(tt.major, tt.currentStable, tt.currentPrerelease)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("got %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestCalculateManualRelease(t *testing.T) {
	tests := []struct {
		name              string
		major             string
		currentStable     string
		currentPrerelease string
		bumpType          string
		releaseType       string
		expected          string
	}{
		{
			name:              "minor bump prerelease - first RC",
			major:             "v1",
			currentStable:     "v1.0.0",
			currentPrerelease: "",
			bumpType:          "minor",
			releaseType:       "prerelease",
			expected:          "v1.1.0-rc.1",
		},
		{
			name:              "minor bump stable",
			major:             "v1",
			currentStable:     "v1.0.0",
			currentPrerelease: "",
			bumpType:          "minor",
			releaseType:       "stable",
			expected:          "v1.1.0",
		},
		{
			name:              "patch bump prerelease - first RC",
			major:             "v1",
			currentStable:     "v1.0.0",
			currentPrerelease: "",
			bumpType:          "patch",
			releaseType:       "prerelease",
			expected:          "v1.0.1-rc.1",
		},
		{
			name:              "minor bump prerelease - same target continues RC",
			major:             "v1",
			currentStable:     "v1.0.0",
			currentPrerelease: "v1.1.0-rc.1",
			bumpType:          "minor",
			releaseType:       "prerelease",
			expected:          "v1.1.0-rc.2",
		},
		{
			name:              "minor bump prerelease - different target starts new RC",
			major:             "v1",
			currentStable:     "v1.0.0",
			currentPrerelease: "v1.0.1-rc.3",
			bumpType:          "minor",
			releaseType:       "prerelease",
			expected:          "v1.1.0-rc.1",
		},
		{
			name:              "patch bump prerelease - same target continues RC",
			major:             "v1",
			currentStable:     "v1.1.0",
			currentPrerelease: "v1.1.1-rc.2",
			bumpType:          "patch",
			releaseType:       "prerelease",
			expected:          "v1.1.1-rc.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calculateManualRelease(tt.major, tt.currentStable, tt.currentPrerelease, tt.bumpType, tt.releaseType)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("got %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		version     string
		wantMajor   int
		wantMinor   int
		wantPatch   int
		expectError bool
	}{
		{"v1.2.3", 1, 2, 3, false},
		{"v0.0.0", 0, 0, 0, false},
		{"v10.20.30", 10, 20, 30, false},
		{"1.2.3", 0, 0, 0, true},   // missing v prefix
		{"v1.2", 0, 0, 0, true},    // incomplete
		{"invalid", 0, 0, 0, true}, // invalid format
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			major, minor, patch, err := parseVersion(tt.version)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for version %s, got nil", tt.version)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if major != tt.wantMajor || minor != tt.wantMinor || patch != tt.wantPatch {
				t.Errorf("got (%d, %d, %d), want (%d, %d, %d)",
					major, minor, patch, tt.wantMajor, tt.wantMinor, tt.wantPatch)
			}
		})
	}
}
