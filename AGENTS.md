# Cutre開発ガイドライン

このファイルは、コーディングエージェントがこのリポジトリで作業する際のガイダンスを提供します。

## 概要

Cutreはキャラクターグッズの物々交換を簡単にするためのサービスです。

## 技術スタック

- Go 1.27.1
  - chi/v5: HTTPルーターとミドルウェア
  - templ: HTMLテンプレートエンジン
  - go-i18n/v2: 国際化
  - database/sql + pgx (stdlibドライバ): PostgreSQLへの接続
  - sqlc: SQLクエリからGoコードを生成
  - river: バックグラウンドジョブキュー (PostgreSQLを使う。テーブルはdbmateのマイグレーションで管理する)
- PostgreSQL 18.6
- dbmate: データベースマイグレーション
- pnpm (Node.js 24)
  - Tailwind CSS v4
  - Basecoat 1.0: UIコンポーネントライブラリ
  - htmx 4: npmパッケージ `htmx.org` をesbuildでバンドルする

ツールのバージョンの正本は [mise.toml](./mise.toml)・[go/go.mod](./go/go.mod)・[go/package.json](./go/package.json)・[docker-compose.yml](./docker-compose.yml) です。
バージョンを上げたときは、この節もあわせて更新してください。

Cutreが定義する環境変数には `CUTRE_` プレフィックスを付けます (`DATABASE_URL` など、外部ツールが要求するものを除く)。

## 開発コマンド

コマンドは開発コンテナ内の `/workspace` で実行します。

```sh
# 開発サーバー群 (アセットのウォッチ + Goサーバーのhot reload) を起動
make dev

# テスト
make -C go test                                          # GoとJavaScriptの全テスト
make -C go test-pkg PKG=internal/handler/health          # パッケージ指定
make -C go test-run PKG=internal/handler/health RUN=TestShow  # テスト指定

# フォーマット・リント
make -C go templ-generate  # templファイルからGoコードを生成
make -C go fmt             # go fmt + goimports
make -C go lint            # sqlcの同期チェック + golangci-lint
make fmt                   # Markdown・JavaScriptなど (Oxfmt)
make lint-md-base BASE=origin/main  # Markdownの句点改行 (基準ブランチとの差分行)

# データベース
make -C go db-new name=create_users  # マイグレーションファイルを作成
make -C go db-migrate                # マイグレーションを実行し、db/schema.sqlを更新
make -C go sqlc-generate             # SQLクエリからGoコードを生成

# デザイントークン
make -C go tokens-generate  # web/tokens.json から web/tokens.css を生成
```

ほかのターゲットは `make help` / `make -C go help` で確認できます。

## 開発ワークフロー

### 実装時のガイドライン

**既存コードとの一貫性**:

実装を行う前に、コードベース内に類似の処理がないか確認してください。
類似処理が存在する場合は、そのパターンに従って実装することで、コードベース全体の一貫性を保ちます。

### 実装後のチェック

実装を終え作業の完了を伝える前に、必ず以下を確認してください:

- コードフォーマット
- リント
- テスト

実行するコマンドは `Makefile` で管理しています。
[Makefile](./Makefile), [go/Makefile](./go/Makefile) を参照してください。

## Pull Requestのガイドライン

- 1つのPull Requestの変更ファイル数は20以下を目安にする
- 実装コードの変更は300行以下を目安にする (テストコードの行数は制限しない)
- 実装とそのテストは同じPull Requestに含める
