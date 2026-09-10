# Maxim product and implementation roadmap

Updated: 2026-09-10

This is the standing implementation plan for Maxim. Work through the phases in order; update the checklist and current-work section as changes land. A checked item means implemented and verified, not merely discussed. This document proposes future work; it does not claim those features already exist.

## Product direction

**Make Maxim a dependable PostgreSQL workspace for developers who want to inspect, query, and troubleshoot their database without leaving the terminal.**

The core workflow is: connect → find a table → filter records → inspect a row → run a query → export or reuse the result. Every release should make that workflow more useful or reliable.

Primary users are backend developers working on local and staging databases, developers accessing remote databases through terminal sessions, and maintainers investigating application data. Production access becomes a supported workflow only after the connection, cancellation, and read-only protections below are verified.

Success means people return to Maxim for their own database work. Installation counts and feature counts are secondary to successful, repeated use.

## Product decisions already made

- Keep the persistent workspace, compact header, data grid, and temporary table navigator.
- Show saved connections during connection selection, not as a permanent workspace column.
- Keep the accepted simple vertical row peek. Improve scrolling, wrapping, and value accuracy within it.
- Do not reintroduce the rejected field-list/preview inspector or the three-column dashboard without a new user decision.
- Keep keyboard actions discoverable through contextual help. Ordinary typing in an editor or form must not activate global commands.
- Focus on PostgreSQL first. Preserve existing database-management and Docker features, but prioritize everyday browsing and querying.
- Passwords stay out of configuration files. Do not add telemetry or query-history collection silently.

## Current baseline and limitations

Based on repository inspection, not a new test or performance run:

| Area | Available today | Work still needed |
| --- | --- | --- |
| Connections | Named profiles, host/port/database/SSL mode, password prompt | Connection lifecycle, read-only profiles, certificate configuration, recovery |
| Workspace | Table navigator, Data/Structure, simple row peek, SQL editor shortcut | Responsive layout, consistent view navigation, request isolation, help |
| Browsing | Bounded pages, equality filter, sort cycling | More filter types, predictable errors, wider-table navigation |
| Pagination | Single-primary-key keyset path; offset fallback | Typed cursors, compound keys, stable tie-breaking, concurrency semantics |
| Timeout | Shared deadlines for connection, metadata, browsing, and SQL; cancellation in workspace/editor | Reconnect behavior and cancellation coverage for future operations |
| SQL | Async execution, cancellation, autocomplete, first 100 result rows displayed | Retained drafts, reliable SQL parsing, clear truncation/affected-row reporting |
| Structure | Column type, nullability, default, primary-key indicator for public tables | Multiple schemas, foreign keys, indexes, constraints, comments |
| Delivery | Unit/race tests, PostgreSQL and Docker integration, platform CI, release workflow | Edge-case fixtures, interaction tests, benchmarks, release smoke tests |

Known code issues to address first:

- `internal/tui/workspace.go` accepts table responses without request identities. Rapid navigation can let an old response replace newer state; cursor history is changed before success.
- `internal/db/browser.go` derives the cursor from a displayed string. Float formatting and timestamp formatting must never determine database cursor identity.
- Offset fallback does not guarantee a unique order for tables without a single-column primary key. It can also shift under concurrent writes.
- Metadata lookup assumes `public`, but data queries use an unqualified table name. Schema resolution must be consistent.
- `internal/tui/sqlEditor.go` has a statement splitter that understands only basic single-quoted strings.
- `internal/db/query_executor.go` stops displaying results at 100 rows, but this is not a database-work limit or a cancellation mechanism.
- Values are flattened to strings, including the same visible representation for SQL NULL and text containing `NULL`.
- The reverted inspector still has unused helper functions in `workspace.go`. Remove that dead code while retaining the accepted row peek.

## Execution order and milestones

| Milestone | Phases | Release gate |
| --- | --- | --- |
| Reliable browsing foundation | 0–1 | Navigation, values, and pagination remain correct under failures and large fixtures |
| Useful everyday preview | 2–4 | A developer can browse, inspect, query, and export their own database |
| Remote-use beta | 5–6 | Safe connection behavior, diagnosis tools, and repeat usage from external testers |
| Optional data editing | 7 | Exact row identity, transaction behavior, and conflict handling verified |
| Stable release | 8 | Supported platforms, documentation, packaging, and core workflows pass release gates |

These are completion gates, not calendar promises or predetermined version numbers. Deliver small reviewable changes within each phase. Bug fixes and user feedback can move ahead of new features.

## Phase 0 — Stabilize the accepted workspace

Goal: predictable interaction before adding more controls.

- [x] **M0.1 Request lifecycle:** attach IDs and cancellation to asynchronous loads; ignore stale responses. Handle results even when an editor or input prompt is open.
- [x] **M0.2 Navigation state:** commit the selected table, filter, sort, page, and cursor history only after a successful load. Preserve the previous successful view on failure; support retry.
- [x] **M0.3 Context and deadlines:** pass a caller context through connection/metadata/data work; cancel pending work on disconnect or replacement. Keep the event loop responsive.
- [x] **M0.4 Terminal sizing:** account for the complete header/toolbar/footer height; handle resize in every mode. Support an 80×24 terminal and provide an explicit message for unsupported sizes.
- [x] **M0.5 Row peek:** preserve its existing layout; wrap long and multiline values, support independent scrolling, and return to the same selected row and grid position.
- [ ] **M0.6 State cleanup:** remove unused inspector helpers, redundant metadata loads, and obsolete model fields once callers are verified. Retain functioning legacy paths until replacement tests pass.
- [ ] **M0.7 Help:** add `?` with commands for the active view. Distinguish loading, cancellation, empty data, permission denial, and disconnected states.

Done when: delayed responses, failed next-page loads, rapid repeated keys, switching views during loading, and terminal resizing cannot show the wrong table or lose the last successful page. No intended database call blocks the workspace event loop.

## Phase 1 — Correct and scalable data access

Depends on Phase 0 request/state handling.

- [ ] **M1.1 Typed values:** separate raw values, null flags, metadata, and display text. Preserve numeric precision and timestamp identity; escape terminal control characters and truncate by display width safely.
- [ ] **M1.2 Qualified identifiers:** represent schema and table separately and quote both. Validate user-selected columns against that exact table; bind filter values as parameters.
- [ ] **M1.3 Cursor correctness:** use lossless cursor values and distinguish an absent cursor from an empty string. Bind cursors to the current connection, table, sort, and filter.
- [ ] **M1.4 Stable ordering:** support composite primary keys; add unique tie-breakers for custom sorts. Define NULL placement and both traversal directions explicitly. Use keyset only where the ordering and types support it.
- [ ] **M1.5 Honest fallback:** show when offset pagination is used and explain its limitations. Never promise stable row positions for changing data or a unique order where no usable key exists.
- [ ] **M1.6 Resource limits:** enforce page-size limits and configurable deadlines. Define byte limits and lazy/full-value retrieval for oversized cells; a 100-row limit alone does not bound memory.
- [ ] **M1.7 Large fixtures:** add opt-in benchmarks with 100,000 and 1,000,000 rows, duplicate sort values, compound keys, nulls, long text, JSON, Unicode, and concurrent writes.

Done when: forward/backward traversal on a fixed dataset has no skipped or duplicated records across supported orderings; raw values survive rendering; results stay bounded; concurrent-change behavior is documented. Record benchmark hardware, indexes, query plans, latency, and memory rather than advertising unmeasured speed guarantees.

## Phase 2 — Complete the table-browsing workflow

- [ ] **M2.1 Searchable navigator:** find schemas/tables by name; include views and clearly distinguish them. Add manual metadata refresh.
- [ ] **M2.2 Filter builder:** retain quick equality entry, then add typed operators (`=`, `!=`, comparisons, contains, `IS NULL`, `IS NOT NULL`) and AND conditions. Support empty strings distinctly from NULL; expose apply/reset and readable validation.
- [ ] **M2.3 Sort selection:** provide a searchable column/direction picker so users do not have to cycle through 80 columns. Keep shortcuts as accelerators.
- [ ] **M2.4 Wide tables:** add horizontal scrolling, pinned identifiers where practical, and selectable visible columns. Preserve full values in row peek.
- [ ] **M2.5 Browse state:** retain table/filter/sort/selection within the current session, reset safely when metadata changes, and expose explicit refresh.
- [ ] **M2.6 Status:** show fetched rows, elapsed time, and whether another page exists. Offer estimated table counts labelled as estimates; exact counts run only on request and remain cancelable.

Done when: a developer can find one record in a wide table using filters, inspect its full values, and return to the same browse state without writing SQL or memorizing hidden shortcuts.

## Phase 3 — Make SQL editing dependable

Depends on context handling and typed result models from Phases 0–1.

- [ ] **M3.1 Real Query view:** make Query reachable through consistent view navigation as well as `e`; preserve draft text and results when leaving and returning.
- [ ] **M3.2 Async execution:** show running status and duration; provide a dedicated cancel action and execution timeout. Show cancellation separately from success or query failure.
- [ ] **M3.3 SQL boundaries:** support PostgreSQL comments, quoted identifiers, dollar-quoted bodies, and embedded semicolons using a suitable parser/tokenizer. Test current-statement, selected-text, and explicit run-all semantics.
- [ ] **M3.4 Result handling:** distinguish rowsets, affected rows, notices, and errors; expose truncation accurately. Keep database execution separate from terminal formatting.
- [ ] **M3.5 Transaction semantics:** define behavior after an error in a batch; never silently partially commit a user-requested atomic batch. Display active transaction state and rollback recovery.
- [ ] **M3.6 Better autocomplete:** use schema/table context and the actual caret position; refresh metadata after relevant schema changes. Keep schema loading out of the input event loop.
- [ ] **M3.7 Reuse:** session history first, named SQL snippets and SQL-file open/save next. Persistent history must be opt-in, bounded, removable, and clearly disclose that SQL may contain sensitive literals.

Done when: users can write, execute, cancel, revise, and rerun realistic PostgreSQL without losing their draft or freezing the UI. A truncated result must never be presented as the total matching row count.

## Phase 4 — Export results and repeat useful work

- [ ] **M4.1 CSV/JSON export:** export the current page first, then explicitly requested complete filtered results. Preserve escaping, Unicode, NULL semantics, and numeric values through tests.
- [ ] **M4.2 Streaming export:** use bounded batches/streaming, progress, and cancellation. Define snapshot consistency; distinguish a partially written file from a completed export and handle overwrites explicitly.
- [ ] **M4.3 Saved workflows:** save named filters/sorts and SQL snippets, without persisting database results by default.
- [ ] **M4.4 Scriptable usage:** add query/file execution and export commands with documented stdout/stderr behavior, exit codes, noninteractive authentication, and explicit write policy. Reuse the same tested execution layer.

Done when: the user can investigate a record, run a related query, and export an accurate result for a bug report, then reproduce that investigation later. Complete exports do not silently inherit the preview row cap.

## Phase 5 — Connections suitable for remote work

- [ ] **M5.1 Read-only profiles:** visibly identify restricted connections and enforce a database-supported read-only execution policy. Recommend a restricted PostgreSQL role; SQL keyword checks alone are not enforcement.
- [ ] **M5.2 Connection options:** support verified TLS and certificate paths, clear SSL errors, and reasonable connect deadlines. Never silently downgrade TLS after failure.
- [ ] **M5.3 Recovery:** detect lost connections; provide reconnect with preserved drafts and explicit transaction-loss messaging. Do not automatically retry writes.
- [ ] **M5.4 Credentials:** support established PostgreSQL credential mechanisms where appropriate; optional OS credential storage only by explicit user choice. Redact secrets in errors and diagnostic output.
- [ ] **M5.5 Connection shortcuts:** documented named-profile launch, connection URI import without accidental secret persistence, and export/import of profiles excluding credentials.
- [ ] **M5.6 SSH workflow:** document use with an external SSH tunnel first. Build an integrated tunnel only after repeated user demand and connection cleanup tests.

Done when: a tester can connect to a remote PostgreSQL instance using a restricted role and verified TLS, recover from a dropped connection, and understand the active access mode.

## Phase 6 — Understand schemas and diagnose queries

- [ ] **M6.1 Schema details:** foreign keys, indexes, uniqueness, check constraints, comments, and generated/identity columns. Display schema-qualified names.
- [ ] **M6.2 Follow relationships:** jump from a row's foreign key to the referenced filtered table; support compound and null keys and an obvious return path.
- [ ] **M6.3 Query plans:** expose plain EXPLAIN first with a readable plan view. EXPLAIN ANALYZE must be a separate explicit action because it executes the statement.
- [ ] **M6.4 Useful statistics:** table/index sizes and estimated row counts, with permission-aware errors and refresh. Label estimates and collection freshness.
- [ ] **M6.5 Diagnosis guidance:** explain observed scans or expensive plan nodes without claiming every sequential scan requires an index. Never automatically modify indexes.

Done when: a developer can understand a table's relationships and inspect why a query is expensive while staying in Maxim.

## Phase 7 — Optional controlled data editing

Start after the browsing/query/export preview has external users and Phases 1, 3, and 5 are verified. This phase can be deferred if users primarily need inspection.

- [ ] **M7.1 Single-record updates:** explicit edit mode, typed inputs, original/new value preview, parameterized statements, and exact primary-key identity. Disable edits where row identity cannot be established.
- [ ] **M7.2 Conflicts:** detect concurrent changes and require refresh/review instead of silently overwriting them. Verify the affected-row count.
- [ ] **M7.3 Insert/delete:** required-field and default handling, explicit confirmation for deletion, and clear database validation errors.
- [ ] **M7.4 Transactions:** deliberate commit/rollback, failure recovery, and no misleading promise that committed changes can always be undone.
- [ ] **M7.5 Access policy:** read-only connections cannot enter editing mode or execute these writes. No bulk editing in the first iteration.

Done when: edits target exactly the intended record, conflicts are visible, rollback behavior is tested, and connection loss cannot produce a false success message.

## Phase 8 — Ship, observe, and maintain

Begin documentation and tester recruitment during Phase 2; use this checklist as the final release gate.

- [ ] **M8.1 Onboarding:** accurate prerequisites and installation docs, quick-start fixture, shortcut guide, and a short recording of connect → filter → peek → query → export.
- [ ] **M8.2 Packaging:** verify existing release artifacts on supported OS/architectures; publish checksums and installation/uninstall instructions. Expand distribution channels only when maintainable.
- [ ] **M8.3 Compatibility:** document tested PostgreSQL versions, terminals, minimum dimensions, TLS/auth behavior, and platform limitations. Add CI coverage for the supported matrix.
- [ ] **M8.4 Test strategy:** retain unit tests near Go packages where they need internal access; organize shared fixtures, integration scenarios, and test instructions clearly. Validate user behavior and correctness boundaries rather than increasing coverage with shallow assertions.
- [ ] **M8.5 Release discipline:** changelog, migration notes for config changes, backup/recovery tests, clean-checkout CI, and smoke tests of the actual packaged binaries.
- [ ] **M8.6 Feedback:** recruit 5–10 PostgreSQL developers. Ask each to connect their own database, find a record, inspect it, run a query, and export results. Record confusion and failures with consent, without collecting their data.
- [ ] **M8.7 Maintenance:** issue/bug templates, contributor setup, dependency updates, and redacted diagnostics. Prioritize recurring failures before optional features.

Done when: testers complete the core workflow without live assistance, several choose to use Maxim again, release artifacts work on the documented platforms, and no known data-correctness or critical workflow blocker remains.

## Engineering boundaries

- Keep database execution, typed results, pagination, and export independent of Bubble Tea/lipgloss. Adapt existing code incrementally rather than undertaking a full rewrite.
- Keep workspace navigation, query execution, and prompts in explicit states with a shared request lifecycle. Only successful responses advance persistent browsing state.
- Reuse one identifier-validation and parameter-binding path. Never turn quick-filter text directly into raw SQL.
- Validate correctness with fixtures covering quoted names, multiple schemas, compound keys, NULL vs text `NULL`, empty strings, high-precision numbers, time zones, long Unicode text, and failures during navigation.
- Test long-running operations against disposable PostgreSQL, including cancellation and connection loss. Unit tests alone cannot prove driver behavior.
- Run checks proportionate to each change. Database changes require integration tests; layout changes require terminal-size checks; release changes require artifact validation.
- Benchmark keyset vs offset with documented indexed queries. Proposed target: cancellation feedback within 100 ms in the UI and bounded memory as total row count grows; measure database cancellation separately. Do not use network-dependent page latency as an unconditional CI threshold.
- Existing CI already checks formatting, vet, race tests, platform tests, vulnerabilities, PostgreSQL/Docker integration, and release configuration. Extend it for missing behavior; do not rebuild it from scratch.

## Explicitly deferred

Do not start these unless recurring user feedback justifies changing the roadmap:

- MySQL/SQLite and other database engines before PostgreSQL workflows are reliable.
- AI-generated SQL, hosted collaboration, accounts, cloud sync, or a plugin marketplace.
- A web/desktop frontend, dashboard redesign, or another row-inspector redesign.
- Migration orchestration, automated schema changes, background monitoring, or replacing full database administration tools.
- Graphical relationship diagrams, custom theme galleries, integrated SSH infrastructure, and bulk data editing before the core release gates.

## How to use this plan on every implementation task

1. Read Current work below and select the first unchecked item whose dependencies are complete.
2. State the item ID and the user-visible outcome. Fix any discovered blocker in that scope before moving on.
3. Deliver a small complete change with meaningful checks and updated user documentation.
4. Mark the item complete only when its behavior and relevant phase criteria have been verified. Record limitations and follow-ups explicitly.
5. Update Current work so the next session can continue without another “what next?” discussion.
6. Review priorities after each milestone using actual usage feedback. Changing the accepted UI or adding a deferred product direction requires an explicit decision.

## Current work

- **Status:** M0.1–M0.5 complete; Phase 0 is in progress.
- **Next item:** M0.6 — remove rejected inspector code, redundant metadata work, and obsolete model state without disturbing the accepted workspace.
- **Following item:** M0.7 contextual help and distinct operational states.
- **First milestone:** reliable browsing foundation (Phases 0–1).
- **Planning-only change:** this document does not implement the features above or authorize external releases, telemetry, or database writes.

### Delivery log

| Date | Item | Outcome and verification |
| --- | --- | --- |
| 2026-09-10 | Roadmap | Reviewed current code and captured the agreed UI, known gaps, implementation sequence, and release gates. No runtime changes. |
| 2026-09-10 | M0.1 | Added request IDs and cancellation contexts for table discovery and browsing, ignored stale/duplicate responses, handled results during editor/filter states, and verified cancellation with race and PostgreSQL integration tests. |
| 2026-09-10 | M0.2 | Staged table, filter, sort, page, and cursor changes until successful responses; retained the last successful data after errors; added retry with `r`; verified failure and retry state with unit, race, and PostgreSQL integration tests. |
| 2026-09-10 | M0.3 | Added shared connection, metadata, browse, and query deadlines; moved SQL execution and autocomplete loading off the event loop; added query replacement/stale-result isolation and `Ctrl+X` cancellation; verified with unit, race, vet, and disposable PostgreSQL integration tests. |
| 2026-09-10 | M0.4 | Added a 60×18 minimum-size state, verified the full workspace at 80×24, bounded long lines and SQL panels, kept selected tables visible in long navigators, and resized data, structure, filter, row-peek, loading/error, and Query modes without losing state. |
| 2026-09-10 | M0.5 | Kept the simple vertical row peek, wrapped long and multiline Unicode values, added independent line/page scrolling, retained peek scroll through resize, and restored the exact selected row and grid viewport on exit. |
