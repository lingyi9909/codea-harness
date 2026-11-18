#!/bin/sh
set -eu
exec zsh "$(dirname "$0")/启动工作台.command" "$@"
