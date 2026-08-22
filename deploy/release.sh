#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DIST_DIR="${PROJECT_ROOT}/dist"

DEPLOY_TARGET="${1:-${DEPLOY_TARGET:-}}"
DEPLOY_SSH_PORT="${DEPLOY_SSH_PORT:-22}"
PUBLIC_BASE_URL="${PUBLIC_BASE_URL:-}"
SKIP_TESTS="${SKIP_TESTS:-0}"
RELEASE_ID="$(date +%Y%m%d%H%M%S)"
REMOTE_STAGE="/tmp/accounter-release-${RELEASE_ID}"

if [[ -z "${DEPLOY_TARGET}" ]]; then
  echo "用法: $0 <SSH主机>"
  echo "示例: PUBLIC_BASE_URL=https://www.qingzhang.top $0 qingzhang-ecs"
  exit 2
fi

for command_name in go pnpm ssh scp tar; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    echo "缺少本地命令: ${command_name}"
    exit 2
  fi
done

SSH_ARGS=(-p "${DEPLOY_SSH_PORT}")
SCP_ARGS=(-P "${DEPLOY_SSH_PORT}")

echo "==> 检查 ECS 架构"
REMOTE_MACHINE="$(ssh "${SSH_ARGS[@]}" "${DEPLOY_TARGET}" uname -m)"
case "${REMOTE_MACHINE}" in
  x86_64)
    GO_ARCH="amd64"
    ;;
  aarch64|arm64)
    GO_ARCH="arm64"
    ;;
  *)
    echo "不支持的 ECS 架构: ${REMOTE_MACHINE}"
    exit 1
    ;;
esac
echo "ECS 架构: ${REMOTE_MACHINE}，Go 构建目标: linux/${GO_ARCH}"

echo "==> 检查 ECS 生产配置"
ssh "${SSH_ARGS[@]}" "${DEPLOY_TARGET}" bash -s <<'REMOTE_PREFLIGHT'
set -Eeuo pipefail

if [[ "$(id -u)" -eq 0 ]]; then
  SUDO=()
else
  SUDO=(sudo)
fi

if [[ ! -f /etc/accounter/env ]]; then
  echo "缺少 /etc/accounter/env，停止发布"
  exit 1
fi
if ! "${SUDO[@]}" grep -Eq '^AI_ACTIVATION_SECRET=.{32,}$' /etc/accounter/env; then
  echo "缺少有效的 AI_ACTIVATION_SECRET（至少 32 位），请先执行 openssl rand -hex 32 并写入 /etc/accounter/env"
  exit 1
fi
if "${SUDO[@]}" grep -Eq '^AI_ACTIVATION_SECRET=(CHANGE_|dev-activation-secret)' /etc/accounter/env; then
  echo "AI_ACTIVATION_SECRET 仍是占位值，停止发布"
  exit 1
fi
REMOTE_PREFLIGHT

mkdir -p "${DIST_DIR}"

if [[ "${SKIP_TESTS}" != "1" ]]; then
  echo "==> 运行后端测试"
  (
    cd "${PROJECT_ROOT}/backend"
    go test ./...
    go vet ./...
  )
fi

echo "==> 构建 Linux 后端"
(
  cd "${PROJECT_ROOT}/backend"
  CGO_ENABLED=0 GOOS=linux GOARCH="${GO_ARCH}" \
    go build -trimpath -ldflags="-s -w" \
    -o "${DIST_DIR}/accounter" ./cmd/server
  CGO_ENABLED=0 GOOS=linux GOARCH="${GO_ARCH}" \
    go build -trimpath -ldflags="-s -w" \
    -o "${DIST_DIR}/accounter-admin" ./cmd/admin
)

echo "==> 安装前端依赖并构建 H5"
(
  cd "${PROJECT_ROOT}/frontend"
  pnpm install --frozen-lockfile
  pnpm build:h5
)

H5_ARCHIVE="${DIST_DIR}/accounter-h5-${RELEASE_ID}.tar.gz"
# macOS 的 bsdtar 默认会把 com.apple.* 扩展属性写进 PAX header。
# bsdtar 和 GNU tar 都支持 --no-xattrs，但 bsdtar 3.5 的精简帮助不会列出该参数，不能靠 help 探测。
COPYFILE_DISABLE=1 tar --no-xattrs -czf "${H5_ARCHIVE}" \
  -C "${PROJECT_ROOT}/frontend/dist/build/h5" .

echo "==> 创建 ECS 临时发布目录"
ssh "${SSH_ARGS[@]}" "${DEPLOY_TARGET}" install -d -m 700 "${REMOTE_STAGE}"

echo "==> 上传发布文件"
scp "${SCP_ARGS[@]}" \
  "${DIST_DIR}/accounter" \
  "${DIST_DIR}/accounter-admin" \
  "${H5_ARCHIVE}" \
  "${SCRIPT_DIR}/accounter.service" \
  "${DEPLOY_TARGET}:${REMOTE_STAGE}/"

echo "==> 替换 ECS 应用并执行健康检查"
ssh "${SSH_ARGS[@]}" "${DEPLOY_TARGET}" bash -s -- \
  "${REMOTE_STAGE}" "${RELEASE_ID}" "$(basename "${H5_ARCHIVE}")" <<'REMOTE_SCRIPT'
set -Eeuo pipefail

REMOTE_STAGE="$1"
RELEASE_ID="$2"
H5_ARCHIVE_NAME="$3"
BACKUP_DIR="/opt/accounter/backups/${RELEASE_ID}"
NEW_WEB_DIR="/opt/accounter/web.new-${RELEASE_ID}"
CURRENT_WEB_DIR="/opt/accounter/web"
CURRENT_BINARY="/opt/accounter/bin/accounter"

if [[ "$(id -u)" -eq 0 ]]; then
  SUDO=()
else
  SUDO=(sudo)
fi

if [[ ! -f /etc/accounter/env ]]; then
  echo "缺少 /etc/accounter/env，停止发布"
  exit 1
fi

"${SUDO[@]}" install -d -o accounter -g accounter -m 755 /opt/accounter/bin
"${SUDO[@]}" install -d -o accounter -g accounter -m 755 /opt/accounter/backups
"${SUDO[@]}" install -d -o accounter -g accounter -m 750 /opt/accounter/logs
"${SUDO[@]}" install -d -o accounter -g accounter -m 755 "${BACKUP_DIR}"
"${SUDO[@]}" install -d -o accounter -g accounter -m 755 "${NEW_WEB_DIR}"

"${SUDO[@]}" tar -xzf "${REMOTE_STAGE}/${H5_ARCHIVE_NAME}" -C "${NEW_WEB_DIR}"
"${SUDO[@]}" chown -R accounter:accounter "${NEW_WEB_DIR}"
"${SUDO[@]}" find "${NEW_WEB_DIR}" -type d -exec chmod 755 {} +
"${SUDO[@]}" find "${NEW_WEB_DIR}" -type f -exec chmod 644 {} +

if [[ -f "${CURRENT_BINARY}" ]]; then
  "${SUDO[@]}" cp -p "${CURRENT_BINARY}" "${BACKUP_DIR}/accounter"
fi
if [[ -f /opt/accounter/bin/accounter-admin ]]; then
  "${SUDO[@]}" cp -p /opt/accounter/bin/accounter-admin "${BACKUP_DIR}/accounter-admin"
fi
if [[ -d "${CURRENT_WEB_DIR}" ]]; then
  "${SUDO[@]}" mv "${CURRENT_WEB_DIR}" "${BACKUP_DIR}/web"
fi

"${SUDO[@]}" install -o accounter -g accounter -m 755 \
  "${REMOTE_STAGE}/accounter" "${CURRENT_BINARY}"
"${SUDO[@]}" install -o root -g root -m 750 \
  "${REMOTE_STAGE}/accounter-admin" /opt/accounter/bin/accounter-admin
"${SUDO[@]}" mv "${NEW_WEB_DIR}" "${CURRENT_WEB_DIR}"
"${SUDO[@]}" chmod 755 /opt/accounter "${CURRENT_WEB_DIR}"
"${SUDO[@]}" install -m 644 \
  "${REMOTE_STAGE}/accounter.service" /etc/systemd/system/accounter.service
"${SUDO[@]}" systemctl daemon-reload
"${SUDO[@]}" systemctl restart accounter

healthy=0
for _ in {1..20}; do
  if curl -fsS http://127.0.0.1:8080/health >/dev/null; then
    healthy=1
    break
  fi
  sleep 1
done

if [[ "${healthy}" -ne 1 ]]; then
  echo "新版本健康检查失败，开始自动回滚"
  "${SUDO[@]}" journalctl -u accounter -n 80 --no-pager || true
  "${SUDO[@]}" systemctl stop accounter || true

  if [[ -f "${BACKUP_DIR}/accounter" ]]; then
    "${SUDO[@]}" install -o accounter -g accounter -m 755 \
      "${BACKUP_DIR}/accounter" "${CURRENT_BINARY}"
  fi
  if [[ -f "${BACKUP_DIR}/accounter-admin" ]]; then
    "${SUDO[@]}" install -o root -g root -m 750 \
      "${BACKUP_DIR}/accounter-admin" /opt/accounter/bin/accounter-admin
  fi
  if [[ -d "${BACKUP_DIR}/web" ]]; then
    if [[ -d "${CURRENT_WEB_DIR}" ]]; then
      "${SUDO[@]}" mv "${CURRENT_WEB_DIR}" "${BACKUP_DIR}/failed-web"
    fi
    "${SUDO[@]}" mv "${BACKUP_DIR}/web" "${CURRENT_WEB_DIR}"
  fi

  "${SUDO[@]}" systemctl start accounter || true
  exit 1
fi

"${SUDO[@]}" rm -rf -- "${REMOTE_STAGE}"
echo "ECS 本机健康检查通过"
echo "上一版备份: ${BACKUP_DIR}"
REMOTE_SCRIPT

if [[ -n "${PUBLIC_BASE_URL}" ]]; then
  echo "==> 检查公网健康状态"
  curl --retry 10 --retry-all-errors --retry-delay 2 -fsS "${PUBLIC_BASE_URL%/}/health"
  echo
fi

echo "==> 发布完成: ${RELEASE_ID}"
