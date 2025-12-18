# 分布式定时任务系统 (Distributed Cron) - Go 语言实现

本项目是一个使用 Go 语言构建的分布式定时任务系统，采用了 Master-Worker 架构，通过 Leader 选举实现高可用，并提供一个基于 Vue 3 的前端界面进行管理。本项目是在一次进阶 Go 语言学习路径中完成的毕业项目。

## ✨ 主要功能

- **分布式架构**: 采用 Master-Worker 架构，实现了调度与执行分离，确保了系统的高可扩展性。
- **高可用性**: Master 节点通过 etcd 进行 Leader 选举。当活跃 Leader 节点宕机时，其他 Master 节点会自动接管调度任务。
- **任务持久化**: 所有任务定义均持久化存储在 etcd 中，确保系统重启后任务信息不丢失。
- **多种执行器**: 支持两种任务执行方式：`HTTP` 回调（发起 HTTP 请求）和 `Shell` 命令执行。
- **健壮的任务控制**:
  - **并发控制**: 支持 "Forbid" 并发策略，通过分布式锁防止同一任务的多个实例并发执行。
  - **失败重试**: 为任务执行提供了可配置的重试策略和退避机制，以应对瞬时错误。
- **全栈可观测性**:
  - **结构化日志**: 所有组件均使用 `slog` 输出结构化 JSON 日志，便于机器解析和查询。
  - **指标监控**: 通过 `/metrics` 端点暴露关键操作指标（API 请求、任务执行次数、Leader 状态）供 Prometheus 收集。
  - **分布式追踪**: 集成 OpenTelemetry，实现 Master-Worker 之间 API 请求和任务执行的端到端追踪。
- **Web UI**: 提供一个简洁的 Vue 3 + TypeScript 前端管理界面，用于列表展示、创建、删除任务以及查看任务执行历史。

## 🏛️ 系统架构

系统主要由以下三个核心组件构成：

- **Master 节点**: 系统的“大脑”。
  - 任何时刻只有一个 Master 节点作为 **Leader** 活跃。
  - Leader 负责运行调度器，计算任务的执行时间，并通过 gRPC 将任务派发给可用的 Worker 节点。
  - 同时提供 HTTP API 供用户管理任务。
- **Worker 节点**: 系统的“双手”。
  - 可以运行多个 Worker 节点以扩展任务执行能力。
  - 每个 Worker 启动时会向 etcd 注册自己的存在。
  - 监听来自 Master 的 gRPC 命令，负责实际执行任务（HTTP 或 Shell）。
- **etcd**: 系统的“中央神经系统”。
  - **服务发现**: Worker 节点在此注册，Master 节点通过 etcd 发现可用的 Worker。
  - **Leader 选举**: Master 节点在此竞争分布式锁以成为 Leader。
  - **数据存储**: 持久化存储所有任务的定义和执行历史记录。
  - **分布式锁**: 供 Worker 节点用于实现“禁止并发”的任务并发控制策略。

## 🛠️ 技术栈

- **后端**: Go
- **协调与存储**: etcd
- **内部通信**: gRPC
- **前端**: Vue 3, TypeScript, Vite, Bootstrap 5
- **可观测性**: OpenTelemetry, Prometheus (客户端)
- **主要 Go 库**: `etcd/client/v3`, `robfig/cron/v3`, `slog`, `spf13/viper`, `go-playground/validator`

## 🚀 启动与运行

**前置条件**:
- Go (1.21+)
- Node.js & npm
- Docker (用于运行 etcd)

**1. 启动 etcd**
```bash
docker run -d -p 2379:2379 --name etcd-gcr-v3.5.0 gcr.io/etcd-development/etcd:v3.5.0 /usr/local/bin/etcd --advertise-client-urls http://0.0.0.0:2379 --listen-client-urls http://0.0.0.0:2379
```

**2. 运行后端服务**
请在项目根目录 (`distributed-cron`) 下打开两个独立的终端。

*终端 1: 启动 Master 节点*
```bash
go run ./cmd/master/main.go
```

*终端 2: 启动 Worker 节点*
```bash
go run ./cmd/worker/main.go
```

**3. 运行前端服务**
打开第三个终端。

```bash
cd frontend
npm install # 如果之前没有运行过，或遇到问题，请重新运行
npm run dev
```
现在，打开你的浏览器并访问 `http://localhost:5173` (或你的终端中 Vite 提示的地址)。

## ⌨️ API 使用示例

**创建一个新的 Shell 任务 (每 10 秒执行一次)**:
```bash
curl -X POST -H "Content-Type: application/json" -d '{
  "name": "my-first-shell-job",
  "cron_expr": "*/10 * * * * *",
  "executor_type": "shell",
  "executor": {
    "command": "echo Hello from a distributed cron job on $(date)"
  },
  "concurrency_policy": "Forbid",
  "retry_policy": {
    "max_retries": 3,
    "backoff": "5s"
  }
}' http://localhost:8080/jobs/
```

**创建一个新的 HTTP 任务 (每 15 秒执行一次)**:
```bash
curl -X POST -H "Content-Type: application/json" -d '{
  "name": "my-http-job",
  "cron_expr": "*/15 * * * * *",
  "executor_type": "http",
  "executor": {
    "url": "https://httpbin.org/get",
    "method": "GET"
  }
}' http://localhost:8080/jobs/
```

**获取所有任务列表**:
```bash
curl http://localhost:8080/jobs/
```

**删除任务**:
```bash
curl -X DELETE http://localhost:8080/jobs/my-first-shell-job
```

**获取任务执行历史**:
```bash
curl http://localhost:8080/jobs/my-first-shell-job/history
```

## 📜 许可证

本项目采用 MIT 许可证 - 详情请参阅 [LICENSE](LICENSE) 文件。