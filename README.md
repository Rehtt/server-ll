## server-ll · 网卡流量记录工具

[![Go Version](https://img.shields.io/badge/go-1.24%2B-00ADD8?logo=go)](https://go.dev/) [![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](./LICENSE)

一个基于 SQLite 的本地网卡流量采集与聚合展示工具。适合通过系统 crontab 定时运行，按天/月/年汇总并在终端友好展示。

> 模块路径：`github.com/Rehtt/server-ll`

### 特性
- **无侵入采集**：基于 `gopsutil` 读取各网卡累计字节数，无需管理员权限。
- **定时记录**：配合 crontab/任务计划，按执行频率记录增量流量。
- **聚合展示**：支持按年(`y`)/月(`m`)/日(`d`)汇总。
- **网卡筛选**：支持包含或排除指定网卡名，内置 Docker 网卡过滤。
- **轻量持久化**：单文件 SQLite，自动迁移，无外部依赖。
- **数据清理**：提供 Docker 网卡数据清理功能。

### 环境
- Go 1.24+（见 `go.mod`）
- Linux / macOS（已在 Darwin 上验证）

### 安装
- 使用 Go 安装（推荐）：
```bash
go install github.com/Rehtt/server-ll@latest
```

- 从源码构建：
```bash
git clone https://github.com/Rehtt/server-ll.git
cd server-ll
go build -o server-ll ./
```

### 快速开始
第一次运行用于初始化"基线"（不会产生统计记录），第二次及之后运行会记录增量：
```bash
server-ll -f /path/to/db
# 再运行一次（或等待下一次定时任务），将把与上次之间的增量写入数据库
```

### 命令行参数

#### 全局参数
| 参数 | 说明 | 默认值 |
| --- | --- | --- |
| `-f` | SQLite 数据库文件路径 | `$HOME/.local/var/server-ll/db` |
| `-l` | 时区设置：`auto`/`local`/`utc`/`Asia/Shanghai` 等 | `auto` |
| `-i` | 仅包含的网卡名，逗号分隔（如 `en0,eth0`） | 空 |
| `-e` | 排除的网卡名，逗号分隔（如 `lo,lo0`） | 空 |
| `-exclude-docker` | 排除 Docker 相关网卡（`docker*`、`br-*`、`veth*`） | false |

注意：`-i`/`-e` 的对象是"网卡名"（interface name），不是端口号。

#### 子命令

**记录流量（默认命令）**
```bash
server-ll [全局参数]
```

**查看历史流量**
```bash
server-ll show [参数]
```

| 参数 | 说明 | 默认值 |
| --- | --- | --- |
| `-s` | 展示模式：`y`/`m`/`d`（年/月/日） | `d` |

**清理 Docker 网卡数据**
```bash
server-ll prune
```
### 使用示例

#### 基本使用
- 仅记录（配合 crontab 定时执行）：
```bash
server-ll -f /usr/local/var/server-ll/db
```

- 按日聚合展示（默认模式）：
```bash
server-ll -f /usr/local/var/server-ll/db show
```

- 按月聚合展示：
```bash
server-ll -f /usr/local/var/server-ll/db show-s m
```

#### 网卡筛选
- 仅统计 `en0` 与 `utun2`：
```bash
server-ll -f /usr/local/var/server-ll/db -i en0,utun2 show
```

- 排除回环网卡：
```bash
server-ll -f /usr/local/var/server-ll/db -e lo,lo0 show
```

- 排除 Docker 相关网卡：
```bash
server-ll -f /usr/local/var/server-ll/db -exclude-docker show
```

#### 数据清理
- 清理所有 Docker 网卡数据：
```bash
server-ll -f /usr/local/var/server-ll/db prune
```

示例输出：
```text
time        name                     recv        sent
----        ----                     ----        ----
2025-08-12  en0                      123.4MB     45.6MB
2025-08-12  lo0                      12.0MB      1.2MB
---
2025-08-13  en0                      2.3GB       600MB
```

### 定时任务（crontab）
以每分钟记录一次为例（推荐）：
```bash
mkdir -p /usr/local/var/server-ll
install -m 0755 ./server-ll /usr/local/bin/server-ll
crontab -e
```

在 crontab 中加入：
```cron
* * * * * /usr/local/bin/server-ll -f /usr/local/var/server-ll/db >> /usr/local/var/server-ll/cron.log 2>&1
```

说明：
- 第一次运行仅初始化基线；从第二次开始每次都会记录与上次之间的增量。
- 记录频率决定统计的时间粒度（例如每分钟记录，则每分钟一条增量）。

macOS 也可考虑使用 `launchd` 管理守护，但 crontab 同样可用。

### 数据存储与原理
- 数据库为 SQLite 单文件，由 `-f` 指定路径决定；程序会在路径不存在时自动创建目录与文件。
- 自动迁移的表：
  - `DB(time,timestamp; name,text,index; recv,bigint; sent,bigint)`：每次记录的"增量"数据（相对上次基线的变化量）。
  - `KeyValue(key,text,unique; value,text)`：存储上次各网卡的累计值"基线"（`key = "historical_record"`，`value` 为 JSON）。
- 采集原理：每次运行读取 `gopsutil/net.IOCounters(true)` 的累计字节数，与上次基线对比得到增量；若检测到累计值回绕/变小（如系统重启或计数溢出），则将当次视为新基线并直接记录当前读数。
- 展示原理：使用 SQLite `strftime` 对记录时间分组聚合，再按所选时区格式化输出时间字符串。
- Docker 网卡识别：自动识别 `docker*`、`br-*`、`veth*` 模式的网卡名。

### 常见问题（FAQ）
- 为什么第一次没看到统计数据？
  - 第一次仅写入"基线"，不产生增量；第二次运行后才会看到统计数据。
- 如何获取可用的网卡名？
  - Linux：`ip link`、`ifconfig`；macOS：`ifconfig`（如 `en0`、`lo0`、`utunX`）。
- 时区设置有哪些选项？
  - `auto`：自动检测系统时区（默认）；`local`：本地时区；`utc`：UTC 时区；或指定 IANA 时区名称如 `Asia/Shanghai`。
- 数据会不会丢？
  - 每次运行均落库（事务写入），若进程中断，已提交的数据不受影响。
- 如何清理 Docker 网卡数据？
  - 使用 `server-ll prune` 命令，会列出所有 Docker 网卡并确认后删除。

### 注意与限制
- 确保 `-f` 指向的目录可写；建议放置在本地持久盘路径。
- 若系统累计计数器溢出或重置（如重启、驱动重载、容器重建），程序会自动将当次视为新基线，避免出现负值；该次记录会以当前读数入库。
- 本项目面向 Linux/macOS；Windows 未覆盖默认路径规范。
- Docker 网卡数据清理操作不可逆，请谨慎使用。

### 贡献
欢迎提交 Issue / PR 改进功能与文档。提交 PR 前请确保通过构建与基本自测。

### 许可证
本项目使用 MIT License，详见 [`LICENSE`](./LICENSE)。



