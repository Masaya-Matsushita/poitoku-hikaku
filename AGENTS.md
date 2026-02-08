## Project Overview

ポイ得比較 (Poitoku-Hikaku) - A Japanese web service for cross-searching and comparing point site (ポイ活サイト) offers. Users search by keyword to find which point site offers the best reward.

**Current Phase**: PoC (Proof of Concept) - Validating Crawl4AI data extraction accuracy.

## Tech Stack

- **Frontend**: Next.js (App Router), TypeScript, Tailwind CSS, shadcn/ui
- **Crawler**: Python with Crawl4AI (AI-powered scraping for HTML change resilience)
- **Database**: Supabase (PostgreSQL)
- **Hosting**: Vercel (ISR with daily revalidation)
- **CI/CD**: GitHub Actions (daily crawling cron at 00:00 JST)

## Development Commands

### Frontend (Next.js)
```bash
npm install
npm run dev
```

### Crawler (Python)
```bash
cd crawler
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python main.py
```

### PoC Testing
```bash
cd crawler
source venv/bin/activate
python poc/test_moppy.py      # Test Moppy extraction
python poc/test_hapitas.py    # Test Hapitas extraction
```

## Project Structure

```
poitoku-hikaku/
├── src/                    # Next.js frontend (planned)
│   ├── app/               # App Router pages
│   ├── components/        # UI components (SearchForm, OfferTable)
│   └── lib/               # Supabase client, utilities
├── crawler/               # Python crawler
│   ├── main.py           # Entry point
│   ├── sites/            # Site-specific crawlers (moppy.py, hapitas.py)
│   └── schemas/          # Pydantic schemas for LLM extraction
├── .github/workflows/    # GitHub Actions (crawl.yml)
└── docs/                 # Documentation
```

## Key Architecture Decisions

### Crawl4AI for Data Extraction
The project uses Crawl4AI (LLM-based extraction) instead of traditional CSS selectors to minimize maintenance when target sites change their HTML structure. Fallback to CSS/XPath extraction is available.

### Data Model
The `offers` table stores: `site_name`, `offer_name`, `reward` (in yen), `original_reward`, `url`, `category`, `fetched_at`. Data is deduplicated by `(site_name, url, fetched_at::DATE)`.

### Crawling Strategy
- **Moppy**: Parse search result pages directly (`/search/?word=xxx`)
- **Hapitas**: Crawl individual offer pages (`/item/detail/itemid/xxx/`) since search results require JS execution

## Environment Variables

```env
SUPABASE_URL=<supabase_url>
SUPABASE_ANON_KEY=<supabase_anon_key>
OPENAI_API_KEY=<openai_api_key>  # For Crawl4AI LLM extraction
```

## Constraints & Guidelines

- **Minimize maintenance**: This is the top priority. Prefer solutions that are resilient to HTML changes.
- **Crawling etiquette**: 3-second delay between requests, respect robots.txt, run once daily.
- **Point conversion**: Both Moppy and Hapitas use 1pt = 1 yen. Other sites have different rates (see `docs/point-sites.md`).
- **Error handling**: On crawl failure, retain previous data and display staleness indicator to users.
