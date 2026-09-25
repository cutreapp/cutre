package worker

// appliedRiverMigrationVersion はdbmateのマイグレーションで適用し、river_migrationに記録したRiverのスキーマのversion。
//
// 同じパッケージのテストが、リンクしているRiverのライブラリが知る最新のversionとこの値を比べる。
// Riverを上げて新しいversionが増えるとテストが失敗し、dbmateのマイグレーションを足すきっかけになる。
// スキーマがライブラリの期待より遅れたまま本番へ出ることを、CIで止めるため。
//
// 上げるときの手順は create_river_tables マイグレーションの冒頭のコメントにある。
const appliedRiverMigrationVersion = 7
