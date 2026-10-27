# 校园失物招领平台 · Campus Return

基于 Vue 3、Go/Gin、SQLite 和 Hyperledger Fabric 的校园拾物登记与归还平台。支持多用户并行认领、管理员核验、预约交接、限时交接码、通知与评价、内容治理、贡献积分及交易追溯。

## 项目结构

```text
frontend/                 Vue 3 + TypeScript + Vite，含页面所需静态图片
cmd/server/               Web 服务入口
cmd/maintenance/          SQLite 一致性快照工具
internal/app/             认证、接口、数据库、交易队列与事件投影
internal/app/migrations/  数据库结构，启动时自动应用
internal/domain/          摘要规则、交易命令和账本类型
internal/ledger/          链码与 Demo 共用的确定性业务规则
backend/fabric/           Fabric Gateway、TLS 和组织签名身份
main.go                   Fabric 链码入口
deployment/               独立组织、通道与 Docker Compose 配置
scripts/                  启动、部署、检查、备份恢复与公网隧道
tests/                    单元、接口与浏览器回归测试
```

仓库不包含演示账号、恢复码、数据库、上传文件、运行密钥、节点证书、编译结果或课程提交材料。首次启动生成独立运行环境，不依赖原课程项目目录。页面图片是应用静态资源，来源记录见 `frontend/src/assets/campus/sources.json`；照片保留原署名，第三方图片权利归原作者。

## 环境依赖

| 环境 | 要求 |
|---|---|
| 前端 | Node.js 20.19+ 或 22.12+，npm |
| 服务与链码 | Go 1.23+ |
| 系统 | Linux，或 Windows + WSL 2 Linux |
| 容器与工具 | Docker Engine、Docker Compose v2、Python 3、curl、jq、OpenSSL |
| Fabric | 2.5.15 二进制工具、配置目录及匹配的 peer/orderer/ccenv/baseos 镜像 |

已验证环境：Windows 前端构建、Ubuntu/WSL Go 1.26.0、Docker 29.1.3、Fabric 2.5.15。Fabric 工具由部署者安装；项目脚本不会代装 WSL 或 Docker。

## 首次部署

1. 将仓库下载或克隆到本机，在 `frontend` 构建页面：

   ```bash
   npm ci --prefix frontend
   npm run build --prefix frontend
   ```

2. 在 Linux/WSL 中进入仓库根目录，指定 Fabric Samples 路径。该目录下应存在 `bin/peer`、`bin/cryptogen`、`bin/configtxgen`、`bin/osnadmin` 和 `config/`：

   ```bash
   export FABRIC_SAMPLES_PATH="$HOME/fabric-samples"
   docker info
   docker compose version
   bash scripts/start.sh
   ```

3. 打开 <http://127.0.0.1:8088>。健康检查为 <http://127.0.0.1:8088/api/v1/health>，正式模式应返回 `mode: hyperledger-fabric` 且 `ok: true`。

脚本依次创建组织材料、启动独立网络、加入通道、部署链码并启动 Web 服务。通道为 `returnchannel`，链码为 `campusreturn`；背书策略要求 Org1、Org2 共同背书。默认端口：Web 8088、Orderer 17050/17053、Org1 Peer 17051、Org2 Peer 19051。Fabric 端口仅绑定本机。

Windows 可使用包装脚本（`Distribution` 替换为 `wsl -l -q` 中的发行版名称，默认沿用 `Ubuntu-D`）：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/start.ps1 -Distribution Ubuntu -FabricSamplesPath /home/yourname/fabric-samples
```

首次部署需要拉取依赖和构建链码，等待时间通常长于日常启动。脚本失败时先读取对应输出，不要删除账本卷或密钥重试。

## 日常启动与账号

每次在同一运行目录执行 `bash scripts/start.sh`；Windows 使用上述包装脚本。它保留数据库、上传和账本，链码源码未变则复用现有部署。仅重启 Web 可执行 `bash scripts/restart.sh`。修改前端后重新运行 `npm run build --prefix frontend` 并刷新页面。

运行数据默认位于 `~/.local/share/campus-return`，可用 `CAMPUS_RETURN_RUNTIME` 指定独立目录。该目录中的 `secrets.env` 保存应用加密密钥和首次超管凭据；不要纳入 Git。查看初始管理员：

```bash
bash scripts/admin-info.sh
```

Windows：`powershell -File scripts/show-admin.ps1 -Distribution Ubuntu`。该命令会显示凭据，只应私下执行。

登录地址 `/#/login`，管理地址 `/#/admin`。初始超管用户名 `admin`，密码由脚本随机生成。公开注册只创建普通用户；注册后保存一次性恢复码。密码至少 6 个字符且不超过 bcrypt 的 72 字节上限。

| 角色 | 可操作范围 |
|---|---|
| 普通用户 | 登记、认领、撤回、预约、交接码、本人记录、通知、反馈、评价和贡献 |
| 业务管理员 | 审核、交接、物品治理、举报争议、评价治理、统计、交易进度和日志 |
| 超级管理员 | 全部业务管理，以及账号状态、凭据重置和管理员任免 |

超管在“用户与权限”授权已注册账号，角色或账号状态变化后旧会话撤销，需重新登录。后端独立校验接口权限；禁止自我提权、禁用自身及移除最后一名有效超管。

## 数据与信任边界

- SQLite WAL 保存完整内容、不可覆盖的版本快照、账号、会话、通知、反馈、评价和积分流水；Fabric 保存状态、内部编号、摘要和关键操作信息。
- 公开内容按固定字段顺序序列化后计算 SHA-256；核验答案与认领证明使用随机盐摘要，原文与盐采用 AES-256-GCM 加密后链下保存。摘要一致表示材料未被替换，不等于答案正确。
- 上传校验实际文件摘要与账号归属，支持 JPEG/PNG/WebP，最大 5 MB。公开信息中不要填写敏感证件及联系信息。
- Org2 提交登记、认领和撤回，Org1 提交管理交易。应用账号共享所属组织的 Gateway 签名身份；具体用户权限依赖后端认证，尚未接入个人 Fabric 证书或校园统一认证。
- 写操作先持久化操作编号；网络超时保持待核对，使用原交易信息恢复。有效区块事件及检查点更新数据库查询视图，通知与积分去重。
- 下架仅影响展示，关闭结束未确权物品的认领；已归还事实不能抹除，争议与积分纠正通过追加记录处理。

完整内容、图片、账本与密钥应配套保存。链上摘要无法恢复原文；加密密钥与数据库不匹配时服务拒绝启动。

## 公网与手机访问

先保持本机 Web 与 Fabric 运行，再在另一终端执行：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/public.ps1
```

Linux：`bash scripts/install-tunnel.sh`，随后 `bash scripts/public.sh`。脚本通过 Cloudflare Quick Tunnel 输出临时 HTTPS 地址；从该地址打开“手机访问”即可保存二维码。

本机和公网连接同一服务、账号与账本，不同域名需分别登录。公网增加隧道网络耗时，地址重启后通常变化，且要求电脑、WSL、Docker、Web 与隧道持续运行。它不是云服务器常驻部署；只暴露 Web，不开放数据库或 Fabric 端口。

## 测试

单元与静态检查不改动展示业务数据：

```bash
go test -timeout 120s ./...
go vet ./...
npm run build --prefix frontend
for file in scripts/*.sh; do bash -n "$file"; done
```

以下接口和浏览器回归会注册账号、创建物品并改变业务状态，**仅在专用测试环境运行**：

```bash
source scripts/env.sh
python3 tests/e2e.py
# 一键检查也包含上述写入型 E2E
bash scripts/check.sh
```

`tests/e2e.py` 强制检查真实 Fabric 模式。浏览器回归在仓库根目录执行 `node tests/browser.mjs`，需要 Edge 和前端开发依赖；通过环境变量 `ADMIN_PASSWORD` 或 `CAMPUS_CREDENTIALS_FILE` 提供本机管理员凭据。部分专项回归还需 `MANAGER_USERNAME`、`MANAGER_PASSWORD` 及对应测试记录，见脚本前置条件。结果输出到被 Git 忽略的 `artifacts/`。

只验证界面时，可在另一个端口与运行目录启动 Demo：

```bash
CAMPUS_RETURN_RUNTIME="$HOME/.local/share/campus-return-demo" APP_ADDR=127.0.0.1:8089 APP_MODE=demo bash scripts/run.sh
```

Demo 不产生真实 Fabric 交易，不能用于区块链验收。

## 备份、恢复与停止

先在 Web 终端按 Ctrl+C 停止服务，然后：

```bash
export BACKUP_PASSWORD='自行设置至少12位的备份密码'
bash scripts/backup.sh
```

脚本创建一致性 SQLite 快照，短暂停止本项目网络以备份账本卷，并打包上传和网络身份，加密后恢复网络容器。`secrets.env` 不进入备份包，须另外安全保存；备份完成后重新启动 Web。

恢复仅支持数据库及同名账本卷均不存在的空环境。先恢复配套 `secrets.env` 到运行目录，再执行：

```bash
export BACKUP_PASSWORD='原备份密码'
bash scripts/restore.sh /absolute/path/backup.tar.gz.enc
bash scripts/start.sh
```

停止新网络执行 `bash scripts/stop.sh`，数据卷保留。不要把单独的 SQLite 文件当作完整系统备份。

## 常见问题

- **页面空白或静态资源 404**：先构建 `frontend/dist`，确认从当前源码目录启动。
- **无法连接 Fabric**：检查 Docker、`cr-*` 容器、证书路径及 Samples 工具；查看健康接口。只重启本项目，不清理其他课程网络。
- **操作待确认**：在“操作进度”查看同一操作，不反复新建请求。服务恢复后会继续核对。
- **本机正常但公网失败**：检查隧道日志与最新地址；旧二维码不会自动指向新隧道。
- **密码重置或角色变化后退出**：会话撤销是权限规则，重新登录即可。

## 开发说明

本项目在本组原课程项目基础上迭代，复用 Go/Gin、Fabric Gateway 连接与签名身份接入经验，扩展数据模型、链码命令、管理流程及前端。业务规则位于 `internal/ledger`，通过同一实现供链码和 Demo 调用，修改时应保持规则与权限测试同步。仓库提供源码和运行说明；演示材料、课程报告与本机凭据由交接目录单独管理。
