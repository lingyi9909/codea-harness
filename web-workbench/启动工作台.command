#!/bin/zsh
set -eu
if (( $# != 1 )); then
  print -u2 '用法：启动工作台.command "/Users/me/project"（必须指定已有工作目录）'
  exit 2
fi
project_dir="$1"
[[ -d "$project_dir" ]] || { print -u2 "工作目录不存在：$project_dir"; exit 2; }
cd "${0:A:h}"
export PATH="$HOME/.opencode/bin:/opt/homebrew/bin:/usr/local/bin:$PATH"
node_bin="$(command -v node)" || { print -u2 '找不到 Node.js，请安装项目要求的 Node.js 22.16+、24 或 26。'; exit 1; }
print '智能工作台启动菜单：选择访问方式、端口和机构执行权限。'
print 'Mac 将在服务就绪后自动打开浏览器。保留此终端窗口，Control+C 停止服务。'
exec "$node_bin" --experimental-strip-types scripts/launch-workbench.mjs "${project_dir:a}"
