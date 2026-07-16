#!/usr/bin/env bash
#
# nmsappsrv 部署冒烟脚本 (Tier 2 — 对应 docs/DEPLOYMENT_CHECKLIST.md §6)
#
# 设计目标：部署后按顺序跑，任一步红即停。脚本面向 Linux 容器部署目标。
#
# 用法:
#   ./scripts/smoke.sh [BASE_URL] [REDIS_CLI]
#     BASE_URL     HTTP 基址, 默认 http://localhost:8080
#     REDIS_CLI    redis-cli 路径, 默认 redis-cli (用于队列/online 检查)
#
# 退出码: 0 = 全绿, 非0 = 在第一步失败处停止
#
set -uo pipefail

BASE_URL="${1:-http://localhost:8080}"
REDIS_CLI="${2:-redis-cli}"

log()  { printf '\033[32m[smoke]\033[0m %s\n' "$*"; }
info() { printf '\033[36m[smoke]\033[0m %s\n' "$*"; }
warn() { printf '\033[33m[smoke]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[31m[smoke] FAIL:\033[0m %s\n' "$*" >&2; exit 1; }

# 带超时的 curl，返回 body；失败(非2xx)时 die
curl_ok() {
  local path="$1"; shift
  local out
  out="$(curl -fsS --max-time 5 "${BASE_URL}${path}" "$@" 2>&1)" || die "GET ${path} 非 2xx 或无响应: ${out}"
  echo "$out"
}

# 1. 静态检查（仅在仓库根目录且有 go 时执行；CI 中通常单独跑）
step_static() {
  log "step 1/10: 静态检查 (go build / vet / test)"
  if [ -f go.mod ] && command -v go >/dev/null 2>&1; then
    go build ./... || die "go build 失败"
    go vet ./...  || die "go vet 失败"
    go test ./... -count=1 || die "go test 失败"
  else
    warn "不在仓库根目录或无可执行 go，跳过静态检查（请在 CI 中单独执行）"
  fi
}

# 2. 配置加载（进程已起时无法在此拉起；这里仅提示人工确认项）
step_config() {
  log "step 2/10: 配置加载"
  info "确认启动日志含 'config loaded' 且无 'config validation failed'。"
  info "确认注入的环境变量生效: NMS_DB_* / NMS_REDIS_* / NMS_JWT_SECRET / NMS_FILE_SERVER_*。"
}

# 3. license 校验（可选：有 license-tool 时检查 fingerprint 一致性）
step_license() {
  log "step 3/10: license 校验 (fingerprint 一致性)"
  if command -v license-tool >/dev/null 2>&1; then
    local fp
    fp="$(license-tool fingerprint 2>/dev/null | tr -d '-' | tr 'a-z' 'A-Z')"
    local host_uuid
    host_uuid="$(dmidecode -s system-uuid 2>/dev/null | tr -d '-' | tr 'a-z' 'A-Z')" || true
    if [ -n "${host_uuid:-}" ] && [ "${fp}" != "${host_uuid}" ]; then
      die "license fingerprint(${fp}) 与宿主 system-uuid(${host_uuid}) 不一致"
    fi
    info "fingerprint=${fp:-<空>}, host_uuid=${host_uuid:-<空>}"
  else
    warn "未找到 license-tool，跳过 fingerprint 检查（手动确认 license.required=true 下能过中间件）"
  fi
}

# 4. HTTP 健康检查
step_health() {
  log "step 4/10: HTTP 健康检查 (/healthz + /readyz)"
  local lz; lz="$(curl_ok /healthz)"
  info "/healthz -> ${lz}"
  local rz; rz="$(curl_ok /readyz)"
  info "/readyz -> ${rz}"
}

# 5. TR-069 Inform（redis 中应有 tr069:queue:<sn> 与 online_<neId>）
step_tr069() {
  log "step 5/10: TR-069 Inform 链路"
  if command -v "${REDIS_CLI}" >/dev/null 2>&1; then
    local n
    n="$("${REDIS_CLI}" keys 'tr069:queue:*' 2>/dev/null | wc -l)"
    info "tr069:queue:* 键数 = ${n}"
    local on
    on="$("${REDIS_CLI}" keys 'online_*' 2>/dev/null | wc -l)"
    info "online_* 键数 = ${on} (设备上线后出现)"
  else
    warn "未找到 ${REDIS_CLI}，跳过队列检查（手动确认设备 Inform 后 online_<neId> 出现）"
  fi
}

# 6. operation_queue 链路（发一个 SPV，确认入队并被消费）
step_operation_queue() {
  log "step 6/10: operation_queue 链路"
  if command -v "${REDIS_CLI}" >/dev/null 2>&1; then
    local len_before
    len_before="$("${REDIS_CLI}" LLEN operation_queue 2>/dev/null || echo 0)"
    info "operation_queue 当前长度 = ${len_before}"
    info "在 UI/API 触发一个 SetParameterValues 后，队列应短暂增长并由 worker 消费清空。"
  else
    warn "未找到 ${REDIS_CLI}，跳过队列检查"
  fi
}

# 7. web_callback 链路
step_web_callback() {
  log "step 7/10: web_callback 链路"
  if command -v "${REDIS_CLI}" >/dev/null 2>&1; then
    local len
    len="$("${REDIS_CLI}" LLEN queue:web_callback 2>/dev/null || echo 0)"
    info "queue:web_callback 当前长度 = ${len} (Inform 触发后应出现并被 bridge 消费)"
  else
    warn "未找到 ${REDIS_CLI}，跳过队列检查"
  fi
}

# 8. 文件服务
step_file_server() {
  log "step 8/10: 文件服务 (/acs-file-server/ca/...)"
  # 仅探活：ca 目录根返回 200/404 都算端点可达；401/403 也说明路由存在
  local code
  code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "${BASE_URL}/acs-file-server/ca/" 2>/dev/null)" || code="000"
  info "/acs-file-server/ca/ -> HTTP ${code}"
  [ "${code}" != "000" ] || die "文件服务不可达"
}

# 9. WebSocket
step_websocket() {
  log "step 9/10: WebSocket 推送"
  if command -v wscat >/dev/null 2>&1; then
    info "wscat -c ws://localhost:8080/ws 应收到任意推送（手动确认）"
  else
    warn "未找到 wscat，跳过（手动确认 ws://<host>/ws 能收到推送）"
  fi
}

# 10. mTLS（按需）
step_mtls() {
  log "step 10/10: mTLS (GMLC/LMF 客户端证书, 按需)"
  if [ -n "${MTLS_HOST:-}" ] && command -v openssl >/dev/null 2>&1; then
    info "openssl s_client -connect ${MTLS_HOST} -cert cert/gmlc/client.p12 ... 验证握手"
  else
    warn "未配置 MTLS_HOST 或无 openssl，跳过（按需启用）"
  fi
}

main() {
  step_static
  step_config
  step_license
  step_health
  step_tr069
  step_operation_queue
  step_web_callback
  step_file_server
  step_websocket
  step_mtls
  log "冒烟全绿 ✓"
}

main "$@"
