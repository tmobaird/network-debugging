#!/bin/sh

set -eu

script_directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
streamlit="$script_directory/.venv/bin/streamlit"

if [ ! -x "$streamlit" ]; then
	echo "Dashboard environment is not installed." >&2
	echo "Run: $script_directory/setup.sh" >&2
	exit 1
fi

exec "$streamlit" run "$script_directory/dashboard.py"
