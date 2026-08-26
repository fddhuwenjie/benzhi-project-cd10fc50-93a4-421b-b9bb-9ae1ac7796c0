# 木构件虫害处置质量治理服务

本项目为文物建筑保护团队提供版本化 HTTP JSON API，以单个虫害处置案件为聚合，覆盖冻结前构件批量校订、勘察基线冻结、风险分级、方案退回与版本化审批、多分区原子施作、偏差整改、效果监测、独立复核及可验证归档。

数据默认保存在本地 SQLite。所有案件写操作均使用 `request_id` 保证幂等，并使用 `expected_revision` 防止陈旧覆盖；审计事件通过 SHA-256 摘要串联。服务默认仅监听 `127.0.0.1:19081`，可通过 `-addr` 修改，或通过只包含端口号的 `PORT` 绑定到对应回环端口。

## 构建

```bash
go build ./cmd/server
```

## 运行

```bash
go run ./cmd/server -addr=127.0.0.1:19081 -data=./data/ledger.db
```

健康检查地址为 `GET /healthz`。API 统一位于 `/api/v1`，写请求使用 JSON 字段 `request_id`、`actor_id` 和 `expected_revision`。

新增流程入口包括：

- `POST /api/v1/cases/{case_id}/baseline/components/revise`：冻结前批量新增、替换或移除构件。
- `POST /api/v1/cases/{case_id}/plan/return`：退回当前待批方案并记录需修订分区。
- `POST /api/v1/cases/{case_id}/executions/batch`：一次原子登记两个及以上分区施作。
- `GET /api/v1/cases/{case_id}/monitoring/summary`：读取分区监测阻塞项、下一动作及复核就绪状态。
- `GET /api/v1/cases/{case_id}/archive/verification-report`：读取归档事件链、清单和规范化事实的分层诊断。

## 测试与自检

```bash
go test ./...
go run ./cmd/server -selftest -selftest-timeout=20s -addr=127.0.0.1:19081
```

自检会创建临时 SQLite 数据库、启动真实回环 HTTP 服务、完成一条从建案到归档校验的完整业务流程，然后主动关闭服务并退出。
