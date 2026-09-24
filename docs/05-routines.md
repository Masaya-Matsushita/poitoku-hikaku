# 05 — 夜間Routine設計

Claude Code Routines（クラウド実行、毎回ゼロコンテキストで起動、実行中に承認プロンプトなし）を前提とする。
Max Plan には1日あたりのクラウドスケジュールセッション数に上限があるため、**ジョブは集約する**。

## Routine 一覧

| 名前 | 頻度 | 役割 | マージ |
|---|---|---|---|
| `nightly-improve` | 毎日 03:00 JST | GitHub Issues の `ready` を優先度順に 1 件消化し PR | CI が判定（ADR-0006） |
| `weekly-report` | 毎週月曜 | KPI推移・無料枠使用量・未マージPRの棚卸し・**Secrets の期限チェック**を `reports/` に生成 | CI が判定（ADR-0006） |

（日次クロール自体は GitHub Actions cron。Routine ではない）

Routine は自分でマージしない。PR を作ったら終わり。自動マージの条件（AGENTS.md）を満たせば CI がマージし、満たさなければ `needs-owner-review` が付いてオーナーを待つ。

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

（2026-09-25 時点では設計のみ。ラベルの作成と Routine の登録は次の PR で行う）

### Issue とラベル

| ラベル | 意味 | 付ける人 |
|---|---|---|
| `ready` | 着手してよい。本文に目的と受け入れ条件がある | オーナー（例外は下の「異常時の起票」） |
| `priority:high` / `priority:medium` / `priority:low` | 消化の順番。無ければ `low` 扱い | オーナー |

Issue の本文には「なぜやるか」「受け入れ条件（何が確認できたら完了か）」を書く。書けない Issue には `ready` を付けない。
Issue はオーナーか対話セッションが書く。

### 1 回の実行でやること

1 回の実行で扱う Issue は 1 件。PR を小さく保ち、失敗しても影響を 1 件に閉じるため。

1. `AGENTS.md` を読む
2. open で `ready` が付いた Issue から 1 件選ぶ：`priority:high` → `medium` → `low`（無印）の順、同じ優先度なら古い順。
   その Issue を閉じる open な PR（本文に `Closes #N`）が既にあるものは飛ばす
3. Issue と関連する `docs/` を読む。受け入れ条件が曖昧・1 PR に収まらない・`docs/` と矛盾する場合は実装しない。
   Issue に何が足りないかをコメントし、`ready` を外して終わる
4. 実装し、AGENTS.md の検証を通して PR を作る。本文に `Closes #N`、何をしたか、受け入れ条件をどう確かめたかを書く
5. マージは CI に任せて終わる。`needs-owner-review` が付いても何もしない（オーナーを待つ）

`ready` の Issue が無い時：

6. 最新の `reports/` の「判定」を読む。**クロール失敗・抽出精度低下**（データが止まると全てが止まる）があれば、
   根拠（レポートの日付と数値）を書いた Issue を起票し、`ready` と `priority:high` を付けて 2 に戻る（異常時の起票）
7. それ以外に気づいた改善は、`ready` を付けずに起票してよい（オーナーが優先度を決める）。起票は 1 回の実行で 1 件まで
8. 何も無ければ何もせず「異常なし」とだけ報告する

### プロンプト骨子

```
あなたはポイ得比較の夜間改善担当です。docs/05-routines.md「nightly-improve」の手順に従う。
1. AGENTS.md を読む
2. ready の Issue を優先度順（priority:high → medium → low/無印、同順位は古い順）に 1 件選ぶ。Closes する open PR があるものは飛ばす
3. 受け入れ条件が曖昧・大きすぎる・docs と矛盾するなら、コメントして ready を外して終わる
4. 実装し、検証を通し、PR を作る（Closes #N、何をしたか、受け入れ条件の確かめ方）
5. 自分でマージしない
6. ready が無ければ最新 reports/ の判定を読み、クロール失敗・抽出精度低下なら Issue を起票して ready と priority:high を付けて 2 へ
7. 何も無ければ何もせず「異常なし」とだけ報告する

禁止：.env* の読み取り、リクエスト間隔の短縮、破壊的マイグレーション、有料サービスの有効化、gh pr merge
```

「何もしない」を正解として明示するのが重要。改善を強制すると無意味な変更が積み上がる。

## 優先度の目安（オーナーが Issue に付ける時の基準）

1. `priority:high`：クロール失敗・抽出精度低下（データが止まると全てが止まる）
2. `priority:medium`：対象案件数の拡大、SEO（sitemap・構造化データ・内部リンク）
3. `priority:low`：ドキュメントの陳腐化修正

## オーナーの週次レビュー

- `reports/` を読む（10分）
- `needs-owner-review` の PR を処理する（20分）
- Issue を書き、`ready` と優先度を付ける。Routine が `ready` を外した Issue に答える
- 事故があれば `logs/incidents/` を書き、ガードレールを追加する
