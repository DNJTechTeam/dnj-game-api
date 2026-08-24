#!/usr/bin/env bash
set -euo pipefail

profile="${1:?coverage profile is required}"
minimum="${2:-90}"
slice_minimum="${3:-$minimum}"

awk -v minimum="$minimum" -v slice_minimum="$slice_minimum" '
function location(file_and_range, parts, start) {
  split(file_and_range, parts, ":")
  split(parts[2], start, ".")
  current_file = parts[1]
  current_line = start[1] + 0
}
function in_cross_layer_slice(file, line) {
  return file ~ /admin_installation_service.go$/ ||
    file ~ /admin_installation_handler.go$/ ||
    file ~ /admin_operation_mapper.go$/ ||
    file ~ /admin_operation_repository.go$/ ||
    file ~ /space_repository.go$/ ||
    (file ~ /activity_repository.go$/ && line >= 49) ||
    (file ~ /user_repository.go$/ && line >= 103)
}
NR > 1 {
  location($1)
  key = $1 " " $2
  if ($3 > hits[key]) hits[key] = $3
  statements[key] = $2
  files[key] = current_file
  lines[key] = current_line
}
END {
  for (key in statements) {
    if (files[key] ~ /admin_installation_service.go$/) {
      service_total += statements[key]
      if (hits[key] > 0) service_covered += statements[key]
    }
    if (in_cross_layer_slice(files[key], lines[key])) {
      slice_total += statements[key]
      if (hits[key] > 0) slice_covered += statements[key]
    }
  }
  service_percent = 100 * service_covered / service_total
  slice_percent = 100 * slice_covered / slice_total
  service_rounded = int(service_percent * 10 + 0.5) / 10
  slice_rounded = int(slice_percent * 10 + 0.5) / 10
  printf "Admin service coverage: %.1f%% (%d/%d statements)\n", service_rounded, service_covered, service_total
  printf "Admin cross-layer coverage: %.1f%% (%d/%d statements)\n", slice_rounded, slice_covered, slice_total
  if (service_rounded < minimum || slice_rounded < slice_minimum) exit 1
}
' "$profile"
