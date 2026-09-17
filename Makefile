.PHONY: help
help: ## ヘルプを表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: dev
dev: ## 全サービスの開発サーバーを起動
	hivemind Procfile.dev

.PHONY: fmt
fmt: ## コードをフォーマット (Oxfmt)
	pnpm fmt

.PHONY: fmt-check
fmt-check: ## フォーマットチェック (Oxfmt)
	pnpm fmt:check

# koryluslintはKorylus共通のリンタで、mdサブコマンドがMarkdownの句点改行を検査する。
# バージョンは go/go.mod の tool ディレクティブで固定している。
#
# Goモジュールは go/ にある一方で走査対象のMarkdownはリポジトリルートにあり、
# `go tool koryluslint` はモジュールのディレクトリを起点に走査してしまう。
# そのため固定版のバイナリをビルドし、リポジトリルートから実行する。
KORYLUSLINT ?= /tmp/koryluslint

.PHONY: koryluslint-build
koryluslint-build:
	@go -C go build -o $(KORYLUSLINT) github.com/korylus/tools/cmd/koryluslint

.PHONY: lint-md
lint-md: koryluslint-build ## Markdownの句点改行をチェック (変更行のみ)
	@$(KORYLUSLINT) md

.PHONY: lint-md-base
lint-md-base: koryluslint-build ## BASE refとの差分行でMarkdownの句点改行をチェック (例: BASE=origin/main)
	@$(KORYLUSLINT) md -base=$(BASE)

.PHONY: lint-md-fix
lint-md-fix: koryluslint-build ## Markdownの句点改行を自動修正 (変更ファイル)
	@$(KORYLUSLINT) md --write
