# CIでは環境変数がエクスポート済みのため op run を介さない。
# ローカルでは .env 内の op:// 参照を op run が解決してから起動する。
ifeq ($(CI),true)
  OP_RUN_DEV = APP_ENV=dev
  OP_RUN_TEST = APP_ENV=test
else
  OP_RUN_DEV = APP_ENV=dev op run --env-file=".env" --
  OP_RUN_TEST = APP_ENV=test op run --env-file=".env" --
endif

.PHONY: help
help: ## ヘルプを表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: run
run: ## サーバーを起動
	$(OP_RUN_DEV) go run ./cmd/cutre serve

.PHONY: browse-check
browse-check: ## devサイトのブラウザ確認に必要な環境を、資格情報の値を出さずに検証
	$(OP_RUN_DEV) bash scripts/browse.sh check

.PHONY: browse-shot
browse-shot: ## devサイトのページを開きスクショ (例: make browse-shot URL=/en)
	$(OP_RUN_DEV) bash scripts/browse.sh shot "$(URL)"

.PHONY: browse-close
browse-close: ## ブラウザセッションを閉じ一時ファイルを削除 (スクショは残す)
	bash scripts/browse.sh close

.PHONY: assets
assets: ## CSS / JSを static/ にビルド
	pnpm build

.PHONY: templ-generate
templ-generate: ## templファイルからGoコードを生成 (goimportsも実行)
	go tool templ generate
	@$(MAKE) goimports

.PHONY: build
build: templ-generate ## バイナリを bin/cutre にビルド
	go build -o bin/cutre ./cmd/cutre

.PHONY: fmt
fmt: ## コードをフォーマット (go fmt + goimports)
	go fmt ./...
	@$(MAKE) goimports

.PHONY: goimports
goimports: ## goimportsでimport文を整理
	go tool goimports -w .

.PHONY: goimports-check
goimports-check: ## goimportsでimport文をチェック (差分があれば失敗)
	@unformatted=$$(go tool goimports -l .) || exit $$?; \
	if [ -n "$$unformatted" ]; then \
		echo "import文が整理されていないファイルがあります:"; \
		echo "$$unformatted"; \
		echo ""; \
		echo "make goimports を実行してください"; \
		exit 1; \
	fi

.PHONY: lint
lint: sqlc-diff ## リントを実行 (sqlcの同期チェック + golangci-lint)
	golangci-lint run --config=.golangci.yml ./...

.PHONY: test
test: test-go test-js ## GoとJavaScriptのテストを実行

# set -o pipefail を付けないと make は tparse の終了コードしか見ず、失敗したテストを成功と誤認する。
.PHONY: test-go
test-go: db-setup-test ## Goのテストを実行
	@bash -c 'set -o pipefail && $(OP_RUN_TEST) go test -json -race ./... | go tool tparse -all'

.PHONY: test-js
test-js: ## フロントエンドの単体テストを実行 (Vitest)
	pnpm test

.PHONY: test-pkg
test-pkg: db-setup-test ## 特定のパッケージのテストを実行 (使用例: make test-pkg PKG=internal/handler/health)
	@if [ -z "$(PKG)" ]; then \
		echo "エラー: PKG変数を指定してください"; \
		echo "使用例: make test-pkg PKG=internal/handler/health"; \
		exit 1; \
	fi
	@bash -c 'set -o pipefail && $(OP_RUN_TEST) go test -json -race ./$(PKG) | go tool tparse -all'

.PHONY: test-run
test-run: db-setup-test ## 特定のテストを実行 (使用例: make test-run PKG=internal/handler/health RUN=TestShow)
	@if [ -z "$(PKG)" ]; then \
		echo "エラー: PKG変数を指定してください"; \
		echo "使用例: make test-run PKG=internal/handler/health RUN=TestShow"; \
		exit 1; \
	fi
	@if [ -z "$(RUN)" ]; then \
		echo "エラー: RUN変数を指定してください"; \
		echo "使用例: make test-run PKG=internal/handler/health RUN=TestShow"; \
		exit 1; \
	fi
	@bash -c 'set -o pipefail && $(OP_RUN_TEST) go test -json -race ./$(PKG) -run $(RUN) | go tool tparse -all'

.PHONY: test-verbose
test-verbose: db-setup-test ## 詳細ログ付きで全テストを実行 (デバッグ用)
	@bash -c 'set -o pipefail && $(OP_RUN_TEST) go test -json -race -count=1 ./... | go tool tparse -all'

# dbmateはマイグレーションファイルが1つも無いとエラーで終了するため、db/migrations/ に .sql が無い間はdbmate upを呼ぶターゲットを空振りさせる。
HAS_MIGRATIONS = ls db/migrations/*.sql > /dev/null 2>&1

# pg_dump 17.6以降が出力するランダムな \restrict / \unrestrict の対と、サーバーおよびpg_dumpのビルドを記した見出しを削除する。
# これらを残すとdb/schema.sqlが開発コンテナとCIで食い違い、db-check-driftの比較が成立しなくなる。
# https://github.com/amacneil/dbmate/issues/678
NORMALIZE_SCHEMA_DUMP = sed -i '/^\\restrict /d;/^\\unrestrict /d;/^-- Dumped /d' db/schema.sql

.PHONY: db-migrate
db-migrate: ## データベースマイグレーションを実行し、db/schema.sqlを更新
	@if $(HAS_MIGRATIONS); then \
		$(OP_RUN_DEV) dbmate up && $(NORMALIZE_SCHEMA_DUMP); \
	else \
		echo "db/migrations/ にマイグレーションが無いため、適用をスキップします"; \
	fi

# 開発用データベースを、既存データを捨てずに初期化する。
# `dbmate create` はデータベースが存在しなかったときだけ成功するため、スキーマのロードは新規環境の初回だけ行われ、以降の実行は未適用のマイグレーションを進めるだけになる。
# 初回のスキーマロードに失敗した場合は、その実行で作成したデータベースを削除してからエラーを返し、再実行で最初から構築し直せるようにする。
# --no-dump-schema を付けるのは、dbmateがダンプのたびにpg_dumpのバージョン見出しを書き換え、通常のセットアップでGit管理下のファイルが変更されるのを避けるため。
# マイグレーションが0件でも失敗を見落とさないよう、`dbmate create` が握り潰した接続エラーをSELECT 1で検出する。
.PHONY: db-prepare-dev
db-prepare-dev: ## 開発用データベースが無ければ構築し、未適用のマイグレーションを適用
	@if $(OP_RUN_DEV) dbmate create > /dev/null 2>&1; then \
		if ! $(OP_RUN_DEV) sh -c 'psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -q -f db/schema.sql > /dev/null'; then \
			$(OP_RUN_DEV) dbmate drop; \
			exit 1; \
		fi; \
	fi
	@$(OP_RUN_DEV) sh -c 'psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -c "SELECT 1" > /dev/null'
	@if $(HAS_MIGRATIONS); then \
		$(OP_RUN_DEV) dbmate --no-dump-schema up; \
	fi

# データベースをdb/schema.sqlから作り直す。
# マイグレーションを先頭から流さずにダンプをロードするのは、CIのtestジョブにはdbmateが無く、PostgreSQLサービスコンテナに対してpsqlだけで構築するため。
#
# スキーマのリセットに `dbmate drop` ではなくDROP SCHEMAを使うのは、他から接続が張られていると `dbmate drop` が失敗するため。
# `dbmate create` はデータベースが既にあると失敗するため、エラーは握り潰して実際の失敗は後続のリセットで報告する。
# `AUTHORIZATION pg_database_owner` と `COMMENT ON SCHEMA` は、initdbがpublicスキーマに与える所有者とコメントを再現する。
# 所有者が無いと接続ロールが所有者になり、コメントが無いとpg_dumpが `COMMENT ON SCHEMA public IS '';` を出力する。
# どちらも次回のダンプでdb/schema.sqlに不要な差分を入れ、ダンプの内容がデータベースの作り方に依存してしまう。
#
# コメント文字列の `'\''` は、この変数を `sh -c '...'` の中へ埋め込むためのエスケープで、シェルに1個のシングルクォートとして渡る。
DB_RESET_SCHEMA_SQL = SET client_min_messages TO warning; DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public AUTHORIZATION pg_database_owner; COMMENT ON SCHEMA public IS '\''standard public schema'\'';

.PHONY: db-setup-dev
db-setup-dev: ## 開発用データベースをdb/schema.sqlから作り直す (既存データは失われる)
	@$(OP_RUN_DEV) sh -c 'dbmate create > /dev/null 2>&1 || true'
	@$(OP_RUN_DEV) sh -c 'psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -c "$(DB_RESET_SCHEMA_SQL)" > /dev/null'
	@$(OP_RUN_DEV) sh -c 'psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -q -f db/schema.sql > /dev/null'

.PHONY: db-setup-test
db-setup-test: ## テスト用データベースをdb/schema.sqlから作り直す (既存データは失われる)
	@$(OP_RUN_TEST) sh -c 'dbmate create > /dev/null 2>&1 || true'
	@$(OP_RUN_TEST) sh -c 'psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -c "$(DB_RESET_SCHEMA_SQL)" > /dev/null'
	@$(OP_RUN_TEST) sh -c 'psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -q -f db/schema.sql > /dev/null'

.PHONY: db-rollback
db-rollback: ## 最後のマイグレーションをロールバック
	$(OP_RUN_DEV) dbmate down
	@$(NORMALIZE_SCHEMA_DUMP)

.PHONY: db-new
db-new: ## 新しいマイグレーションファイルを作成 (使用例: make db-new name=create_users)
	@if [ -z "$(name)" ]; then \
		echo "エラー: name変数を指定してください"; \
		echo "使用例: make db-new name=create_users"; \
		exit 1; \
	fi
	dbmate new $(name)

.PHONY: db-dump
db-dump: ## データベーススキーマをdb/schema.sqlにダンプ
	$(OP_RUN_DEV) dbmate dump
	@$(NORMALIZE_SCHEMA_DUMP)

# テスト用データベースをdb/schema.sqlから作り直し、未適用のマイグレーションを適用して、結果のダンプが元と食い違えば失敗する。
# どのデータベースもdb/schema.sqlをロードして作るため、ダンプし戻していないマイグレーションはこの検査が無いと本番で初めて実行されることになる。
# 検査に期待するのはドリフトの報告であって解消ではないため、上書きされたdb/schema.sqlは終了前に元へ戻す。
.PHONY: db-check-drift
db-check-drift: db-setup-test ## db/schema.sqlがマイグレーションと同期しているかをチェック
	@if ! $(HAS_MIGRATIONS); then \
		echo "db/migrations/ にマイグレーションが無いため、同期チェックをスキップします"; \
		exit 0; \
	fi; \
	committed=$$(mktemp); \
	cp db/schema.sql "$$committed"; \
	status=0; \
	$(OP_RUN_TEST) dbmate up > /dev/null || status=$$?; \
	if [ "$$status" -eq 0 ]; then \
		$(NORMALIZE_SCHEMA_DUMP); \
		diff -u --label "db/schema.sql" --label "db/schema.sql (dumped)" "$$committed" db/schema.sql || status=1; \
	fi; \
	cp "$$committed" db/schema.sql; \
	rm -f "$$committed"; \
	if [ "$$status" -ne 0 ]; then \
		echo "db/schema.sqlがマイグレーションとずれています。make db-migrate を実行して差分をコミットしてください" >&2; \
	fi; \
	exit "$$status"

# sqlcはクエリが1つも無いとエラーで終了するため、db/queries/ に .sql が無い間は生成も比較もしない。
SQLC_HAS_QUERIES = ls db/queries/*.sql > /dev/null 2>&1

.PHONY: sqlc-generate
sqlc-generate: ## SQLクエリからGoコードを生成
	@if $(SQLC_HAS_QUERIES); then \
		go tool sqlc generate; \
	else \
		echo "db/queries/ にクエリが無いため、sqlcの生成をスキップします"; \
	fi

# `sqlc diff` は差分を表示するだけなので、lintから呼んでもコードは書き換わらない。
.PHONY: sqlc-diff
sqlc-diff: ## sqlcの生成コードがクエリ・スキーマと同期しているかをチェック
	@if $(SQLC_HAS_QUERIES); then \
		go tool sqlc diff; \
	else \
		echo "db/queries/ にクエリが無いため、sqlcの同期チェックをスキップします"; \
	fi
