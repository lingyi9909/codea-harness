#!/bin/zsh
set -eu
if (( $# > 1 )); then
  print -u2 '用法：启动工作台.command [项目目录]'
  exit 2
fi
workspace_args=()
if (( $# == 1 )); then
  project_dir="$1"
  workspace_args=(--workspace "${project_dir:a}")
fi
cd "${0:A:h}"
export PATH="$HOME/.opencode/bin:/opt/homebrew/bin:/usr/local/bin:$PATH"
node_bin="$(command -v node)" || { print -u2 '找不到 Node.js，请安装项目要求的 Node.js 22.16+。'; exit 1; }
port="${HARNESS_HTTP_PORT:-4176}"
if /usr/sbin/lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
  print -u2 "端口 $port 已被使用。如果工作台已启动，请使用原来的窗口。"
  exit 1
fi
print '工作台将在浏览器自动打开。保留此窗口，按 Control+C 可停止工作台。'
(
  for attempt in {1..50}; do
    if /usr/bin/curl -fsS --max-time 1 "http://127.0.0.1:$port/" -o /dev/null 2>/dev/null; then
      /usr/bin/open "http://127.0.0.1:$port/"
      break
    fi
    sleep 0.2
  done
) &
exec "$node_bin" --experimental-strip-types apps/server/src/start.ts --local-only "${workspace_args[@]}"
