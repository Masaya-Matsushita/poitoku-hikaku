#!/usr/bin/env bash
# SQL ファイルから破壊的な文を検出する（docs/03-guardrails.md「破壊的マイグレーション」）。
#
# 判定：コメントを除いた各文（; 区切り）に、次のいずれかが含まれる
#   - drop            （drop table / drop column / drop index ... すべて）
#   - truncate
#   - alter ... type  （alter table ... alter column ... type ...）
# 大文字小文字は区別しない。文字列リテラル内の語も拾うため、疑わしい側に倒れる（誤検知は
# オーナーが見て外せばよい。見逃しの方が困る）。
#
# 使い方:   detect-destructive-sql.sh <file.sql>...
# 出力:     該当した文を「<file>: <文の先頭 160 字>」で 1 行ずつ stdout へ
# 終了コード: 0 = 該当なし、1 = 該当あり、2 = 引数なし
set -euo pipefail

if [ "$#" -lt 1 ]; then
  echo "usage: $0 <file.sql>..." >&2
  exit 2
fi

found=0
for file in "$@"; do
  matches="$(perl -0777 -ne '
    s{/\*.*?\*/}{ }gs;      # ブロックコメント
    s{--[^\n]*}{}g;         # 行コメント
    s{\s+}{ }g;             # 改行・連続空白を 1 つに
    for my $stmt (split /;/) {
      $stmt =~ s/^\s+|\s+$//g;
      next if $stmt eq "";
      if ($stmt =~ /\bdrop\b/i || $stmt =~ /\btruncate\b/i || $stmt =~ /\balter\b.*\btype\b/i) {
        print substr($stmt, 0, 160), "\n";
      }
    }
  ' "$file")"
  if [ -n "$matches" ]; then
    found=1
    while IFS= read -r line; do
      printf '%s: %s\n' "$file" "$line"
    done <<< "$matches"
  fi
done

exit "$found"
