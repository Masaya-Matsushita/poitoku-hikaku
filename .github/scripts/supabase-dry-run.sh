#!/usr/bin/env bash
# supabase db push --dry-run を本番に対して実行し、PR コメント本文と判定結果を作る
# （ci.yml の supabase-dry-run ジョブ。運用は supabase/README.md）。
#
# 使い方:   supabase-dry-run.sh <project-ref> <出力ディレクトリ>
# 環境変数: SUPABASE_ACCESS_TOKEN / SUPABASE_DB_PASSWORD  CLI が読む（GitHub Secrets）
#           SUPABASE_BIN   テスト用に CLI を差し替える。既定は supabase
#           GITHUB_OUTPUT  あれば status / destructive / pending_count を書く
# 生成物（出力ディレクトリ内）:
#   dry-run.json     CLI の構造化出力 {"upToDate","dryRun","migrations",...}
#   dry-run.log      CLI の stderr（接続状況、エラー）
#   pending.txt      適用予定のマイグレーションファイル（1 行 1 件）
#   destructive.txt  破壊的と判定した文（detect-destructive-sql.sh の出力）
#   comment.md       PR コメント本文
# 終了コード: CLI が失敗したら 1（comment.md にエラーを書いた上で）。判定結果では失敗しない
set -euo pipefail

ref="${1:?usage: $0 <project-ref> <out-dir>}"
out="${2:?usage: $0 <project-ref> <out-dir>}"
mkdir -p "$out"

here="$(cd "$(dirname "$0")" && pwd)"
supabase_bin="${SUPABASE_BIN:-supabase}"
marker='<!-- supabase-migration-dry-run -->'
sha="${GITHUB_SHA:-$(git rev-parse HEAD)}"
# GitHub のコメント上限は 65,536 字。1 ファイルあたり載せる SQL をこの字数で切る
max_sql_chars=20000

set_output() {
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    echo "$1=$2" >> "$GITHUB_OUTPUT"
  fi
}

# ---------------------------------------------------------------------------
# 1. dry-run（適用はしない）。deploy.yml の本番適用と同じフラグを使う
# ---------------------------------------------------------------------------
set +e
"$supabase_bin" db push --dry-run --include-all --project-ref "$ref" --output-format json --yes \
  > "$out/dry-run.json" 2> "$out/dry-run.log"
code=$?
set -e

if [ "$code" -ne 0 ]; then
  {
    echo "$marker"
    echo "## Supabase マイグレーション dry-run：失敗"
    echo
    echo "コミット \`${sha:0:7}\`、プロジェクト \`$ref\`。\`supabase db push --dry-run --include-all\` が終了コード $code で失敗しました。"
    echo
    echo '```text'
    tail -c 6000 "$out/dry-run.log" | sed 's/\x1b\[[0-9;]*m//g'
    echo '```'
    echo
    echo "典型的な原因：Secrets（\`SUPABASE_ACCESS_TOKEN\` / \`SUPABASE_DB_PASSWORD\`）の未設定、リモートにだけ存在する履歴（\`supabase migration repair\` が必要）、DB への接続失敗。"
  } > "$out/comment.md"
  set_output status error
  set_output destructive false
  set_output pending_count 0
  echo "dry-run failed (exit $code). see $out/dry-run.log" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# 2. 適用予定ファイルの解決。CLI はファイル名だけを返すことがあるので migrations/ を補う
# ---------------------------------------------------------------------------
jq -r '.migrations[]?' "$out/dry-run.json" | while IFS= read -r m; do
  [ -n "$m" ] || continue
  case "$m" in
    */*) echo "$m" ;;
    *)   echo "supabase/migrations/$m" ;;
  esac
done > "$out/pending.txt"

pending_count="$(grep -c . "$out/pending.txt" || true)"
files=()
while IFS= read -r f; do
  files+=("$f")
done < "$out/pending.txt"

# ---------------------------------------------------------------------------
# 3. 破壊的変更の判定（drop / alter ... type / truncate）
# ---------------------------------------------------------------------------
: > "$out/destructive.txt"
destructive=false
if [ "$pending_count" -gt 0 ]; then
  set +e
  "$here/detect-destructive-sql.sh" "${files[@]}" > "$out/destructive.txt"
  dcode=$?
  set -e
  case "$dcode" in
    0) destructive=false ;;
    1) destructive=true ;;
    *) echo "detect-destructive-sql.sh failed (exit $dcode)" >&2; exit 1 ;;
  esac
fi

# ---------------------------------------------------------------------------
# 4. コメント本文
# ---------------------------------------------------------------------------
{
  echo "$marker"
  echo "## Supabase マイグレーション dry-run"
  echo
  echo "コミット \`${sha:0:7}\`、プロジェクト \`$ref\`、コマンド \`supabase db push --dry-run --include-all\`（main マージ後に deploy.yml が同じフラグで適用）。"
  echo
  if [ "$pending_count" -eq 0 ]; then
    echo "適用予定のマイグレーションはありません。リモートは最新です。"
  else
    echo "### 適用予定（$pending_count 件、この順で適用）"
    echo
    for f in "${files[@]}"; do
      echo "- \`$(basename "$f")\`"
    done
    echo
    echo "### 判定"
    echo
    if [ "$destructive" = true ]; then
      echo "> [!WARNING]"
      echo "> **破壊的変更を含みます**（drop / alter ... type / truncate）。ラベル \`destructive-migration\` を付けました。自動マージの対象外で、オーナーの承認が必要です（\`docs/03-guardrails.md\`）。"
      echo
      echo '```text'
      cat "$out/destructive.txt"
      echo '```'
    else
      echo "破壊的変更（drop / alter ... type / truncate）は検出されませんでした。"
    fi
    echo
    echo "### 実行される SQL"
    echo
    for f in "${files[@]}"; do
      echo "<details><summary><code>$f</code></summary>"
      echo
      echo '```sql'
      if [ "$(wc -c < "$f")" -gt "$max_sql_chars" ]; then
        head -c "$max_sql_chars" "$f"
        echo
        echo "-- （以下省略。全文はファイルを参照）"
      else
        cat "$f"
      fi
      echo '```'
      echo
      echo "</details>"
      echo
    done
  fi
  echo
  echo "<sub>CLI の出力: <code>$(jq -c . "$out/dry-run.json")</code></sub>"
} > "$out/comment.md"

set_output status ok
set_output destructive "$destructive"
set_output pending_count "$pending_count"
echo "pending=$pending_count destructive=$destructive comment=$out/comment.md"
