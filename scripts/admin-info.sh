#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"
printf '用户名：admin\n初始密码：%s\n' "$ADMIN_PASSWORD"
echo '若已修改密码，请使用修改后的密码。此信息只供本机所有者查看，不要放入公开视频或提交材料。'
