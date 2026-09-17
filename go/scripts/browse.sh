#!/usr/bin/env bash
#
# browse.sh はplaywright-cliを駆動して、devサイトのブラウザ確認 (環境の検証・スクリーンショット・後片付け) を行う。
#
# dev URLのoriginは KORYLUS_BROWSING_BASE_URL から、exe.devのプロキシが認証に使うVMトークンは CUTRE_EXE_DEV_API_KEY から読む。
# .envをシェルで評価するとトークンに含まれる `$` が展開されて壊れるため、op runのラッパー配下 (go/Makefile の browse-* ターゲット) から実行する。
set -euo pipefail

SESSION=dev
TMP_DIR=/workspace/tmp
CONFIG_FILE="$TMP_DIR/browse-cli.config.json"
CURL_CONFIG_FILE="$TMP_DIR/browse-cli.curlrc"
SHOT_DIR="$TMP_DIR/browse"
AUTH_HEADER=X-Exedev-Authorization

pw() { playwright-cli -s="$SESSION" "$@"; }

# playwright-cliはコマンドが失敗しても終了コード0のまま出力にエラーを書くため、出力からも失敗を検出する。
# 呼び出し側が成功時の出力を捨てても、エラーはstderrに残す。
pw_checked() {
  local output
  if ! output="$(pw "$@" 2>&1)" || [[ "$output" == *"### Error"* ]]; then
    printf '%s\n' "$output" >&2
    return 1
  fi
  printf '%s\n' "$output"
}

# 資格情報の値は出力せず、設定の有無と形式だけを検証する。
check_environment() {
  local failed=0

  if [ "${APP_ENV:-}" != "dev" ]; then
    echo "APP_ENV が dev ではありません (go/Makefile の browse-* ターゲットから実行してください)" >&2
    failed=1
  fi

  if [ -z "${CUTRE_EXE_DEV_API_KEY:-}" ]; then
    echo "CUTRE_EXE_DEV_API_KEY が設定されていません" >&2
    failed=1
  fi

  local command_name
  for command_name in node curl playwright-cli; do
    if ! command -v "$command_name" >/dev/null 2>&1; then
      echo "$command_name が見つかりません" >&2
      failed=1
    fi
  done

  # トークン方式の前段は401チャレンジを返さないため、URLに埋め込んだ資格情報は送られない。
  # 埋め込まれていれば設定の誤りとして止め、以降の出力にoriginとして資格情報が現れないようにする。
  if [ -z "${KORYLUS_BROWSING_BASE_URL:-}" ]; then
    echo "KORYLUS_BROWSING_BASE_URL が設定されていません" >&2
    failed=1
  elif command -v node >/dev/null 2>&1; then
    node -e '
      let url;
      try {
        url = new URL(process.env.KORYLUS_BROWSING_BASE_URL);
      } catch {
        console.error("KORYLUS_BROWSING_BASE_URL がURLとして解釈できません");
        process.exit(1);
      }
      if (url.protocol !== "http:" && url.protocol !== "https:") {
        console.error("KORYLUS_BROWSING_BASE_URL はhttpかhttpsである必要があります");
        process.exit(1);
      }
      if (url.username || url.password) {
        console.error("KORYLUS_BROWSING_BASE_URL に資格情報を含めないでください (トークンは CUTRE_EXE_DEV_API_KEY に設定します)");
        process.exit(1);
      }
    ' || failed=1
  fi

  return "$failed"
}

origin() {
  node -e 'process.stdout.write(new URL(process.env.KORYLUS_BROWSING_BASE_URL).origin)'
}

# トークンをargvに載せないよう、curlには0600の設定ファイルからヘッダーを読ませる。
# trapをサブシェル内に閉じ込め、どの終了経路でも設定ファイルを削除する。
check_reachable() (
  trap 'rm -f "$CURL_CONFIG_FILE"' EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM

  mkdir -p "$TMP_DIR"
  node -e '
    const fs = require("fs");
    const token = process.env.CUTRE_EXE_DEV_API_KEY;
    if (/[\u0000-\u001f\u007f"\\]/u.test(token)) {
      console.error("CUTRE_EXE_DEV_API_KEY に制御文字・引用符・バックスラッシュを含めないでください");
      process.exit(1);
    }
    fs.rmSync(process.argv[1], { force: true });
    fs.writeFileSync(process.argv[1], `header = "${process.argv[2]}: Bearer ${token}"\n`, { mode: 0o600 });
  ' "$CURL_CONFIG_FILE" "$AUTH_HEADER"

  local response
  if ! response="$(curl -sS --max-time 10 -K "$CURL_CONFIG_FILE" -w '\n%{http_code}' "$(origin)/health")"; then
    echo "dev URLが応答しません" >&2
    return 1
  fi

  # exe.devのプロキシはトークンが無いとログインページへリダイレクトし、誤っていると401を返す。
  # ステータスだけでなく本文も見て、Cutreのアプリ自身が応答したことを確かめる。
  local status="${response##*$'\n'}"
  local body="${response%$'\n'*}"
  case "$status" in
    200) ;;
    401 | 301 | 302 | 303 | 307 | 308)
      echo "dev URLが /health に $status を返しました (CUTRE_EXE_DEV_API_KEY のトークンが拒否された可能性があります)" >&2
      return 1
      ;;
    *)
      echo "dev URLが /health に $status を返しました" >&2
      return 1
      ;;
  esac
  if [ "$body" != '{"status":"ok"}' ]; then
    echo "dev URLの /health がCutreの応答ではありません (make dev でアプリが起動しているか確認してください)" >&2
    return 1
  fi
)

cleanup_session() {
  pw close >/dev/null 2>&1 || true
  rm -f "$CONFIG_FILE" "$CURL_CONFIG_FILE"
}

# 名前付きセッションが無いときだけ、トークンのヘッダーを全リクエストに付けるconfigでブラウザを開く。
# exe.devのプロキシは401チャレンジを返さないため、httpCredentialsではなくextraHTTPHeadersで常時送る。
ensure_session() {
  local sessions
  sessions="$(playwright-cli list 2>&1 || true)"
  if [[ "$sessions" == *"- $SESSION:"* ]]; then
    return 0
  fi

  check_environment

  # ここから先で失敗したら、トークンを持つconfigと開きかけのセッションを残さない。
  trap cleanup_session EXIT

  mkdir -p "$TMP_DIR"
  # modeはファイルの新規作成時にしか効かないため、既存のファイルを消してから書く。
  node -e '
    const fs = require("fs");
    const cfg = { browser: { contextOptions: { extraHTTPHeaders: {
      [process.argv[2]]: `Bearer ${process.env.CUTRE_EXE_DEV_API_KEY}`,
    } } } };
    fs.rmSync(process.argv[1], { force: true });
    fs.writeFileSync(process.argv[1], JSON.stringify(cfg), { mode: 0o600 });
  ' "$CONFIG_FILE" "$AUTH_HEADER"

  pw_checked open --browser=chromium --config="$CONFIG_FILE" >/dev/null

  # ヘッダーはブラウザコンテキストが保持したため、トークンをディスクに残さない。
  rm -f "$CONFIG_FILE"
  trap - EXIT
}

cmd_check() {
  check_environment
  check_reachable
  echo "ブラウザ確認に必要な環境が整っています"
}

cmd_shot() {
  local path="${1:-/}"
  if [[ "$path" != /* ]]; then
    echo "URL は / から始まるパスで指定してください (例: make browse-shot URL=/en)" >&2
    exit 2
  fi

  ensure_session

  local base
  base="$(origin)"
  local name
  name="$(printf '%s' "$path" | sed 's#[^a-zA-Z0-9]#_#g; s#^_*##')"
  [ -n "$name" ] || name=home
  local filename="$SHOT_DIR/$name.png"

  # 撮影に失敗したとき、古い画像を新しい結果と取り違えないよう先に消す。
  mkdir -p "$SHOT_DIR"
  rm -f "$filename"

  pw_checked goto "$base$path" >/dev/null

  # --rawは返り値の文字列を二重引用符で囲むため、取り除いてから比べる。
  local result
  result="$(pw_checked --raw run-code "async page => {
    await page.waitForLoadState('networkidle');
    const status = await page.evaluate(() => performance.getEntriesByType('navigation')[0]?.responseStatus ?? 0);
    return status + ' ' + page.url();
  }")"
  result="${result%\"}"
  result="${result#\"}"
  local status="${result%% *}"
  local actual_url="${result#* }"

  # トークンが通らないとexe.devのログインページ (別origin) へ飛ばされるか、同じoriginで401のページが返るため、撮影前に止める。
  if [[ "$actual_url" != "$base" && "$actual_url" != "$base/"* ]]; then
    echo "dev URLのoriginから外れました: $actual_url (make browse-check で環境を確認してください)" >&2
    exit 1
  fi
  if [ "$status" = 401 ]; then
    echo "dev URLが401を返しました (make browse-close のあと make browse-check で環境を確認してください)" >&2
    exit 1
  fi

  pw_checked screenshot --filename="$filename" >/dev/null
  if [ ! -s "$filename" ]; then
    echo "スクリーンショットが作成されませんでした: $filename" >&2
    exit 1
  fi

  # 同一originへのリダイレクト (末尾スラッシュの正規化など) は上の判定を通るため、実際に撮ったURLを添える。
  echo "スクリーンショット: $filename ($status $actual_url)"
}

cmd_close() {
  cleanup_session
  echo "ブラウザセッションを閉じ、一時ファイルを削除しました"
}

case "${1:-}" in
  check)
    cmd_check
    ;;
  shot)
    shift
    cmd_shot "${1:-/}"
    ;;
  close)
    cmd_close
    ;;
  *)
    echo "使い方: browse.sh {check | shot <path> | close}" >&2
    exit 2
    ;;
esac
