[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md) | [Türkçe](/README.tr_TR.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/MHSanaei/3x-ui/releases"><img src="https://img.shields.io/github/v/release/mhsanaei/3x-ui" alt="Release"></a>
  <a href="https://github.com/MHSanaei/3x-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/mhsanaei/3x-ui/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/mhsanaei/3x-ui.svg" alt="GO Version"></a>
  <a href="https://github.com/MHSanaei/3x-ui/releases/latest"><img src="https://img.shields.io/github/downloads/mhsanaei/3x-ui/total.svg" alt="Downloads"></a>
  <a href="https://www.gnu.org/licenses/gpl-3.0.en.html"><img src="https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true" alt="License"></a>
  <a href="https://pkg.go.dev/github.com/mhsanaei/3x-ui/v3"><img src="https://pkg.go.dev/badge/github.com/mhsanaei/3x-ui/v3.svg" alt="Go Reference"></a>
  <a href="https://docs.sanaei.dev"><img src="https://img.shields.io/badge/docs-docs.sanaei.dev-22d3ee" alt="Documentation"></a>
</p>

**3X-UI** 是一个先进的开源 Web 控制面板，用于管理 [Xray-core](https://github.com/XTLS/Xray-core) 服务器。它提供简洁、多语言的界面，用于部署、配置和监控各种代理与 VPN 协议——从单台 VPS 到多节点部署。

3X-UI 作为原始 X-UI 项目的增强分支（fork），增加了更广泛的协议支持、更好的稳定性、按客户端的流量统计以及许多提升使用体验的功能。

> [!IMPORTANT]
> 本项目仅供个人使用。请勿将其用于非法目的，也请勿在生产环境中使用。

## 功能特性

- **多协议入站** — VLESS、VMess、Trojan、Shadowsocks、WireGuard、AmneziaWG、TUIC v5、Hysteria2、MTProto、HTTP、SOCKS (Mixed)、Dokodemo-door / Tunnel 和 TUN。
- **现代传输与安全** — TCP (Raw)、mKCP、WebSocket、gRPC、HTTPUpgrade 和 XHTTP，并通过 TLS、XTLS 和 REALITY 加密。
- **内置 AmneziaWG** — 抗 DPI 的 WireGuard 直接在面板内的用户态网络栈上运行，无需内核模块、DKMS 或额外软件包。
- **内置 TUIC v5** — 基于 QUIC 的高性能代理，支持原生 UDP 中继流量统计、0-RTT 握手和 BBR 拥塞控制。
- **MTProto 代理** — 按客户端配置 FakeTLS 密钥、广告标签和配额，实时生效且不会断开已有连接。
- **回落 (Fallback)** — 通过 Xray 的 fallback 功能在单个端口上提供多种协议（例如在 443 端口上同时使用 VLESS 和 Trojan）。
- **按客户端管理** — 流量配额、到期日期、可豁免受信任地址的 IP 限制、HWID 设备数限制、定时续期周期、实时在线状态，以及一键分享链接、二维码和订阅。
- **流量统计** — 按入站、按客户端、按出站统计，并支持重置控制。
- **多节点支持** — 从单一面板管理并扩展到多台服务器，并可将入站克隆到其他节点。
- **出站与路由** — WARP、NordVPN、PIA、自定义路由规则、支持均衡器间回退的负载均衡器，以及出站代理链。内置的 geosite 与 geoip 分类可直接在规则编辑器中浏览。
- **内置订阅服务器** — 提供 raw、JSON 和 Clash 输出，可依据客户端 User-Agent 自动选择，并支持[自定义页面模板](docs/custom-subscription-templates.md)。
- **Telegram 和 Discord 机器人**，用于远程监控和管理。
- **RESTful API**，支持带作用域、可设置有效期的令牌，并提供面板内置的 API 参考文档。
- **可安装面板 (PWA)** — 将 3X-UI 固定到桌面或手机主屏幕。
- **灵活的存储** — SQLite（默认）或 PostgreSQL。
- **14 种界面语言**，支持深色和浅色主题。
- **Fail2ban 集成**，用于强制执行按客户端的 IP 限制。

## 截图

<details>
<summary>点击展开</summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
  <img alt="Overview" src="./media/01-overview-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/02-add-inbound-dark.png">
  <img alt="Inbounds" src="./media/02-add-inbound-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/03-add-client-dark.png">
  <img alt="Add client" src="./media/03-add-client-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/05-add-nodes-dark.png">
  <img alt="Configs" src="./media/05-add-nodes-light.png">
</picture>

</details>

## 快速开始

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

若要安装特定版本，请在命令后附加对应的标签（例如 `v3.7.0`）：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) v3.7.0
```

若要安装滚动更新的 **dev** 版本（来自 `main` 的最新逐次提交预发布版本，而非稳定版本），请传入 `dev-latest`：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) dev-latest
```

安装过程中会生成随机的用户名、密码和访问路径。安装完成后，运行 `x-ui` 打开管理菜单，您可以在其中启动/停止服务、查看或重置登录凭据、管理 SSL 证书等。

每个发布资源都会在其旁边附带一个 `.sha256` 校验和。`install.sh` 和更新程序都会据此校验压缩包，不匹配时中止。

完整文档（安装、配置、运维以及完整的 API 参考）请访问 **[docs.sanaei.dev](https://docs.sanaei.dev/zh)**。

### HTB 分支原生安装（不使用 Docker）

上面的官方安装命令安装上游版本。本分支的 HTB 代码需要使用本分支编译的程序，不能仅下载本仓库的脚本后安装上游安装包。

构建脚本支持 amd64、arm64、armv7、armv6 和 386。在 Linux 构建机准备 Go 1.27+、Node.js 26+、npm、musl C 编译器（例如 `musl-tools` 提供的 `musl-gcc`）、curl、unzip、tar 后，进入本仓库的 `dev` 分支运行：

```bash
bash scripts/build-native.sh
```

输出位于 `dist/native/`，包含当前架构的 `x-ui-linux-<架构>.tar.gz`、同名 `.sha256` 校验文件和 `install.sh`。程序使用 musl 静态链接；可通过 `XUI_NATIVE_CC` 指定对应架构的 musl 编译器。构建过程不需要 Docker，复用 `DockerInit.sh` 下载 Xray 等运行资源。将这三个文件复制到目标 Linux 主机，在其目录中运行（以下以 amd64 为例）：

```bash
sudo env XUI_INSTALL_ARCHIVE="$PWD/x-ui-linux-amd64.tar.gz" bash ./install.sh
```

脚本会验证安装包及必需文件，安装发行版对应的 tc/kmod 依赖，配置 HTB/IFB 等模块开机加载，并沿用原有 systemd/OpenRC 安装流程。控制脚本和服务文件从同一个本地包读取，不会下载上游程序覆盖本分支。目标主机仍需支持这些内核功能；安装用户态依赖不能替代缺失的内核支持。

远程安装源由 `XUI_RELEASE_REPO` 指定，默认 `limit81995/3x-ui`；`dev-latest` 的脚本分支默认 `dev`，可用 `XUI_RELEASE_REF` 覆盖。远程安装必须先在指定仓库发布包含本分支代码的对应 Release；仅推送 `dev` 不会自动产生安装包。目前本地包安装不依赖发布 Release。

本分支升级也使用同样的本地包安装命令，保留原数据库。原有管理菜单/面板的在线更新入口仍跟随上游，使用它们会换回上游程序；本次未改动这些业务入口。

### 无人值守安装

安装程序也可以**非交互式**运行，适用于 cloud-init。
设置 `XUI_NONINTERACTIVE=1`（或在无 TTY 的情况下通过管道传入），它就会全程
零提示地完成端到端安装，生成随机凭据并写入
`/etc/x-ui/install-result.env`。请参阅 [`deploy/`](deploy/)：

- [Cloud-init user-data](deploy/cloud-init/) — 在任意云平台上无人值守安装（Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle）
- [Hetzner Cloud 说明](deploy/marketplace/hetzner/) — 在 Hetzner 上基于 cloud-init 的部署

## 支持的平台

**操作系统：** Ubuntu、Debian、Armbian、Fedora、CentOS、RHEL、AlmaLinux、Rocky Linux、Oracle Linux、Amazon Linux、Virtuozzo、Arch、Manjaro、Parch、openSUSE (Tumbleweed / Leap)、Alpine 和 Windows。

**架构：** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`。

## 数据库选项

3X-UI 支持两种后端，可在安装时选择：

- **SQLite**（默认）— 位于 `/etc/x-ui/x-ui.db` 的单个文件。无需配置，适合中小型部署。
- **PostgreSQL** — 推荐用于大量客户端或多节点设置。安装程序可以为您在本地安装 PostgreSQL，或接受指向现有服务器的 DSN。

运行时通过环境变量选择后端（安装程序会为您写入 `/etc/default/x-ui`）：

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### 将现有的 SQLite 安装迁移到 PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# 然后在 /etc/default/x-ui 中设置 XUI_DB_TYPE 和 XUI_DB_DSN 并重启：
systemctl restart x-ui
```

源 SQLite 文件保持不变；在确认新后端正常工作后，请手动删除它。

### Docker

默认的 `docker compose up -d` 仍使用 SQLite。若要使用捆绑的 PostgreSQL 服务运行，请取消注释 `docker-compose.yml` 中的两行 `XUI_DB_*` 环境变量，并使用该 profile 启动：

```bash
docker compose --profile postgres up -d
```

该镜像捆绑了 Fail2ban（默认启用），用于强制执行按客户端的 **IP 限制**。Fail2ban 使用 `iptables` 封禁违规者，这需要 `NET_ADMIN` 权限。`docker-compose.yml` 已通过 `cap_add` 授予该权限；如果您改用 `docker run` 启动容器，请自行添加这些权限，否则封禁只会被记录而永远不会生效：

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

### 入站端口限速（tc）

新增和编辑入站时可设置“端口限速”，范围为 `0–10000 Mbps`；`0` 表示不限速。
同一端口的所有用户及 TCP/UDP、IPv4/IPv6 流量共享额度，上传、下载各有独立额度。
例如设置 `100`，表示上传合计最多约 100 Mbps，下载合计最多约 100 Mbps。
实现使用 Linux `tc clsact + flower + mirred → IFB + HTB + FQ-CoDel`：上传、下载各使用一个 IFB，每个端口有独立的 HTB 类别，`rate = ceil = 配置速率`。超过即时速率的报文先排队发送，不再使用 police 丢弃超额报文。队列拥塞时仍可能丢包；实际应用层速度还受协议开销、链路和对端影响，不保证测速一定达到设定值。它不是每用户限速或流量配额。

配置保存在数据库中，在入站列表和详情中显示。保存后后台立即尝试应用；面板启动时及每分钟重新读取内核实际规则，修复速率变化、丢失或被修改的分类规则。
取消限速、禁用、删除、到期、耗尽配额或更换端口后，会移除不再使用的规则。旧版本 police 动作会自动迁移并清理，其内核引用的释放可能延迟到下一次校准。
多个启用的入站若在同一节点共用端口，即使监听地址或协议不同，也必须设置相同的限速数值。

镜像已安装 `iproute2-tc`，官方 Compose 已提供 `NET_ADMIN`，无需为 HTB 修改主 Compose 文件；即使关闭 Fail2ban，启用端口限速时仍须保留 `NET_ADMIN`。宿主机内核须支持 `ifb`、`sch_htb`、`sch_fq_codel`、`sch_ingress`（clsact）、`cls_flower`、`act_mirred`；二进制部署需安装系统的 `iproute2`/`tc` 并提供 `CAP_NET_ADMIN`。
默认作用于当前网络命名空间中具有 IPv4/IPv6 默认路由的接口。多出口、策略路由或其他公网接口可设置 `XUI_TC_INTERFACES=eth0,eth1`，只选择真实承载入站流量的接口，避免同一报文经过多个选中接口而重复计量。
bridge 模式匹配容器内的监听端口，host 模式匹配宿主机端口；若前面有反向代理、隧道或端口转发，应在实际承载该端口流量的位置设置规则。
回环和 Unix socket 入站不支持此设置，应限制其公网父入站。

API 的 `POST /panel/api/inbounds/add`、`POST /panel/api/inbounds/update/:id` 接受整数 `speedLimit`，列表和详情接口返回该字段。例如在原有入站请求中添加 `"speedLimit": 100`；更新时传 `0` 或省略此字段会取消限速。
远端入站由实际承载流量的节点执行，主面板与节点均需升级到支持此字段的版本。

物理接口规则使用优先级 `48100–48103` 和 filter handle `0x3a00xxxx`，请为本面板保留这段编号。IFB 接口 `xui-tc-up`、`xui-tc-down` 由面板独占管理，root handle 为 `3a00:`，类别编号为端口的十六进制值，接口 alias 用于确认所有权；同名但未标记归属的接口不会被接管。同一个网络命名空间只应由一个面板管理这段规则。
校准不替换物理接口现有 root qdisc，也不删除其他 clsact 规则；只在面板自己的 IFB 上维护 HTB 和队列。需要先执行的 tc ACL/打标规则应放在优先级 `48100` 之前，因为 redirect 会终止当前分类链。如果接口已有旧式 ingress qdisc，需先自行规划迁移为 clsact。
应用失败会在面板日志中记录 `tc speed limit: reconciliation failed` 并自动重试；保存成功表示配置已持久化，不代表内核一定已应用。

## 环境变量

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `XUI_DB_TYPE` | 数据库后端：`sqlite` 或 `postgres` | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL 连接字符串（当 `XUI_DB_TYPE=postgres` 时） | — |
| `XUI_DB_FOLDER` | SQLite 数据库文件所在目录 | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | 最大打开连接数（PostgreSQL 连接池） | — |
| `XUI_DB_MAX_IDLE_CONNS` | 最大空闲连接数（PostgreSQL 连接池） | — |
| `XUI_INIT_WEB_BASE_PATH` | Web 面板的初始 URI 路径 | `/` |
| `XUI_ENABLE_FAIL2BAN` | 启用基于 Fail2ban 的 IP 限制 | `true` |
| `XUI_TC_INTERFACES` | 端口限速接口，多个接口用逗号分隔 | IPv4/IPv6 默认路由接口 |
| `XUI_LOG_LEVEL` | 日志级别（`debug`、`info`、`warning`、`error`） | `info` |
| `XUI_DEBUG` | 启用调试模式 | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | 启用隧道健康监控（探测某个 URL，在连续多次失败后重启 xray；重启会断开所有客户端） | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | 探测请求所经过的代理；将其指向本地 xray 入站，使探测能够测试隧道（例如 `socks5://127.0.0.1:1080`）。留空表示探测仅检查主机连通性 | — |
| `XUI_TUNNEL_HEALTH_URL` | 用于检测隧道健康状况的探测 URL | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | 两次探测之间的间隔 | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | 单次探测的超时时间 | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | 触发重启前的连续失败次数 | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | 两次连续重启之间的最小间隔 | `5m` |
| `NODE_TOKEN_ENCRYPTION` | 节点 API 令牌的静态加密：`off`、`migration` 或 `required`（注意：无 `XUI_` 前缀） | `off` |
| `XUI_NODE_TOKEN_KEY_FILE` | JSON 密钥环（权限 `0600`），包含活动密钥 ID 及其 base64 编码的 32 字节密钥 | `/etc/x-ui/node_token_key.json` |
| `XUI_NODE_TOKEN_KEY` | 单个 base64 编码的 32 字节密钥，仅在无法加载密钥文件时使用 | — |

完整列表请参阅[环境变量参考](https://docs.sanaei.dev/zh/docs/reference/env-vars)。

## 支持的语言

面板界面提供 14 种语言：

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil) · Slovenčina

## 贡献

欢迎贡献。在提交 issue 或 pull request 之前，请阅读[贡献指南](/CONTRIBUTING.md)。

## 特别感谢

- [alireza0](https://github.com/alireza0/)

## 致谢

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (许可证: **GPL-3.0**): _增强的 v2ray/xray 和 v2ray/xray-clients 路由规则，内置伊朗域名，专注于安全性和广告拦截。_
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (许可证: **GPL-3.0**): _此仓库包含基于俄罗斯被阻止域名和地址数据自动更新的 V2Ray 路由规则。_

## 社区工具

社区围绕 3x-ui 构建的工具和集成。

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (许可证: **MIT**): _使用 Terraform / OpenTofu 通过代码管理入站、客户端、面板设置和 Xray 配置。_
- [3X-UI Manager](https://github.com/yukh975/3X-UI-Manager) (许可证: **MIT**): _3x-ui 的原生 Android 客户端 — 仪表板、入站、带二维码分享的客户端、节点以及多面板管理。可在 F-Droid 获取。_

## 支持项目

**如果这个项目对您有帮助，您可以给它一个**:star2:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

## 星标历史

<a href="https://www.star-history.com/?repos=mhsanaei%2F3x-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=mhsanaei/3x-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=mhsanaei/3x-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=mhsanaei/3x-ui&type=date&legend=top-left" />
 </picture>
</a>

<p align="center">
 <a href="https://www.star-history.com/mhsanaei/3x-ui">
  <picture><source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/badge?repo=MHSanaei/3x-ui&type=rank&theme=dark" /><source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/badge?repo=MHSanaei/3x-ui&type=rank" /><img alt="Star History Rank" src="https://api.star-history.com/badge?repo=MHSanaei/3x-ui&type=rank" /></picture> <picture><source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/badge?repo=MHSanaei/3x-ui&type=trending&theme=dark" /><source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/badge?repo=MHSanaei/3x-ui&type=trending" /><img alt="GitHub Trending Repository of the Day" src="https://api.star-history.com/badge?repo=MHSanaei/3x-ui&type=trending" /></picture>
 </a>
</p>
