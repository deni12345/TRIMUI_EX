#!/bin/sh
# Keep the /bin/bash shebang entry point safe without requiring an SD card.
SHELL=${SHELL:-/bin/sh}
export SHELL
exec /usr/local/bin/trimui-ex-bash "$@"
