#!/bin/sh
set -eu
if [ "$#" -ne 1 ]; then
  echo '用法：sh 启动机构报文排查.command "/你的唯一工作目录"'
  exit 2
fi
EMBED_WORKSPACE="$1"
cd "$(dirname "$0")"
if [ ! -f apps/web/dist/index.html ]; then
  echo '请先执行 npm run web:build'
  exit 2
fi
exec node --experimental-strip-types apps/server/src/start.ts --embedded-only --workspace "$EMBED_WORKSPACE"
