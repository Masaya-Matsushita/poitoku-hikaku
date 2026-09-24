// 自動マージの判定（ADR-0006）。.github/workflows/automerge.yml から actions/github-script で呼ぶ。
// CI（ci.yml）が完了するたびに、その head コミットを持つ PR を 1 件判定する。
// PR のコードはチェックアウトも実行もしない（API で変更ファイル名とラベルだけを見る）。
'use strict';

const OWNER_REVIEW_LABEL = 'needs-owner-review';
const DESTRUCTIVE_LABEL = 'destructive-migration';
const COMMENT_MARKER = '<!-- auto-merge-gate -->';

// オーナー承認が必要なパス（AGENTS.md「自動マージしてよい範囲」、docs/03-guardrails.md）。
// この一覧を変える PR 自体が .github/** に触れるので、オーナー承認になる
const OWNER_PATHS = [
  // ワークフローだけでなく CI が実行するスクリプト（破壊的 SQL の検出、この判定）も含める
  { match: (p) => p.startsWith('.github/'), rule: '.github/**' },
  { match: (p) => p.startsWith('crawler/internal/policy/'), rule: 'crawler/internal/policy/**' },
  { match: (p) => p === 'docs/03-guardrails.md', rule: 'docs/03-guardrails.md' },
];

// 対象サイトの定義。既存ファイルの変更（セレクタの修復）は自動マージ可、新規追加（対象サイトが増える）はオーナー承認
const SITE_DEFINITION = /^crawler\/sites\/[^/]+\.ya?ml$/;
const NEW_FILE_STATUSES = new Set(['added', 'copied', 'renamed']);

// 判定する CI の結果。取り消し（新しい push に置き換わった）や承認待ちは判定せず、次の CI の完了を待つ
const JUDGED_CONCLUSIONS = new Set(['success', 'failure', 'timed_out', 'startup_failure']);

// files は pulls.listFiles の要素（filename / status / previous_filename）。オーナー承認が要る理由を返す
function ownerReviewReasons(files) {
  const reasons = [];
  for (const f of files) {
    // 移動は移動元も見る（policy から外へ出す変更も policy の変更）
    const paths = [f.filename, f.previous_filename].filter(Boolean);
    for (const { match, rule } of OWNER_PATHS) {
      if (paths.some(match)) reasons.push(`\`${f.filename}\`（${rule}）`);
    }
    if (SITE_DEFINITION.test(f.filename) && NEW_FILE_STATUSES.has(f.status)) {
      reasons.push(`\`${f.filename}\`（crawler/sites/*.yaml の新規追加）`);
    }
  }
  return reasons;
}

// merge：auto-merge を有効にするか。ownerReview：needs-owner-review を付けるか。
// CI 未通過と draft はどちらでもない（オーナーの判断ではなく、PR 側で直すもの）
function decide({ ciConclusion, pr, files, repoFullName }) {
  if (ciConclusion !== 'success') {
    return { merge: false, ownerReview: false, reasons: [], summary: `CI 未通過（${ciConclusion}）` };
  }
  if (pr.draft) {
    return { merge: false, ownerReview: false, reasons: [], summary: 'draft の PR' };
  }

  const labels = pr.labels.map((l) => l.name);
  const reasons = [];
  if (pr.head.repo?.full_name !== repoFullName) reasons.push('fork からの PR');
  if (labels.includes(DESTRUCTIVE_LABEL)) reasons.push(`\`${DESTRUCTIVE_LABEL}\` ラベル（破壊的マイグレーション）`);
  // 一度付いたラベルは外さない（外すのはオーナー）。付いている間は自動マージしない
  if (labels.includes(OWNER_REVIEW_LABEL)) reasons.push(`\`${OWNER_REVIEW_LABEL}\` ラベルが付いている`);
  // pulls.listFiles は 3000 件までしか返さない。見えないファイルがあれば安全側に倒す
  if (files.length < pr.changed_files) reasons.push(`変更ファイルを全件取得できない（${pr.changed_files} 件中 ${files.length} 件）`);
  reasons.push(...ownerReviewReasons(files));

  if (reasons.length > 0) {
    return { merge: false, ownerReview: true, reasons, summary: 'オーナー承認が必要' };
  }
  return { merge: true, ownerReview: false, reasons: [], summary: '自動マージの条件を満たす' };
}

async function findPullRequest({ github, owner, repo, run }) {
  const numbers = new Set((run.pull_requests || []).map((p) => p.number));
  if (numbers.size === 0) {
    // workflow_dispatch で起動した CI（report.yml）は pull_requests が空のことがあるので、コミットから引く
    const { data } = await github.rest.repos.listPullRequestsAssociatedWithCommit({
      owner, repo, commit_sha: run.head_sha,
    });
    for (const p of data) numbers.add(p.number);
  }
  for (const pull_number of numbers) {
    const { data } = await github.rest.pulls.get({ owner, repo, pull_number });
    if (data.state === 'open' && data.head.sha === run.head_sha) return data;
  }
  return null;
}

async function ensureLabel({ github, owner, repo }) {
  try {
    await github.rest.issues.getLabel({ owner, repo, name: OWNER_REVIEW_LABEL });
  } catch (e) {
    if (e.status !== 404) throw e;
    await github.rest.issues.createLabel({
      owner, repo, name: OWNER_REVIEW_LABEL, color: 'FBCA04',
      description: '自動マージの条件を満たさない PR。オーナーがレビューしてマージする（ADR-0006）',
    });
  }
}

// 同じ PR に既にコメントがあれば更新し、push ごとに増やさない
async function upsertComment({ github, owner, repo, issue_number, body }) {
  const comments = await github.paginate(github.rest.issues.listComments, {
    owner, repo, issue_number, per_page: 100,
  });
  const existing = comments.find((c) => c.body && c.body.includes(COMMENT_MARKER));
  if (existing) {
    await github.rest.issues.updateComment({ owner, repo, comment_id: existing.id, body });
  } else {
    await github.rest.issues.createComment({ owner, repo, issue_number, body });
  }
}

// outputs：pr（空なら後続は何もしない）、sha、action（enable / disable / keep）、description（commit status 用）
async function run({ github, context, core }) {
  const wr = context.payload.workflow_run;
  const { owner, repo } = context.repo;

  if (!JUDGED_CONCLUSIONS.has(wr.conclusion)) {
    core.notice(`CI の結果が ${wr.conclusion} なので判定しない。次の CI の完了で判定する`);
    return;
  }
  const pr = await findPullRequest({ github, owner, repo, run: wr });
  if (!pr) {
    core.notice(`${wr.head_sha} を head に持つ open な PR が無い（head が進んだか、閉じられた）。判定しない`);
    return;
  }

  const files = await github.paginate(github.rest.pulls.listFiles, {
    owner, repo, pull_number: pr.number, per_page: 100,
  });
  const d = decide({ ciConclusion: wr.conclusion, pr, files, repoFullName: `${owner}/${repo}` });

  if (d.ownerReview) {
    await ensureLabel({ github, owner, repo });
    await github.rest.issues.addLabels({ owner, repo, issue_number: pr.number, labels: [OWNER_REVIEW_LABEL] });
    const body = [
      COMMENT_MARKER,
      `### 自動マージしない：\`${OWNER_REVIEW_LABEL}\``,
      '',
      `${wr.head_sha.slice(0, 7)} の判定。次の理由でオーナーのレビューが必要（ADR-0006、AGENTS.md「自動マージしてよい範囲」）。`,
      '',
      ...d.reasons.map((r) => `- ${r}`),
      '',
      'ラベルを外すのはオーナー。外しても再判定は次の push の CI 完了時。',
    ].join('\n');
    await upsertComment({ github, owner, repo, issue_number: pr.number, body });
  }

  const autoMergeEnabled = pr.auto_merge != null;
  let action = 'keep';
  if (d.merge && !autoMergeEnabled) action = 'enable';
  if (!d.merge && autoMergeEnabled) action = 'disable';

  core.setOutput('pr', String(pr.number));
  core.setOutput('sha', wr.head_sha);
  core.setOutput('action', action);
  core.setOutput('description', d.summary);

  await core.summary
    .addHeading(`PR #${pr.number}：${d.summary}`, 3)
    .addRaw(`CI：${wr.conclusion}、auto-merge：${autoMergeEnabled ? '有効' : '無効'} → ${action}`, true)
    .addList(d.reasons)
    .write();
}

module.exports = { run, decide, ownerReviewReasons };
