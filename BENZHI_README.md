# BENZHI_README

## 项目说明
- 项目：benzhi-project-cd10fc50-93a4-421b-b9bb-9ae1ac7796c0
- 项目用途：面向文物建筑保护团队的木构件虫害处置质量治理 HTTP 服务，完整实现勘察基线冻结、可追溯风险分级、分区方案审批、施作偏差整改、效果监测、独立复核、SQLite 审计摘要链与确定性归档校验。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 项目描述
- 项目名称：timber-pest-remediation-ledger
- 项目介绍：面向文物建筑保护团队的木构件虫害处置质量治理服务，以单个处置案件为聚合，从勘察基线冻结、风险判定和方案批准，经现场分区施作与效果监测，完成独立复核并生成可验证归档。
- 项目概述：面向文物建筑保护团队的木构件虫害处置质量治理服务，以单个处置案件为聚合，从勘察基线冻结、风险判定和方案批准，经现场分区施作与效果监测，完成独立复核并生成可验证归档。
- 核心工作流：保护工程师登记木构件虫害案件并冻结勘察基线，系统依据危害证据形成风险分级；负责人提交分区处置方案并通过门禁后记录现场施作，监测期内若指标不达标则进入整改并重新监测，全部分区合格后由独立复核员签发结论，系统最终冻结带摘要链的只读归档。
- 对外接口：仅提供版本化 HTTP JSON API，使用案件命令端点推进唯一状态流程，并通过查询、时间线与归档校验端点读取结果；服务支持 -addr=127.0.0.1:<port>，默认监听 127.0.0.1:19081，PORT 为端口号时绑定 127.0.0.1:<PORT>，不得默认绑定 0.0.0.0。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...

cd /app && GOTOOLCHAIN=local go run ./cmd/server -selftest -selftest-timeout=20s -addr=127.0.0.1:19081

cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh

./build_benzhi_docker.sh benzhi-project-cd10fc50-93a4-421b-b9bb-9ae1ac7796c0-amd64 linux/amd64

./build_benzhi_docker.sh benzhi-project-cd10fc50-93a4-421b-b9bb-9ae1ac7796c0-arm64 linux/arm64

docker run -it benzhi-project-cd10fc50-93a4-421b-b9bb-9ae1ac7796c0-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selftest -selftest-timeout=20s -addr=127.0.0.1:19081`
