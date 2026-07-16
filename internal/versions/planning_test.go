package versions

import (
	"regexp"
	"testing"
)

func TestGenerateCalVerTimestamp(t *testing.T) {
	tests := []struct {
		name         string
		rancherMinor string
		releaseType  string
		wantPattern  string
	}{
		{
			name:         "stable release for 2.14",
			rancherMinor: "2.14",
			releaseType:  "stable",
			wantPattern:  `^v2\.14-\d{8}T\d{4}Z$`,
		},
		{
			name:         "prerelease for 2.15",
			rancherMinor: "2.15",
			releaseType:  "prerelease",
			wantPattern:  `^v2\.15-\d{8}T\d{4}Z-dev$`,
		},
		{
			name:         "stable release for 2.16",
			rancherMinor: "2.16",
			releaseType:  "stable",
			wantPattern:  `^v2\.16-\d{8}T\d{4}Z$`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateCalVerTimestamp(tt.rancherMinor, tt.releaseType)
			matched, err := regexp.MatchString(tt.wantPattern, got)
			if err != nil {
				t.Fatalf("invalid regex pattern: %v", err)
			}
			if !matched {
				t.Errorf("got %s, does not match pattern %s", got, tt.wantPattern)
			}
		})
	}
}

func TestParseCalVerTag(t *testing.T) {
	tests := []struct {
		name            string
		tag             string
		wantMinor       string
		wantReleaseType string
		wantErr         bool
	}{
		{
			name:            "stable release",
			tag:             "v2.14-20260716T1430Z",
			wantMinor:       "2.14",
			wantReleaseType: "stable",
			wantErr:         false,
		},
		{
			name:            "prerelease",
			tag:             "v2.15-20260805T1600Z-dev",
			wantMinor:       "2.15",
			wantReleaseType: "prerelease",
			wantErr:         false,
		},
		{
			name:            "invalid format - missing timestamp",
			tag:             "v2.14",
			wantMinor:       "",
			wantReleaseType: "",
			wantErr:         true,
		},
		{
			name:            "invalid format - old SemVer",
			tag:             "v1.0.0",
			wantMinor:       "",
			wantReleaseType: "",
			wantErr:         true,
		},
		{
			name:            "invalid format - wrong timestamp format",
			tag:             "v2.14-20260716-1430Z",
			wantMinor:       "",
			wantReleaseType: "",
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMinor, gotType, err := ParseCalVerTag(tt.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCalVerTag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotMinor != tt.wantMinor {
				t.Errorf("ParseCalVerTag() gotMinor = %v, want %v", gotMinor, tt.wantMinor)
			}
			if gotType != tt.wantReleaseType {
				t.Errorf("ParseCalVerTag() gotType = %v, want %v", gotType, tt.wantReleaseType)
			}
		})
	}
}

func TestPlanAutoPrerelease(t *testing.T) {
	tests := []struct {
		name           string
		changedMinors  []string
		wantCount      int
		wantMinors     []string
		wantReleaseTyp string
	}{
		{
			name:           "single changed minor",
			changedMinors:  []string{"2.15"},
			wantCount:      1,
			wantMinors:     []string{"2.15"},
			wantReleaseTyp: "prerelease",
		},
		{
			name:           "multiple changed minors",
			changedMinors:  []string{"2.14", "2.15"},
			wantCount:      2,
			wantMinors:     []string{"2.14", "2.15"},
			wantReleaseTyp: "prerelease",
		},
		{
			name:           "no changed minors",
			changedMinors:  []string{},
			wantCount:      0,
			wantMinors:     []string{},
			wantReleaseTyp: "prerelease",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plans, err := PlanAutoPrerelease(tt.changedMinors)
			if err != nil {
				t.Fatalf("PlanAutoPrerelease() error = %v", err)
			}
			if len(plans) != tt.wantCount {
				t.Errorf("PlanAutoPrerelease() plan count = %v, want %v", len(plans), tt.wantCount)
			}
			for i, plan := range plans {
				if plan.RancherMinor != tt.wantMinors[i] {
					t.Errorf("Plan[%d] RancherMinor = %v, want %v", i, plan.RancherMinor, tt.wantMinors[i])
				}
				if plan.ReleaseType != tt.wantReleaseTyp {
					t.Errorf("Plan[%d] ReleaseType = %v, want %v", i, plan.ReleaseType, tt.wantReleaseTyp)
				}
				// Verify version format matches CalVer
				matched, _ := regexp.MatchString(`^v\d+\.\d+-\d{8}T\d{4}Z-dev$`, plan.Version)
				if !matched {
					t.Errorf("Plan[%d] Version = %v, does not match CalVer prerelease pattern", i, plan.Version)
				}
			}
		})
	}
}

func TestPlanManualRelease(t *testing.T) {
	tests := []struct {
		name          string
		rancherMinors []string
		releaseType   string
		wantCount     int
		wantPattern   string
		wantErr       bool
	}{
		{
			name:          "stable release",
			rancherMinors: []string{"2.14"},
			releaseType:   "stable",
			wantCount:     1,
			wantPattern:   `^v2\.14-\d{8}T\d{4}Z$`,
			wantErr:       false,
		},
		{
			name:          "prerelease",
			rancherMinors: []string{"2.15"},
			releaseType:   "prerelease",
			wantCount:     1,
			wantPattern:   `^v2\.15-\d{8}T\d{4}Z-dev$`,
			wantErr:       false,
		},
		{
			name:          "multiple minors stable",
			rancherMinors: []string{"2.14", "2.15"},
			releaseType:   "stable",
			wantCount:     2,
			wantPattern:   `^v\d+\.\d+-\d{8}T\d{4}Z$`,
			wantErr:       false,
		},
		{
			name:          "invalid release type",
			rancherMinors: []string{"2.14"},
			releaseType:   "invalid",
			wantCount:     0,
			wantPattern:   "",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plans, err := PlanManualRelease(tt.rancherMinors, tt.releaseType)
			if (err != nil) != tt.wantErr {
				t.Errorf("PlanManualRelease() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if len(plans) != tt.wantCount {
				t.Errorf("PlanManualRelease() plan count = %v, want %v", len(plans), tt.wantCount)
			}
			for _, plan := range plans {
				matched, _ := regexp.MatchString(tt.wantPattern, plan.Version)
				if !matched {
					t.Errorf("Plan Version = %v, does not match pattern %s", plan.Version, tt.wantPattern)
				}
				if plan.ReleaseType != tt.releaseType {
					t.Errorf("Plan ReleaseType = %v, want %v", plan.ReleaseType, tt.releaseType)
				}
			}
		})
	}
}
