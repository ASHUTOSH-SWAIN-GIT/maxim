#!/usr/bin/env sh

set -eu

required_variables="MAXIM_TEST_DB_HOST MAXIM_TEST_DB_PORT MAXIM_TEST_DB_USER MAXIM_TEST_DB_PASSWORD MAXIM_TEST_DB_NAME"
configured_count=0
for variable_name in $required_variables; do
  eval "variable_value=\${$variable_name-}"
  if [ -n "$variable_value" ]; then
    configured_count=$((configured_count + 1))
  fi
done

if [ "$configured_count" -ne 0 ] && [ "$configured_count" -ne 5 ]; then
  echo "Either set all MAXIM_TEST_DB_* variables or leave all of them unset."
  exit 1
fi

test_container_id=""
cleanup_test_database() {
  if [ -n "$test_container_id" ]; then
    docker rm -f "$test_container_id" >/dev/null 2>&1 || true
  fi
}
trap cleanup_test_database EXIT INT TERM

if [ "$configured_count" -eq 0 ]; then
  if ! docker version >/dev/null 2>&1; then
    echo "Docker is required when MAXIM_TEST_DB_* variables are not provided."
    exit 1
  fi

  echo "Starting a disposable PostgreSQL 17 test database..."
  test_container_id=$(docker run -d --rm \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_PASSWORD=postgres \
    -e POSTGRES_DB=maxim_test \
    -p 127.0.0.1::5432 \
    postgres:17-alpine)

  ready=false
  attempt=1
  while [ "$attempt" -le 30 ]; do
    if docker exec "$test_container_id" pg_isready -U postgres -d maxim_test >/dev/null 2>&1; then
      ready=true
      break
    fi
    sleep 1
    attempt=$((attempt + 1))
  done
  if [ "$ready" != true ]; then
    echo "PostgreSQL did not become ready in time."
    docker logs "$test_container_id" || true
    exit 1
  fi

  MAXIM_TEST_DB_HOST=localhost
  MAXIM_TEST_DB_PORT=$(docker port "$test_container_id" 5432/tcp | sed 's/.*://')
  MAXIM_TEST_DB_USER=postgres
  MAXIM_TEST_DB_PASSWORD=postgres
  MAXIM_TEST_DB_NAME=maxim_test
  export MAXIM_TEST_DB_HOST MAXIM_TEST_DB_PORT MAXIM_TEST_DB_USER MAXIM_TEST_DB_PASSWORD MAXIM_TEST_DB_NAME
fi

if [ -n "${MAXIM_BENCH_ROWS-}" ]; then
  benchmark_time=${MAXIM_BENCH_TIME-3s}
  echo "Running opt-in database benchmark with $MAXIM_BENCH_ROWS rows for $benchmark_time per case..."
  go test -v -tags=integration -run '^$' -bench '^BenchmarkBrowseLargeFixture$' -benchmem -benchtime "$benchmark_time" -count=1 ./internal/db/...
else
  go test -tags=integration -count=1 -v ./cmd/... ./internal/db/...
fi
