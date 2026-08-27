# BENZHI 评测说明

基于 Go 实现的微流控液滴谱系复核后端服务，一款后端服务，完成液滴跨帧跟踪、分裂/合并体积与荧光标记守恒校验及不可变谱系版本发布。

## 启动

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/task288 --addr :8080 --db dropletlineage.db
```

## 自检（不启动长驻服务）

```bash
go run ./cmd/task288 --smoke-test
```

自检会真实跑完导入、跟踪、守恒、版本发布场景，关闭数据库后重开验证持久化，以 0 退出。

## 构建门禁

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...
go run ./cmd/task288 --smoke-test
```

## HTTP API

路由统一 `/api` 前缀：批次创建/推进、帧与液滴观测导入（幂等）、跨帧跟踪、遮挡候选、分裂/合并守恒校验、事件裁决、谱系版本创建/冻结/派生、统计与健康检查。

## 持久化

SQLite（modernc.org/sqlite 纯 Go 驱动），保存批次、帧、观测、轨迹、谱系事件、守恒校验与版本快照；重启后恢复未完成谱系，帧序与观测序号幂等，冻结版本保留完整事件证据。
