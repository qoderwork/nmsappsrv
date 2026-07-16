# Tier 2 上线前自检表（Deployment Pre-flight Checklist）

> 范围：把 `nmsappsrv` 从"能编译跑通"推到"能切流量替换 Java nms-serv"。  
> 配套：mTLS PKCS12 证书、T-Platform RSA 密钥、配置核对、冒烟脚本。  
> 不含：业务功能补全（那是 Tier 1），也不含 mail 双 Vault 密钥兼容（Tier 3，按需）。

---

## 0. 必读约束

- **nmsappsrv 不使用 HashiCorp Vault**（已在 2026-07-13 锁定剔除出分母）。
- **nmsappsrv 不使用 HA cluster**（vip-monitor/vip-subscriber worker 默认不启动；`ha.enabled=false`）。
- **T-Platform 的 RSA 私钥格式 = PKCS#8 PEM**（`/cert/t-platform/private-key-pkcs8.pem`），公钥为 SPKI PEM。go-infra licensing 包验签 Ed25519 但 T-Platform 走 RSA 路径独立。
- **mTLS 客户端证书 = PKCS#12**（`.p12`），目标 GMLC + LMF1-4 各一份。
- **机器绑定 = 宿主 `dmidecode -s system-uuid` 去杠大写**（不是 `/etc/machine-id`、不是 `board_serial`）。容器部署需把宿主 `/dev/mem` + `/sbin/dmidecode` bind-mount 进容器（同 Java `java-compose.yml`）。

---

## 1. 配置文件 `configs/config.yaml` 核对

按以下清单逐项确认，标注"现状 / 计划值 / 备注"。

### 1.1 基础服务

| 字段 | 现状（默认） | 计划值 | 备注 |
|---|---|---|---|
| `server.name` | `nmsappsrv` | `nmsappsrv` | |
| `server.port` | `8080` | `8080` | 与反向代理/LB 对齐 |
| `server.mode` | `release` | `release` | 上线必须 release |
| `server.cors_allowed_origins` | localhost:3000/5173 | 替换为前端正式域名 | |

### 1.2 数据库

| 字段 | 计划值 | 备注 |
|---|---|---|
| `database.host` | 目标 MySQL host | 建议复用 Java 的 `base_station` schema |
| `database.port` | `13306` | |
| `database.user` / `password` | 由部署环境注入 | 强烈建议改用 `NMS_DB_*` env 覆盖，不要把密码写进 git |
| `database.dbname` | `base_station` | |
| `database.max_idle_conns` | `10` | |
| `database.max_open_conns` | `100` | |
| `database.log_level` | `warn`（上线）/ `info`（调试期） | |

### 1.3 Redis

| 字段 | 计划值 | 备注 |
|---|---|---|
| `redis.host` / `port` | 目标 Redis | |
| `redis.password` | 由部署环境注入 | |
| `redis.db` | `0` | |
| `redis.pool_size` | `20` | |

### 1.4 JWT

| 字段 | 计划值 | 备注 |
|---|---|---|
| `jwt.secret` | **必须 ≥32 字节**，env `NMS_JWT_SECRET` 注入 | `openssl rand -base64 48` 生成；yaml 留空，靠 env |

### 1.5 Logger

| 字段 | 计划值 | 备注 |
|---|---|---|
| `logger.filename` | `/var/log/nmsappsrv/nmsappsrv.log` 或 `./logs/nmsappsrv.log` | 确保目录可写 |
| `logger.level` | `info` | |
| `logger.max_size_mb` | `100` | |
| `logger.max_backups` | `10` | |
| `logger.retention_days` | `30` | |
| `logger.compress` | `true` | |
| `logger.stdout` | `true`（容器化） | |

### 1.6 TR-069

| 字段 | 计划值 | 备注 |
|---|---|---|
| `tr069.acs_url` | `http://<host>:8080/acs` | LB 后改成 https |
| `tr069.inform_interval` | `300` | |
| `tr069.connection_timeout` | `30` | |
| `tr069.udp_connection_request_port` | `50000` | UDP 端口需放通 |
| `tr069.file_server_ip` | `http://<host>:8080` | 设备拉文件的 baseURL |
| `tr069.file_server_username` | 由部署环境注入（env `NMS_FILE_SERVER_USERNAME`） | 默认 `admin` |
| `tr069.file_server_password` | 由部署环境注入（env `NMS_FILE_SERVER_PASSWORD`） | 默认 `admin`，**上线必须改** |
| `tr069.enable_ask_reboot` | `false` | |
| `tr069.enable_xml_signature` | 按需 | |
| `tr069.private_key_path` | `/cert/t-platform/private-key-pkcs8.pem`（如果走 XML 签名） | |
| `tr069.certificate_path` | `/cert/t-platform/public-key.pem` | |

### 1.7 SNMP

| 字段 | 计划值 | 备注 |
|---|---|---|
| `snmp.trap_listen_port` | `162` | 需 root 或 CAP_NET_BIND |
| `snmp.enterprise_oid` | `31664` | |
| `snmp.default_version` | `v2c` | |
| `snmp.default_community` | 由部署环境注入 | |

### 1.8 Mail（最小集）

| 字段 | 计划值 | 备注 |
|---|---|---|
| `mail.aes_key` | 64 hex 字符（32 字节 AES-256） | 由部署环境注入（env `NMS_MAIL_AES_KEY`） |

> Tier 3 兼容（Java 双 Vault 密钥加密的存量 mail 密码）按需做，本表只覆盖最低要求。

### 1.9 HA / Vault

| 字段 | 计划值 | 备注 |
|---|---|---|
| `ha.enabled` | **`false`** | nmsappsrv 不做 HA cluster |
| `vault.*` | **不存在** | 已被剔除，不配置 |

### 1.10 平台 / 升级 / 文件服务 / License / ZTP

| 字段 | 计划值 | 备注 |
|---|---|---|
| `platform_files.rsa_public_key_path` | `/cert/password/publicKey.pem` | license 验签 |
| `platform_files.nms_manual_doc_path` | `./docs/nms_manual.pdf` | |
| `platform_files.platform_log_dir` | `./logs/platform` | |
| `upgrade.upload_dir` | `./data/upgrade-files` | 设备升级包上传 |
| `file_server.*` | 全部按 `default` 路径即可 | 见下表目录结构 |
| `license.required` | **`true`** | 上线必须 |
| `license.install_dir` | `./data/license` | |
| `license.machine_fingerprint_override` | 留空 | 让程序读宿主 `dmidecode -s system-uuid` |
| `ztp.sftp_enabled` | 按需 | 默认 `false`（HTTP `/acs-file-server/ztpFile` 已能服务 AOS） |
| `ztp.sftp_host` | `:10022` | |
| `ztp.sftp_host_key` | `./data/ztp/hostkey.pem` | 首次启动自动生成 |

---

## 2. 文件系统目录结构

容器外/宿主机需要存在以下目录（缺则启动失败）：

```
<project_root>/
├── cert/                                  # 关键证书目录（bind-mount）
│   ├── t-platform/
│   │   ├── private-key-pkcs8.pem          # T-Platform RSA 私钥（PKCS#8 PEM）
│   │   └── public-key.pem                 # T-Platform RSA 公钥（SPKI PEM）
│   ├── password/
│   │   └── publicKey.pem                  # license 验签公钥
│   ├── gmlc/                              # GMLC 客户端 PKCS#12（mTLS 客户端证书）
│   │   └── client.p12
│   └── lmf1..lmf4/                        # 每个 LMF 一份
│       └── client.p12
├── data/
│   ├── acs-file-server/                   # file_server.root
│   │   ├── upgrade/  config/  batch-process/  log/  mr/  capture/
│   │   ├── mml-result/  pm/  nrm/  mnormal/  north-file/
│   │   ├── piecemeal-temp/  mr-report/  ca/  license/  ztp/
│   ├── license/                           # license 持久化（license.required=true）
│   ├── upgrade-files/                     # upgrade.upload_dir
│   └── ztp/
│       └── hostkey.pem                    # SFTP 主机密钥（自动生成）
├── logs/                                  # logger.filename 父目录
│   ├── nmsappsrv.log
│   └── platform/
├── docs/
│   └── nms_manual.pdf                     # platform_files.nms_manual_doc_path
└── configs/
    └── config.yaml                        # 唯一配置文件
```

权限要求：
- 进程用户对 `data/`、`logs/`、`docs/` 有 rwx。
- 进程用户对 `cert/t-platform/private-key-pkcs8.pem` 有 `r--`（400 更佳）。
- 进程用户对 `cert/gmlc/client.p12`、`cert/lmf*/client.p12` 有 `r--`。

---

## 3. 网络端口

| 端口 | 用途 | 放通 |
|---|---|---|
| `8080/tcp` | HTTP 主服务（API + ACS + acs-file-server） | LB → 容器 |
| `50000/udp` | TR-069 Connection Request（设备主动连 ACS 的回调端口） | 公网/内网入站 |
| `162/udp` | SNMP Trap 接收（可选） | 内网入站 |
| `10022/tcp` | ZTP 嵌入式 SFTP（仅当 `ztp.sftp_enabled=true`） | 内网入站 |
| `3306/tcp` | MySQL（出站到 DB） | |
| `6379/tcp` | Redis（出站到 Redis） | |

---

## 4. 环境变量覆盖（推荐用法）

所有带"由部署环境注入"的字段都建议用 env 覆盖，避免把密钥写进 `config.yaml`：

```bash
export NMS_DB_HOST=192.168.x.x
export NMS_DB_USER=nms
export NMS_DB_PASSWORD=<vault-or-secret>
export NMS_REDIS_HOST=192.168.x.x
export NMS_REDIS_PASSWORD=<secret>
export NMS_JWT_SECRET=$(openssl rand -base64 48)
export NMS_FILE_SERVER_USERNAME=admin
export NMS_FILE_SERVER_PASSWORD=<strong>
export NMS_MAIL_AES_KEY=<64-hex>
export NMS_LICENSE_INSTALL_DIR=./data/license
```

> 当前 nmsappsrv 还未把所有 `Cfg.*` 字段接 env 覆盖。**上线前需要补 env-binding**（这是 Tier 2 的代码工作之一）。

---

## 5. 数据库 / Redis 前置

- **MySQL**：`base_station` schema 存在；账号有 DML 权限（nmsappsrv 不做 DDL 迁移，靠 Java 侧先迁移好或独立 migration 工具）。
- **Redis**：与 Java 共用一个实例，db 编号要隔开（Java 用 db0，nmsappsrv 建议用 db1 或独立实例）；MQ 队列 key 空间已统一为 `operation_queue` / `queue:web_callback` / `queue:inform` / `queue:event_result` / `queue:alarm` / `queue:snmp` / `queue:pm` 等，避免和 Java RabbitMQ key 冲突（nmsappsrv 用 Redis LIST/PUBSUB，不是 RabbitMQ）。

---

## 6. 冒烟脚本（部署后必跑）

按顺序跑，任意一步红就停：

1. **静态检查**
   ```bash
   go build ./... && go vet ./... && go test ./... -count=1
   ```
2. **配置加载**：启动进程，看 `config loaded` 日志，无 `invalid` 报错。
3. **license 校验**：`license-tool fingerprint` 输出与 `dmidecode -s system-uuid`（去杠大写）一致；放入 `./data/license/*.lic` 后 `license.required=true` 下能正常通过中间件。
4. **HTTP 健康检查**：`curl -sf http://localhost:8080/healthz`（如果加了健康检查端点）或 `curl -sf http://localhost:8080/api/v2/...` 任一未鉴权端点。
5. **TR-069 Inform**：启动后 `tr069:queue:<sn>` LIST 出现；任意一台设备 Inform 进来后 `online_<neId>` 出现。
6. **operation_queue 链路**：调一个 SPV（如 `SetParameterValues`），看 `redis-cli LRANGE operation_queue 0 -1` 出现，worker 消费后队列清空。
7. **web_callback 链路**：Inform 触发后 `redis-cli LRANGE queue:web_callback 0 -1` 出现并被 bridge 消费。
8. **文件服务**：`curl -I http://localhost:8080/acs-file-server/ca/...` 200。
9. **WebSocket**：`wscat -c ws://localhost:8080/ws` 收到任意推送。
10. **mTLS**（如果启用）：用 `openssl s_client -connect <host>:port -cert cert/gmlc/client.p12 -key ...` 验证握手。

---

## 7. 已知"上线前必须修"的代码缺口（Tier 2 范围）

1. **env 覆盖缺失**：`internal/config/config.go` 多数字段没接 env，需要补 `NMS_DB_*` / `NMS_REDIS_*` / `NMS_JWT_SECRET` / `NMS_FILE_SERVER_*` 绑定（当前只有 `mail.aes_key` 的 validator，没有 from-env loader）。
2. **健康检查端点**：未确认有 `/healthz`，需要补（容器探针用）。
3. **优雅关闭信号**：确认 `pkg/shutdown` 已生效，所有 worker（`operation-worker` / `mml-worker` / `param-scheduler` / `ws-bridge` / `offline-worker`）都能 graceful stop。
4. **mTLS 中间件**：如果有 GMLC/LMF 客户端证书需求，需要在 router 层加 TLS 双向认证中间件 + `ca cert pool`。
5. **license fingerprint 一致性**：宿主机 `dmidecode -s system-uuid` 在容器内能读到（`java-compose.yml` 已挂 `/dev/mem` + `/sbin/dmidecode`，nmsappsrv 容器需要同样挂载）。
6. **T-Platform 私钥读取**：`tr069.private_key_path` / `certificate_path` 启动时存在性检查 + 失败 fail-fast。

---

## 8. 不在本表（按需）

- **mail 双 Vault 密钥兼容**（Tier 3）：只有迁移存量 Java 加密 mail 密码时才需要。
- **HA VIP 切换**（Tier 已被剔除）：`ha.enabled=false` 即可。
- **路径兼容**（Tier 5）：`/v1` 别名已加，子路径仍是 Go 命名，绑死 Java 路径的客户端才需要重命名。
