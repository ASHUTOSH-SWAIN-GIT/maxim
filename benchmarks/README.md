# Database browsing benchmarks

These benchmarks are opt-in and never run in normal CI because they create and
populate a disposable PostgreSQL table with either 100,000 or 1,000,000 rows.

Run:

```sh
make benchmark-db-100k
make benchmark-db-1m
```

Set `MAXIM_BENCH_TIME` to change the default three-second duration per case.
Set all `MAXIM_TEST_DB_*` variables to use an existing disposable PostgreSQL
instance; otherwise the runner starts and removes a PostgreSQL 17 container.

The fixture includes duplicate sort values, a compound primary key, nullable
text, JSON, Unicode, periodic large text, and an indexed custom ordering. The
output includes operating system, architecture, CPU count, Go version, fixture
size, `EXPLAIN (ANALYZE, BUFFERS)` output, nanoseconds per operation, bytes per
operation, and allocations per operation. Cases compare first-page and deep
keyset access, deep nullable/offset access, and keyset reads during inserts.

Save results when evaluating a release or regression:

```sh
make benchmark-db-100k | tee benchmarks/results-100k.txt
```

Record the PostgreSQL version and host hardware alongside any committed result.
Do not publish a speed claim from a single machine or without comparing query
plans and indexes.
