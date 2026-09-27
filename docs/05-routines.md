# 05 — 夜間Routine設計

Claude Code Routines（クラウド実行、毎回ゼロコンテキストで起動、実行中に承認プロンプトなし）を前提とする。
Max Plan には1日あたりのクラウドスケジュールセッション数に上限があるため、**ジョブは集約する**。

## Routine 一覧

| 名前 | 頻度 | 役割 | プロンプト | マージ |
|---|---|---|---|---|
| `nightly-improve` | 毎日 08:00 JST | GitHub Issues の `ready` を優先度順に 1 件実装し PR | `.claude/routines/nightly-improve.md` | CI が判定（ADR-0006） |
| `weekly-report` | 毎週月曜 | KPI推移・無料枠使用量・未マージPRの棚卸し・**Secrets の期限チェック**を `reports/` に生成 | 未作成 | CI が判定（ADR-0006） |

（日次クロール自体は GitHub Actions cron。Routine ではない）

nightly-improve を 08:00 にするのは、日次レポートが出た後に動かすため（cron は 03:00 だが scheduled 実行の遅延で
クロール開始は 06:00 前後、レポートは 06:30 前後。`docs/lessons.md`）。調査系の Issue が当日のレポートを読める。

Routine は自分でマージしない。PR を作ったら終わり。自動マージの条件（AGENTS.md）を満たせば CI がマージし、満たさなければ `needs-owner-review` が付いてオーナーを待つ。

Routine の登録（claude.ai/code の Routines 画面）はオーナーが行う。登録するプロンプトは「リポジトリの `.claude/routines/<名前>.md` を読み、その手順に従う」だけにし、手順の本体はリポジトリに置く（変更を PR で追えるようにするため）。

## weekly-report の Secrets 期限チェック

1. `docs/03-guardrails.md`「シークレットの期限」の表を読む
2. 実行日から各 Secret の期限までの残り日数を計算する
3. **残り 30 日を切った項目**を、その週の `reports/` に警告として書く。書くこと：Secret 名、期限、残り日数、切れると止まるもの、更新手順（新しい値を発行 → GitHub Secrets を更新 → 表の期限を書き直す PR）
4. 表の「切れると止まるもの」を警告に添える（オーナーが緊急度を判断できるようにするため）。2026-09 時点で期限付きの Secret は無い
5. 該当がなければ「Secrets の期限：問題なし（次の期限 YYYY-MM-DD）」と 1 行だけ書く

Routine は Secrets の値を読めない（読まない）。期限は表に書かれた日付だけを信じる。

## nightly-improve：Issue の `ready` を消化する

**何をやるかはオーナーが Issue で決め、Routine はその順に実装する。** Routine が自分で改善を選ぶと
無意味な変更が積み上がりやすく、何が進んでいるかも Issue を見れば分かるようにしたい。
手順とコマンドは `.claude/routines/nightly-improve.md`。ここには設計（何を・なぜ）だけを書く。

### Issue の書き方

テンプレート `.github/ISSUE_TEMPLATE/task.md` を使い、**目的・完了条件・触ってよいパス**の 3 つを書く。
3 つが埋まっていない Issue には `ready` を付けない。AI（対話セッション・Routine）が書く Issue は必ず `proposal` を付けて起票し、`ready` を付けるのはオーナーだけ。
Routine が起票するのは、実装中に範囲外の改善に気づいた時だけ（AGENTS.md「実装中に気づいた改善の扱い」。1 回の実行で 3 件まで）。

### ラベル

| ラベル | 意味 | 付ける | 外す |
|---|---|---|---|
| `ready` | 着手してよい。目的・完了条件・触ってよいパスが書いてある | オーナー | Routine（着手時・`needs-clarification` にする時） |
| `in-progress` | Routine が着手した。PR のマージで Issue ごと閉じる | Routine（着手時） | Routine（行き詰まった時）。PR が無いまま残っていたらオーナー |
| `needs-clarification` | Routine が判断に迷い着手しなかった（または行き詰まった）。理由は Issue のコメント | Routine | オーナー（答えを書いて外し、`ready` を付け直す） |
| `proposal` | AI（対話セッション・Routine）が起票した提案。オーナーが採用するまで着手しない | 対話セッション・Routine | オーナー（採用なら `ready` に替える、不採用なら閉じる） |
| `priority:high` / `priority:medium` / `priority:low` | 消化の順番。無印は `low` と同じ | オーナー | オーナー |

状態の流れ：`proposal` →（オーナーが採用）→ `ready` →（Routine が着手）→ `in-progress` →（PR マージ）→ closed。
迷ったら `needs-clarification` →（オーナーが答える）→ `ready` に戻る。

### 1 回の実行でやること

1 回の実行で扱う Issue は **1 件だけ**。PR を小さく保ち、失敗しても影響を 1 件に閉じるため。

1. `ready` のうち `in-progress` / `needs-clarification` が付いていないものを、`priority:high` → `medium` → それ以外の順、同じ優先度なら Issue 番号の小さい順に 1 件選ぶ
2. `ready` が無ければ何もせず終了する（自分で改善を探しに行かない）
3. 判断に迷う Issue（完了条件が曖昧、触ってよいパスの外が要る、1 PR に収まらない、docs と矛盾する、実サイトへのアクセスが要る）は着手せず、コメントして `needs-clarification` を付けて終了する
4. 着手時に `ready` を `in-progress` に替え、実装・検証し、`Closes #N` を本文に書いた PR を作って終了する。
   実装中に気づいた範囲外の改善は、ガードレール強化なら同じ PR に入れ、それ以外は `proposal` で起票する

「何もしない」を正解として明示するのが重要。改善を強制すると無意味な変更が積み上がる。

## 優先度の目安（オーナーが Issue に付ける時の基準）

1. `priority:high`：クロール失敗・抽出精度低下（データが止まると全てが止まる）
2. `priority:medium`：対象案件数の拡大、SEO（sitemap・構造化データ・内部リンク）
3. `priority:low`：ドキュメントの陳腐化修正

## オーナーの週次レビュー

- `reports/` を読む（10分）
- `needs-owner-review` の PR を処理する（20分）
- `needs-clarification` の Issue に答え、`proposal` を採用するか決める。Issue を書き、`ready` と優先度を付ける
- PR が無いまま残っている `in-progress` を確認する（Routine が途中で落ちた可能性）
- 事故があれば `logs/incidents/` を書き、ガードレールを追加する
