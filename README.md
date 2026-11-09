> [!CAUTION]
> 🔴 **原作者官方说明：[MHSanaei/3x-ui 中文 README](https://github.com/MHSanaei/3x-ui/blob/main/README.zh_CN.md)**
>
> **原有功能、协议配置与基础使用请查阅原作者文档。** 本页只介绍 `limit81995/3x-ui` 的安装、更新和 HTB 端口限速扩展。使用本分支时，请执行下方安装命令。

# 3X-UI · HTB 端口限速扩展

感谢原项目作者 [MHSanaei](https://github.com/MHSanaei) 及所有贡献者。本项目基于 [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui)，通用功能继续跟随上游维护。

本仓库在原项目基础上增加端口限速，保持扩展解耦，持续跟随上游更新。安装和升级使用当前仓库 **`limit81995/3x-ui`**。

## 安装

### Releases 和 Packages 有什么区别

| 使用方式 | 所需发布产物 | 查看位置 |
| --- | --- | --- |
| Linux 脚本安装、命令行和面板更新 | 对应架构的 `x-ui-linux-*.tar.gz` 安装包及 `.sha256` 校验文件 | [本仓库 Releases](https://github.com/limit81995/3x-ui/releases) |
| Docker 安装（GitHub） | `ghcr.io/limit81995/3x-ui` 镜像及对应标签 | [本仓库 Packages](https://github.com/limit81995/3x-ui/packages) |
| Docker 安装（Docker Hub） | `limit81995/3xui` 镜像及对应标签 | [Docker Hub 标签列表](https://hub.docker.com/r/limit81995/3xui/tags) |

**没有 Packages 不影响脚本安装；脚本能否安装取决于 Releases 是否已有所选版本和架构的安装包。** 安装器下载编译好的程序，不会直接把 GitHub 源码编译成服务；只有源码或 Actions 构建产物、没有 Release 安装包时，在线安装仍不可用。缺少版本时不会回退到原作者的安装包。

- 推送符合路径条件的代码到 `main` 后，[Release 工作流](.github/workflows/release.yml) 构建成功才会发布或更新 `dev-latest` 预发布版；只修改 README 不触发发布。
- Docker 镜像由现有 [Docker 工作流](.github/workflows/docker.yml) 构建并发布到 GHCR；配置好 Docker Hub 凭据后，同一次构建也发布到 `limit81995/3xui`。推送到 `main`、手动运行或推送 `v*.*.*` 标签都会触发；推送 `dev` 不自动发布镜像。
- GHCR 使用 GitHub 自动提供的 `GITHUB_TOKEN`，工作流已声明 `packages: write`，无需另建 GitHub Token。首次发布后需将包设为 Public，其他用户才能匿名拉取。
- 不带版本的安装命令和 `x-ui update` 查找最新稳定版；仅有 `dev-latest` 或其他预发布版时，应明确选择开发版或指定已有标签。当前版本标签构建默认也是预发布，维护者确认后需在 Release 页面取消预发布标记，才会成为稳定版候选。

发布流程和最近运行结果见 [GitHub Actions](https://github.com/limit81995/3x-ui/actions)。

### 第一次发布 Docker 镜像

GitHub 仓库存放源码；GitHub Packages（GHCR）和 Docker Hub 存放构建好的容器镜像。两个镜像仓库彼此独立，不会自动同步。Fork 源码或创建 Docker Hub 空仓库，都不会自动产生镜像。本项目的工作流负责构建并向两个仓库上传，你无需在自己的电脑上执行 `docker push`。

1. 在 Docker Hub 创建公开仓库 `limit81995/3xui`。注意这里的名称是 **`3xui`**，GHCR 的名称是 **`3x-ui`**。
2. 在 Docker Hub 账号设置中生成具备仓库 **Read & Write** 权限的 Personal Access Token（PAT）。在 GitHub 本仓库的 **Settings → Secrets and variables → Actions → Secrets** 中配置以下两项；已经配置则无需重复添加。

   | Secret 名称 | 值 |
   | --- | --- |
   | `DOCKER_HUB_USERNAME` | `limit81995`，填写账号名，不是邮箱或完整镜像名 |
   | `DOCKER_HUB_TOKEN` | Docker Hub PAT，需能向 `limit81995/3xui` 写入 |

3. 将当前发布配置合入 `main` 后会自动运行。也可打开 **Actions → Release 3X-UI for Docker → Run workflow**，选择 `main` 后运行。只推到 `dev` 时不会自动发布；想先测试可手动选择 `dev`，得到 `:dev` 镜像。
4. 等待 `Login to GHCR`、`Login to Docker Hub`、`Build and push Docker image` 成功。在该次运行的 Summary 中查看实际发布的镜像标签；多个 CPU 架构需要全部构建完成。
5. 首次发布 GHCR 后，从仓库右侧 **Packages** 打开 `3x-ui`，进入 **Package settings → Change visibility → Public**。工作流通过本仓库的 `GITHUB_TOKEN` 发布会自动关联仓库。Docker Hub 的 Tags 页面也应出现同次构建的标签。

如果缺少任一 Docker Hub Secret，工作流会显示跳过提示并只发布 GHCR；如果填写了错误、过期或权限不足的 Token，登录或推送仍会失败，需修正对应 Secret。凭据只能配置在 Secrets 中，不要放入源码或 README。

| 操作 | 两个仓库使用的标签 |
| --- | --- |
| 推送或合并到 `main`；手动运行 `main` | `main`、`latest` |
| 推送 `dev` | 不自动发布 |
| 手动运行 `dev` | `dev`，不更新 `main` 或 `latest` |
| 推送版本标签，例如 `v3.11.0` | `v3.11.0`、`3.11.0`，不更新 `latest` |

这里的 **`latest` 是 `main` 的滚动镜像，不表示已经发布稳定版**。Docker Hub 展示的 `docker push limit81995/3xui:tagname` 是上传命令示例，`tagname` 是需要替换的标签占位符，不会因创建仓库而自动生成。

发布成功并公开后，可在任意安装了 Docker 的机器验证：

```bash
docker pull limit81995/3xui:main
docker pull ghcr.io/limit81995/3x-ui:main
```

两者是同次构建的两个下载入口，实际部署任选一个即可。若刚发布的是 `dev`，将命令中的 `main` 替换为 `dev`。脚本安装仍使用 Releases，与这两个镜像仓库独立。

### 脚本安装（不使用 Docker）

在 Linux 服务器以 root 身份执行。安装来自 `main` 的滚动开发版（需 Releases 已发布 `dev-latest`，这是预发布版）：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/limit81995/3x-ui/main/install.sh) dev-latest
```

安装最新稳定版（需本仓库已有非预发布 Release）：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/limit81995/3x-ui/main/install.sh)
```

安装指定版本时，将第一条命令末尾的 `dev-latest` 换成本仓库已发布的版本标签。

脚本下载编译好的程序，服务器无需安装 Go、Node.js 或编译器；同时安装 tc 依赖并尝试加载限速所需的内核模块。安装完成后会显示面板地址、随机账号和密码，运行 `x-ui` 可查看或重置登录信息。

已有本仓库的完整安装包、同名 `.sha256` 文件和配套 `install.sh` 时，也可本地安装（以下以 amd64 为例，仍需联网安装系统依赖）：

```bash
sudo env XUI_INSTALL_ARCHIVE="$PWD/x-ui-linux-amd64.tar.gz" bash ./install.sh
```

### Docker 安装（host 模式）

以下示例使用 `ghcr.io/limit81995/3x-ui:main` 镜像；需先完成上面的首次发布步骤，并确保镜像公开。也可将命令末尾的镜像地址换成 `limit81995/3xui:main`，从 Docker Hub 下载。

先在 Linux 宿主机加载限速模块，再启动容器：

```bash
sudo modprobe -a ifb sch_htb sch_fq_codel sch_ingress cls_flower act_mirred
mkdir -p db cert acme

docker run -d \
  --name 3xui \
  --network host \
  --cap-add NET_ADMIN \
  --cap-add NET_RAW \
  --restart unless-stopped \
  -v "$PWD/db:/etc/x-ui" \
  -v "$PWD/cert:/root/cert" \
  -v "$PWD/acme:/root/.acme.sh" \
  ghcr.io/limit81995/3x-ui:main
```

host 模式直接使用宿主机端口，无需 `-p` 映射。其他电脑通过服务器 IP 访问，面板和入站需监听可访问的地址，并放行相应防火墙端口。运行 `docker logs 3xui` 查看首次生成的登录信息，或用 `docker exec -it 3xui x-ui` 打开管理菜单。

镜像已包含 tc，内核模块由宿主机提供，重启后也需保持可用。仓库中的 `docker-compose.yml` 使用本地源码构建，并采用桥接网络、默认只映射面板端口；如使用它，需另行配置入站端口映射。上面的 host 模式命令可直接监听宿主机上的入站端口。

## 更新

已安装本分支的脚本版本可执行：

```bash
sudo x-ui update       # 更新到本仓库最新稳定版
sudo x-ui update-dev   # 更新到本仓库 dev-latest 开发版
```

两条命令按所需版本择一执行；没有稳定版时使用 `update-dev`。网页首页点击面板版本号也可更新，版本检查和下载均使用本仓库。更新前请备份数据库；更新保留原数据库。从上游版本切换时，先使用上面的本仓库安装脚本安装对应版本，让程序与管理脚本一并切换到本仓库来源。

Docker 更新时，在原数据目录执行：

```bash
docker pull ghcr.io/limit81995/3x-ui:main
docker stop 3xui
docker rm 3xui
```

然后重新执行上面的 `docker run` 命令，沿用 `db`、`cert`、`acme` 挂载目录，保留数据。

## 相比原项目增加了什么

| 扩展 | 说明 |
| --- | --- |
| 端口限速 | 使用 Linux tc、IFB 和 HTB 队列整形，按入站端口限制带宽，超出即时速率的报文先排队发送。 |
| 上传、下载分别限速 | 一个 Mbps 数值分别约束两个方向；同一端口所有用户、TCP/UDP 和 IPv4/IPv6 流量共享各方向的额度。 |
| 网页设置 | 新增、编辑入站时可设置“端口限速”，入站列表和详情显示该值。范围为整数 `0–10000`，`0` 表示不限速。 |
| API 支持 | 新增和编辑入站接受 `speedLimit` 字段，查询接口返回该字段。 |
| 数据库保存 | 限速配置随入站保存，重启后恢复；旧入站默认不限速。 |
| 自动校准 | 保存后立即尝试应用，面板启动时及每分钟读取实际 tc 规则，修复被修改的速率和丢失的规则。 |
| 自动清理 | 取消限速、禁用、删除、到期、用尽流量或更换端口后，清理不再需要的规则。 |
| 本仓库安装与更新 | 安装、命令行更新、菜单脚本及面板版本检查使用本仓库，构建发布沿用原项目流程。 |

### 限速怎么理解

设置 `20`，表示该端口所有用户**上传合计约 20 Mbps、下载合计约 20 Mbps**，两个方向分别计算，不是每个用户各 20 Mbps。

`20 Mbps` 理论上约为 `2.5 MB/s`。这是带宽上限，实际下载速度还受协议开销、网络线路和对端影响，不保证达到上限。

### API 怎么设置

在原有完整入站请求中增加字段：

```json
{"speedLimit": 20}
```

- 新增：`POST /panel/api/inbounds/add`。
- 编辑：`POST /panel/api/inbounds/update/:id`。
- 取消限速：传入 `"speedLimit": 0`；编辑时省略该字段也会恢复为 `0`。

接口沿用原项目认证方式；配置了面板访问路径时，需在接口地址前加上该路径。

### 使用条件

- 限速功能仅支持 Linux，需具备 tc、上述内核模块和 `NET_ADMIN` 权限；Docker 即使关闭 Fail2ban 也需保留该权限。
- 默认作用于有 IPv4/IPv6 默认路由的网卡；多出口可通过 `XUI_TC_INTERFACES=eth0,eth1` 指定实际承载入站流量的接口，避免重复选中同一条流量路径。
- 同一节点上共用端口的启用入站，必须配置相同限速值；回环地址、Unix socket 和 TUN 入站不支持此设置。
- 远端入站由实际承载流量的节点限速，主面板与节点都需升级到本分支支持该功能的版本。
- 面板管理自己的 IFB 接口和 tc 规则，保留网卡原有根队列及其他分类规则；同一网络命名空间只运行一个限速面板。
- 保存成功表示配置已写入数据库；实际应用失败会记录 `tc speed limit: reconciliation failed`，并在后续校准中重试。

## 原有功能与官方文档

VLESS、VMess、Trojan、Shadowsocks、WireGuard、AmneziaWG、TUIC、Hysteria2、多节点、订阅、数据库、证书和 API 等功能沿用原项目，具体配置请查看：

- [原作者中文 README](https://github.com/MHSanaei/3x-ui/blob/main/README.zh_CN.md)
- [原作者完整文档](https://docs.sanaei.dev/zh/)
- [原作者仓库](https://github.com/MHSanaei/3x-ui)

官方文档中的安装、更新命令和镜像地址用于官方版本；保留本扩展时，请使用本页给出的本仓库入口。扩展问题请提交到 [本仓库 Issues](https://github.com/limit81995/3x-ui/issues)。

## 维护与许可证

保持“原项目基础 + 最小必要扩展”，复用上游安装、构建和发布流程；限速逻辑集中在 `internal/tclimit` 等扩展模块，减少以后合并上游时的冲突。

本项目沿用原项目的 [GPL-3.0 许可证](LICENSE)。感谢 [alireza0](https://github.com/alireza0)、[Xray-core](https://github.com/XTLS/Xray-core) 及原项目所有贡献者；原有组件的版权和许可证继续保留。
