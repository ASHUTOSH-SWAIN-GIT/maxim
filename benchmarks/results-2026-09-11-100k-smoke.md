# 100,000-row benchmark smoke run — 2026-09-11

This was a one-iteration harness validation, not a performance claim.

- Host: macOS, Apple M4, arm64, 10 logical CPUs
- Go: 1.26.5
- PostgreSQL: 17 Alpine disposable container
- Fixture: 100,000 rows
- Index: `(rank, tenant_id, id)` plus the compound primary key
- Benchmark duration: `1x` per case

| Case | Time | Allocated bytes | Allocations |
| --- | ---: | ---: | ---: |
| Indexed keyset, first page | 50.95 ms | 36,088,296 | 2,056 |
| Indexed keyset, deep page | 6.25 ms | 294,776 | 2,758 |
| Nullable offset, 90% depth | 47.67 ms | 267,992 | 2,442 |
| Keyset during concurrent inserts | 50.92 ms | 36,147,216 | 2,840 |

The deep keyset plan used `Index Scan` on `(rank, tenant_id, id)`. PostgreSQL
reported 0.097 ms planning time and 0.105 ms execution time for 101 rows, with
101 shared-buffer hits and four reads. The difference between server execution
and end-to-end benchmark time includes metadata discovery, transport, scanning,
typed value creation, preview limiting, and allocations.

The first-page cases intentionally encounter periodic 120 KB text values. Their
large allocation count shows that retained-page limits do not eliminate driver
and transport allocations while a value is being received. Treat this as a
follow-up optimization target, not as proof that memory is fully bounded at the
wire level.

Run the default multi-iteration benchmark before comparing releases. The
1,000,000-row target was not executed during this smoke validation.
