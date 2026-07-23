#!/bin/sh

set -eu

script_directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

python_command=""
for candidate in python3.12 python3.11 python3.10; do
	if command -v "$candidate" >/dev/null 2>&1; then
		python_command=$candidate
		break
	fi
done

if [ -z "$python_command" ]; then
	echo "Python 3.10 or newer is required." >&2
	exit 1
fi

if [ ! -x "$script_directory/.venv/bin/python" ]; then
	echo "Creating analysis virtual environment with $python_command..." >&2
	"$python_command" -m venv "$script_directory/.venv"
fi

echo "Installing dashboard dependencies..." >&2
"$script_directory/.venv/bin/python" -m pip install \
	--requirement "$script_directory/requirements.txt"

echo "Analysis environment is ready." >&2
