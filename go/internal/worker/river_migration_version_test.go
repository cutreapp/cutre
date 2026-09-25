package worker

import (
	"testing"

	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// TestAppliedRiverMigrationVersion は、Riverのライブラリが知る最新のマイグレーションのversionと、
// dbmateで適用したversionが一致することを検証する。
func TestAppliedRiverMigrationVersion(t *testing.T) {
	t.Parallel()

	// AllVersions はライブラリに埋め込まれたマイグレーションを読むだけで、データベースに接続しない。
	// そのためプールはnilで足りる。
	migrator, err := rivermigrate.New(riverpgxv5.New(nil), nil)
	if err != nil {
		t.Fatalf("rivermigrate.New()のエラー = %v", err)
	}

	versions := migrator.AllVersions()
	if len(versions) == 0 {
		t.Fatal("AllVersions()の件数 = 0、1件以上を期待")
	}

	// AllVersions はversionの昇順に並んでいる。
	latest := versions[len(versions)-1].Version
	if latest != appliedRiverMigrationVersion {
		t.Errorf("Riverのライブラリの最新のversion = %d、期待値 = %d (appliedRiverMigrationVersion)。"+
			"create_river_tables マイグレーションの冒頭の手順で version %d のマイグレーションを足し、定数を上げてください",
			latest, appliedRiverMigrationVersion, latest)
	}
}
