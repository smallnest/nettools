# nettools

[![Go Version](https://img.shields.io/badge/Go-1.26-blue.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/baidu/nettools)](https://goreportcard.com/report/github.com/baidu/nettools)
[![Go Reference](https://pkg.go.dev/badge/github.com/baidu/nettools.svg)](https://pkg.go.dev/github.com/baidu/nettools)
[![CI](https://github.com/baidu/nettools/actions/workflows/ci.yml/badge.svg)](https://github.com/baidu/nettools/actions/workflows/ci.yml)

一组百度物理网络黑盒监控方向开发的网络诊断工具，包括：
- **bitflip**: 用于检测大规模物理网络中的丢包和比特翻转错误。
- **bitflip6**: bitflip 的 IPv6 版本，用于 IPv6 网络诊断。
- 其他: 待整理

> 百度系统部出品


## bitflip

![](docs/bitflip.png)

高频 UDP 探测工具，用于网络比特翻转（报文损坏）检测。

**工作原理：** 客户端每秒向服务端发送大量 UDP 报文，服务端原样回显。双端独立检测：

- **Client 端（往返检测）：** 检测全链路往返丢包和 bitflip。若报文在任一方向丢失，计为丢包；若返回报文内容与预期不符，记录发生 bitflip 的五元组。
- **Server 端（单向检测）：** 仅检测 Client→Server 方向的丢包和 bitflip。每个报文携带 Client 上一时间窗口的实际发送计数和起始端口对，Server 据此计算单向丢包率并还原该窗口的全部端口对——实现**单向丢包五元组定位**，无需时钟同步或跟踪 Client 状态。Server 收到未知 Client 的第一个包时自动创建统计实例，无需预配置 `-c`。

通过对比 Client 端和 Server 端的丢包率，可判断丢包发生在**正向路径**（Client→Server）还是**回程路径**（Server→Client）。

### 快速开始

**编译：**
```bash
make build
```

**启动服务端（远程主机）：**
```bash
# 最简方式——自动检测本机 IP
./bitflip

# 指定 IP
./bitflip -r server -s <server_ip> -c <client_ip>
```

**启动客户端（本地主机）：**
```bash
# -c 未设置时自动检测，-s 为必填
./bitflip -r client -s <server_ip>

# 指定 IP
./bitflip -r client -c <client_ip> -s <server_ip>
```

### 命令行参数

| 短参数 | 长参数 | 默认值 | 说明 |
|--------|--------|--------|------|
| `-r` | `--role` | server | 角色：client 或 server |
| `-c` | `--client-addr` | "" | 客户端 IP 地址（为空时自动检测） |
| `-s` | `--server-addr` | "" | 服务端 IP 地址（server 角色为空时自动检测） |
| `-t` | `--tos` | 64 | IP TOS/DSCP 值 |
| `-n` | `--count` | 0 | 最大发送报文数（0 = 无限制） |
| `-d` | `--duration` | 0 | 最大发送时长（0 = 无限制） |
| | `--client-ports` | "43500,43599" | 客户端端口范围 [min,max] |
| | `--server-ports` | "43500,43509" | 服务端端口范围 [min,max] |
| | `--rate` | 5000 | 每个 span 内的发送速率 |
| | `--msglen` | 1024 | 报文载荷大小（不含 32 字节头部） |
| | `--delay` | 3s | 统计处理延迟（等待在途报文） |
| | `--verbose` | false | 丢包时打印详细端口信息（Client 和 Server 均支持） |

### 示例

```bash
# 服务端——自动检测 IP
./bitflip

# 客户端
sudo ./bitflip -r client -s 10.0.0.2

# 客户端——自定义速率和时长
sudo ./bitflip --role client --server-addr 10.0.0.2 --rate 10000 --duration 60s
```

### 比特翻转检测原理

客户端使用 4 种 salt 填充模式发送报文，通过 `seq % 4` 选择：

| 序号 | 填充模式 | 说明 |
|------|----------|------|
| 0 | `0xFF` | 全 1 字节 |
| 1 | `0x00` | 全 0 字节 |
| 2 | `0x5A` | 固定模式 `01011010` |
| 3 | 互补交替 | `0xAAAA` / `0x5555` 交替 16-bit 字 |

服务端使用相同的 4 种 salt 模式验证报文，确保能准确识别哪个字节发生了翻转。

### 报文格式

```
+----------+----------+-----------+---------------+------------------+------------------+----------+
| Magic(8) | Seq(8)   | Ts(8)     | LastSent(4)   | LastSrcPort(2)   | LastDstPort(2)   | Salt(N)  |
+----------+----------+-----------+---------------+------------------+------------------+----------+
```

- **Magic**：8 字节魔数标识
- **Seq**：8 字节序列号
- **Ts**：8 字节纳秒时间戳
- **LastSent**：4 字节上一 span 发送计数
- **LastSrcPort**：2 字节上一 span 起始源端口
- **LastDstPort**：2 字节上一 span 起始目的端口
- **Salt**：N 字节填充数据（用于比特翻转检测）

通过这种精巧的协议设计，Server 端仅需 `(LastSrcPort, LastDstPort, LastSent)` 三个字段加上确定性的 `GetNextPorts` 算法，即可还原上一个 span 中每一个包的端口对——从而实现**单向丢包的五元组级定位**，无需 Server 维护任何 Client 发送状态。

## bitflip6

bitflip 的 IPv6 版本。用法与 bitflip 一致，使用 IPv6 地址：

```bash
# 服务端
./bitflip6

# 客户端
sudo ./bitflip6 -r client -s fd00::2
```

## 测试

```bash
make test
```

## 测试覆盖率

![](coverage.svg)

## 参与贡献

参见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 安全漏洞

参见 [SECURITY.md](SECURITY.md)。

## 许可证

本项目基于 [MIT 许可证](LICENSE) 开源。
