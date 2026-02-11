# ポイ得比較（Poitoku-Hikaku）

ポイントサイトの案件を横断検索・比較できるWebサービス

## ドキュメント

- [SPEC.md](./SPEC.md) - 仕様書（Single Source of Truth）
- [AGENTS.md](./AGENTS.md) - AI Agent向けルール
- [docs/setup.md](./docs/setup.md) - 環境構築手順

## ディレクトリ構成

```
poitoku-hikaku/
├── SPEC.md                 # 仕様書（技術構成、DB設計、要件）
├── AGENTS.md               # AI Agent向けルール
├── docs/
│   ├── setup.md            # 環境構築手順
│   └── reference/          # 参照資料
│       ├── competitor.md   # 競合分析
│       └── point-sites.md  # ポイントサイト詳細
├── ai/
│   ├── tasks/              # 依頼済みタスク
│   └── backlog/            # 未依頼のアイデア
├── logs/
│   └── meetings/           # 会議記録
├── crawler/                # Python クローラー（予定）
└── src/                    # Next.js フロントエンド（予定）
```

## ライセンス

MIT License
