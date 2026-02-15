package system

import "testing"

func TestParseEnvText(t *testing.T) {
	got := parseEnvText(`
# comment
 BACKUP_CRON_EXPRESSION=0 3 * * *
INVALID LINE
_OK_=1
AWS_S3_BUCKET_NAME=a
AWS_S3_BUCKET_NAME=b
BAD-KEY=1
`)
	want := []string{
		"BACKUP_CRON_EXPRESSION=0 3 * * *",
		"_OK_=1",
		"AWS_S3_BUCKET_NAME=b",
	}
	if len(got) != len(want) {
		t.Fatalf("len=%d want=%d got=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("idx=%d got=%q want=%q all=%v", i, got[i], want[i], got)
		}
	}
}

func TestSanitizeVolumeTarget(t *testing.T) {
	if sanitizeVolumeTarget("a/b:c") != "a_b_c" {
		t.Fatalf("unexpected: %q", sanitizeVolumeTarget("a/b:c"))
	}
	if sanitizeVolumeTarget("   ") != "volume" {
		t.Fatalf("unexpected: %q", sanitizeVolumeTarget("   "))
	}
}

func TestNormalizeStringList(t *testing.T) {
	got := normalizeStringList([]string{" a ", "b", "a", "", "b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected: %v", got)
	}
}
