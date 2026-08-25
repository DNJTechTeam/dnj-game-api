#!/usr/bin/env bash
set -euo pipefail

profile="${1:?coverage profile is required}"
service_minimum="${2:-90}"
slice_minimum="${3:-90}"

awk -v service_minimum="$service_minimum" -v slice_minimum="$slice_minimum" '
function location(file_and_range, parts) {
  split(file_and_range, parts, ":")
  current_file = parts[1]
}
function in_service(file) {
  return file ~ /\/(media_service|moment_service|moment_interaction_service|image_sanitizer|media_moment_helpers)\.go$/
}
function in_slice(file) {
  return in_service(file) ||
    file ~ /\/(media_handler|moment_handler|media_moment_mapper|media_repository|moment_repository|idempotency_registry)\.go$/
}
NR > 1 {
  location($1)
  key = $1 " " $2
  if ($3 > hits[key]) hits[key] = $3
  statements[key] = $2
  files[key] = current_file
}
END {
  for (key in statements) {
    if (in_service(files[key])) {
      service_total += statements[key]
      if (hits[key] > 0) service_covered += statements[key]
    }
    if (in_slice(files[key])) {
      slice_total += statements[key]
      if (hits[key] > 0) slice_covered += statements[key]
    }
  }
  service_percent = 100 * service_covered / service_total
  slice_percent = 100 * slice_covered / slice_total
  service_rounded = int(service_percent * 10 + 0.5) / 10
  slice_rounded = int(slice_percent * 10 + 0.5) / 10
  printf "Iteration 7 service coverage: %.1f%% (%d/%d statements)\n", service_rounded, service_covered, service_total
  printf "Iteration 7 integrated coverage: %.1f%% (%d/%d statements)\n", slice_rounded, slice_covered, slice_total
  if (service_rounded < service_minimum || slice_rounded < slice_minimum) exit 1
}
' "$profile"
