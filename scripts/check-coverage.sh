#!/usr/bin/env sh

set -eu

coverage_file=${1:-coverage.out}
minimum_coverage=${2:-15}

total_coverage=$(go tool cover -func="$coverage_file" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')
if [ -z "$total_coverage" ]; then
  echo "Could not read total coverage from $coverage_file."
  exit 1
fi

awk -v actual="$total_coverage" -v minimum="$minimum_coverage" 'BEGIN {
  if (actual + 0 < minimum + 0) {
    printf "Coverage %.1f%% is below the required %.1f%%.\n", actual, minimum
    exit 1
  }
  printf "Coverage %.1f%% meets the required %.1f%%.\n", actual, minimum
}'
