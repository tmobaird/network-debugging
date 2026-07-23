#!/bin/sh

set -eu

usage() {
	echo "Usage: $0 CONNECTION_LABEL [DURATION] [INTERVAL] [OUTPUT_DIRECTORY]" >&2
	echo "Example: $0 direct-verizon 3h 1m" >&2
}

if [ "$#" -lt 1 ] || [ "$#" -gt 4 ]; then
	usage
	exit 2
fi

connection_label=$1
experiment_duration=${2:-3h}
measurement_interval=${3:-1m}
output_directory=${4:-"$HOME/network-observatory-data"}

case "$connection_label" in
	"" | *[!A-Za-z0-9._-]*)
		echo "Connection label must contain only letters, numbers, periods, underscores, or hyphens." >&2
		exit 2
		;;
esac

for required_command in go swiftc caffeinate mktemp; do
	if ! command -v "$required_command" >/dev/null 2>&1; then
		echo "Required command not found: $required_command" >&2
		exit 1
	fi
done

script_directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/network-observatory.XXXXXX")

cleanup() {
	rm -rf -- "$temporary_directory"
}
trap cleanup EXIT HUP INT TERM

wifi_helper="$temporary_directory/wifi-snapshot"
probe_binary="$temporary_directory/network-probe"

echo "Compiling Wi-Fi snapshot helper..." >&2
swiftc \
	"$script_directory/tools/wifi-snapshot/main.swift" \
	-framework CoreWLAN \
	-o "$wifi_helper"

echo "Compiling network probe..." >&2
(
	cd "$script_directory"
	go build -o "$probe_binary" .
)

mkdir -p "$output_directory"

started_at=$(date -u "+%Y%m%dT%H%M%SZ")
file_prefix="$output_directory/${connection_label}_${started_at}"
data_file="${file_prefix}.jsonl"
error_file="${file_prefix}.errors.log"

echo "Starting experiment:" >&2
echo "  connection label: $connection_label" >&2
echo "  duration:         $experiment_duration" >&2
echo "  interval:         $measurement_interval" >&2
echo "  data:             $data_file" >&2
echo "  errors:           $error_file" >&2

(
	cd "$script_directory"
	caffeinate -i env \
		WIFI_SNAPSHOT_HELPER="$wifi_helper" \
		PROBE_CONNECTION_LABEL="$connection_label" \
		"$probe_binary" \
		--duration="$experiment_duration" \
		--interval="$measurement_interval"
) >>"$data_file" 2>>"$error_file"

echo "Experiment complete." >&2
echo "Data written to: $data_file" >&2
if [ -s "$error_file" ]; then
	echo "Diagnostics written to: $error_file" >&2
else
	rm -f -- "$error_file"
	echo "No diagnostics were recorded." >&2
fi
