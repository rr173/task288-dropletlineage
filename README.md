# 微流控液滴分裂谱系复核台（task288-dropletlineage）

基于 Go 的后端服务：微流控研究者导入高速成像的帧事件与液滴观测（位置、半径、荧光标记），服务执行跨帧跟踪、分裂/合并守恒校验与遮挡裁决，最终发布不可变的液滴谱系版本。

## 业务闭环

创建批次 → 导入帧与液滴观测（帧序单调、坐标校验、幂等）→ 跨帧跟踪（最近邻 + 速度外推，检测遮挡断裂）→ 申报分裂/合并事件并执行守恒校验（体积 + 荧光标记）→ 研究者裁决确认/否决 → 创建并冻结谱系版本 → 批次发布/封存。

## 标准命令

```bash
# 构建 / 静态检查 / 测试
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...

# 启动服务
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/task288 --addr :8080 --db dropletlineage.db

# 自检（不启动长驻服务，验证持久化与重启恢复）
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/task288 --smoke-test
```

## API 入口（/api 前缀）

| 能力 | API |
| --- | --- |
| 批次 | `POST/GET /api/batches`、`GET /api/batches/{id}`、`POST /api/batches/{id}/transition` |
| 帧 | `POST/GET /api/batches/{id}/frames`、`GET /api/batches/{id}/frames/{seq}` |
| 观测 | `POST /api/batches/{id}/frames/{seq}/observations`、`GET /api/batches/{id}/observations`、`PUT /api/observations/{id}/status` |
| 跟踪 | `POST /api/batches/{id}/track`、`GET /api/batches/{id}/tracks`、`GET /api/tracks/{id}` |
| 事件 | `POST/GET /api/batches/{id}/events`、`POST /api/events/{id}/check`、`PUT /api/events/{id}/confirm|reject` |
| 守恒 | `POST /api/batches/{id}/conservation-check`、`GET /api/batches/{id}/conservation-results` |
| 版本 | `POST/GET /api/batches/{id}/versions`、`GET /api/versions/{id}`、`POST /api/versions/{id}/publish|derive` |
| 统计 | `GET /api/batches/{id}/stats`、`GET /api/health` |

## 核心不变量

- 帧序单调递增，重复导入幂等，跳号补旧帧被拒绝。
- 通道坐标越界、半径/强度非法、同帧同液滴重复被拒绝。
- 分裂守恒：父滴体积 ≈ Σ 子滴体积（5% 容差），荧光标记不凭空增加（淬灭下限 80%）。
- 冻结版本不可修改，只能派生新版本（替代链固化）。
- 批次 `review→published` 前置：必须存在冻结谱系版本。
