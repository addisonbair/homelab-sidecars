package raid

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		name           string
		mdstatContent  string
		expectedArrays []string
		wantHealthy    bool
		wantContains   string
	}{
		{
			name: "healthy RAID1",
			mdstatContent: `Personalities : [raid1]
md0 : active raid1 sda[0] sdb[1]
      3906886464 blocks super 1.2 [2/2] [UU]
      bitmap: 2/30 pages [8KB], 65536KB chunk

unused devices: <none>
`,
			expectedArrays: []string{"md0"},
			wantHealthy:    true,
			wantContains:   "healthy",
		},
		{
			name: "degraded RAID1 - one disk missing",
			mdstatContent: `Personalities : [raid1]
md0 : active raid1 sda[0]
      3906886464 blocks super 1.2 [2/1] [U_]
      bitmap: 2/30 pages [8KB], 65536KB chunk

unused devices: <none>
`,
			expectedArrays: []string{"md0"},
			wantHealthy:    false,
			wantContains:   "degraded",
		},
		{
			name: "degraded RAID1 - other disk missing",
			mdstatContent: `Personalities : [raid1]
md0 : active raid1 sdb[1]
      3906886464 blocks super 1.2 [2/1] [_U]
      bitmap: 2/30 pages [8KB], 65536KB chunk

unused devices: <none>
`,
			expectedArrays: []string{"md0"},
			wantHealthy:    false,
			wantContains:   "degraded",
		},
		{
			name: "rebuilding RAID1",
			mdstatContent: `Personalities : [raid1]
md0 : active raid1 sda[0] sdb[1]
      3906886464 blocks super 1.2 [2/1] [U_]
      [>....................]  recovery =  5.0% (195344256/3906886464) finish=305.2min speed=202544K/sec
      bitmap: 2/30 pages [8KB], 65536KB chunk

unused devices: <none>
`,
			expectedArrays: []string{"md0"},
			wantHealthy:    false,
			wantContains:   "rebuilding",
		},
		{
			name: "healthy RAID5",
			mdstatContent: `Personalities : [raid1] [raid5]
md1 : active raid5 sdc[0] sdd[1] sde[2]
      7813771264 blocks super 1.2 level 5, 512k chunk, algorithm 2 [3/3] [UUU]

unused devices: <none>
`,
			expectedArrays: []string{"md1"},
			wantHealthy:    true,
			wantContains:   "healthy",
		},
		{
			name: "degraded RAID5",
			mdstatContent: `Personalities : [raid1] [raid5]
md1 : active raid5 sdc[0] sdd[1]
      7813771264 blocks super 1.2 level 5, 512k chunk, algorithm 2 [3/2] [UU_]

unused devices: <none>
`,
			expectedArrays: []string{"md1"},
			wantHealthy:    false,
			wantContains:   "degraded",
		},
		{
			name: "multiple arrays - all healthy",
			mdstatContent: `Personalities : [raid1]
md0 : active raid1 sda[0] sdb[1]
      1048576 blocks super 1.2 [2/2] [UU]

md1 : active raid1 sdc[0] sdd[1]
      2097152 blocks super 1.2 [2/2] [UU]

unused devices: <none>
`,
			expectedArrays: []string{"md0", "md1"},
			wantHealthy:    true,
			wantContains:   "healthy",
		},
		{
			name: "multiple arrays - one degraded",
			mdstatContent: `Personalities : [raid1]
md0 : active raid1 sda[0] sdb[1]
      1048576 blocks super 1.2 [2/2] [UU]

md1 : active raid1 sdc[0]
      2097152 blocks super 1.2 [2/1] [U_]

unused devices: <none>
`,
			expectedArrays: []string{"md0", "md1"},
			wantHealthy:    false,
			wantContains:   "degraded",
		},
		{
			name: "expected array not found",
			mdstatContent: `Personalities : [raid1]
md0 : active raid1 sda[0] sdb[1]
      1048576 blocks super 1.2 [2/2] [UU]

unused devices: <none>
`,
			expectedArrays: []string{"md0", "md1"},
			wantHealthy:    false,
			wantContains:   "not found",
		},
		{
			name: "no arrays",
			mdstatContent: `Personalities : [raid1]
unused devices: <none>
`,
			expectedArrays: []string{"md0"},
			wantHealthy:    false,
			wantContains:   "no RAID arrays found",
		},
		{
			name: "recovery in progress with percentage",
			mdstatContent: `Personalities : [raid1]
md0 : active raid1 sda[0] sdb[1]
      3906886464 blocks super 1.2 [2/1] [U_]
      [===>.................]  recovery = 17.5% (683954048/3906886464) finish=215.0min speed=250000K/sec

unused devices: <none>
`,
			expectedArrays: []string{"md0"},
			wantHealthy:    false,
			wantContains:   "17.5%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with mdstat content
			tmpDir := t.TempDir()
			mdstatPath := filepath.Join(tmpDir, "mdstat")
			if err := os.WriteFile(mdstatPath, []byte(tt.mdstatContent), 0644); err != nil {
				t.Fatalf("failed to write temp mdstat: %v", err)
			}

			healthy, reason, err := Check(mdstatPath, tt.expectedArrays)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if healthy != tt.wantHealthy {
				t.Errorf("healthy = %v, want %v", healthy, tt.wantHealthy)
			}

			if tt.wantContains != "" && !contains(reason, tt.wantContains) {
				t.Errorf("reason = %q, want to contain %q", reason, tt.wantContains)
			}
		})
	}
}

func TestCheck_FileNotFound(t *testing.T) {
	_, _, err := Check("/nonexistent/path/mdstat", []string{"md0"})
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
