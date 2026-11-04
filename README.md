# 3X-UI · HTB 端口限速扩展

感谢原项目作者 [MHSanaei](https://github.com/MHSanaei) 及所有贡献者。本项目基于 [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui)，原有功能和基础使用说明请参阅 **[上游中文 README](https://github.com/MHSanaei/3x-ui/blob/main/README.zh_CN.md)**。

本仓库在原项目基础上增加端口限速，保持扩展解耦，持续跟随上游更新。安装和升级使用当前仓库 **`limit81995/3x-ui`**。

## 安装

脚本安装需要本仓库已发布对应的 [Release 安装包](https://github.com/limit81995/3x-ui/releases)，Docker 安装需要本仓库已发布对应镜像。仅推送代码到 `dev` 不会自动发布这些产物；缺少版本时不会回退到上游安装包。

### 脚本安装（不使用 Docker）

在 Linux 服务器以 root 身份执行，安装最新稳定版：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/limit81995/3x-ui/dev/install.sh)
```

安装开发版（需已发布 `dev-latest`）：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/limit81995/3x-ui/dev/install.sh) dev-latest
```

安装指定版本时，将末尾的 `dev-latest` 换成本仓库已发布的版本标签。

脚本下载编译好的程序，服务器无需安装 Go、Node.js 或编译器；同时准备 tc 依赖及限速所需的内核模块。安装完成后会显示面板地址、随机账号和密码，运行 `x-ui` 可查看或重置登录信息。

已有本仓库的完整安装包、同名 `.sha256` 文件和配套 `install.sh` 时，也可本地安装（以下以 amd64 为例，仍需联网安装系统依赖）：

```bash
sudo env XUI_INSTALL_ARCHIVE="$PWD/x-ui-linux-amd64.tar.gz" bash ./install.sh
```

### Docker 安装（host 模式）

以下示例使用 `ghcr.io/limit81995/3x-ui:dev` 镜像；需先通过现有 [Docker 工作流](.github/workflows/docker.yml) 在 `dev` 分支手动构建并发布该标签。

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
  ghcr.io/limit81995/3x-ui:dev
```

host 模式直接使用宿主机端口，无需 `-p` 映射。其他电脑通过服务器 IP 访问，面板和入站需监听可访问的地址，并放行相应防火墙端口。运行 `docker logs 3xui` 查看首次生成的登录信息，或用 `docker exec -it 3xui x-ui` 打开管理菜单。

镜像已包含 tc，内核模块由宿主机提供，重启后也需保持可用。官方 `docker-compose.yml` 保持原样；上面的命令用于 host 模式部署。

## 更新

已安装本分支的脚本版本可执行：

```bash
sudo x-ui update       # 更新到本仓库最新稳定版
sudo x-ui update-dev   # 更新到本仓库 dev-latest 开发版
```

两条命令按所需版本择一执行。也可在网页首页点击面板版本号更新。更新保留原数据库；从上游版本切换时，先使用上面的本仓库安装脚本安装对应版本，让程序与管理脚本一并切换到本仓库来源。

Docker 更新时，在原数据目录执行：

```bash
docker pull ghcr.io/limit81995/3x-ui:dev
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

## 维护原则

上游持续维护期间，保持“原项目基础 + 最小必要扩展”，减少合并冲突，不另建重复的安装、构建和发布体系；确认上游停更后再评估维护策略。具体约定见 [项目规范](.agents/skills/project-rules/SKILL.md)。

本项目沿用原项目的 [GPL-3.0 许可证](LICENSE)。
