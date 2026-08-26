# task275-inkorder 法庭文书笔迹笔顺证据复核台

面向司法文书鉴定研究者的笔迹物证分析后端服务：以墨迹交叉覆盖、笔压停顿与笔画方向为证据，
重建手写笔迹的书写先后顺序，并在多份扫描件之间做一致性裁决，最终发布不可变鉴定快照。

## 核心概念

- **鉴定批次**：一次笔顺复核任务的聚合，状态机 `importing → pending_rebuild → pending_review → published → sealed`。
- **扫描层**：同一文书的扫描件，首层为基准层，其余层经标尺校正（scale/offset 相似变换）对齐到基准坐标。
- **笔迹片段**：一段笔画墨迹，状态 `raw → calibrated → artifact → excluded`。
- **观测点**：交叉覆盖、笔压停顿、笔画方向三类观测。
- **交叉覆盖证据**：交叉点处后写笔画覆盖先写笔画，形成偏序边。
- **笔顺候选**：重建出的书写顺序，状态 `generated → consistent/conflict → confirmed/rejected`。
- **鉴定快照**：冻结候选结论与扫描基准，状态 `draft → shared → frozen → superseded`。

## 快速开始

```bash
# 构建与自检
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...
go run ./cmd/task275 --smoke-test

# 启动服务
go run ./cmd/task275 --addr :8080 --db task275-inkorder.db
```

## API 入口（前缀 /api）

- 批次：`POST /api/batches`、`GET /api/batches`、`GET /api/batches/{id}`、
  `POST /api/batches/{id}/rebuild`、`/review`、`/publish`、`/seal`
- 扫描层：`POST /api/batches/{id}/layers`、`GET /api/batches/{id}/layers`、`POST /api/layers/{id}/ruler`
- 片段：`POST /api/batches/{id}/fragments`、`GET /api/batches/{id}/fragments`、`GET /api/fragments/{id}`、
  `POST /api/batches/{id}/calibrate`、`POST /api/fragments/{id}/calibrate`、`/artifact`、`/exclude`
- 观测点：`POST /api/fragments/{id}/observations`、`GET /api/fragments/{id}/observations`
- 交叉与笔顺：`POST /api/batches/{id}/crossings`、`POST /api/batches/{id}/crossings/auto`、
  `GET /api/batches/{id}/crossings`、`POST /api/batches/{id}/candidates`、`GET /api/batches/{id}/candidates`、
  `GET /api/candidates/{id}`、`POST /api/candidates/{id}/confirm`、`/reject`、
  `POST /api/candidates/{id}/objections`、`GET /api/candidates/{id}/objections`
- 快照：`POST /api/batches/{id}/snapshots`、`GET /api/batches/{id}/snapshots`、
  `POST /api/snapshots/{id}/share`、`/freeze`、`/supersede`
- 其它：`GET /api/health`、`GET /api/stats`

## 示例流程

```bash
# 1. 创建批次
curl -X POST localhost:8080/api/batches -d '{"case_ref":"CASE-001","title":"签名笔顺复核"}'

# 2. 注册基准层与对照层
curl -X POST localhost:8080/api/batches/1/layers -d '{"name":"L1","width":1000,"height":800,"is_base":true}'
curl -X POST localhost:8080/api/batches/1/layers -d '{"name":"L2","width":1000,"height":800}'

# 3. 设置 L2 标尺（2 对同名点）
curl -X POST localhost:8080/api/layers/2/ruler -d '{"base_points":[{"x":0,"y":0},{"x":100,"y":0}],"layer_points":[{"x":12,"y":-8},{"x":117,"y":-8}]}'

# 4. 导入片段并校正
curl -X POST localhost:8080/api/batches/1/fragments -d '{"layer_id":1,"label":"A","start_x":100,"start_y":100,"end_x":300,"end_y":100,"pressure":0.3}'
curl -X POST localhost:8080/api/batches/1/fragments -d '{"layer_id":1,"label":"B","start_x":200,"start_y":40,"end_x":200,"end_y":200,"pressure":0.62}'
curl -X POST localhost:8080/api/batches/1/calibrate

# 5. 自动判定交叉覆盖并重建候选
curl -X POST localhost:8080/api/batches/1/crossings/auto -d '{"layer_id":1,"first_fragment_id":1,"second_fragment_id":2}'
curl -X POST localhost:8080/api/batches/1/rebuild
curl -X POST localhost:8080/api/batches/1/candidates
curl -X POST localhost:8080/api/candidates/1/confirm

# 6. 冻结快照并发布封存
curl -X POST localhost:8080/api/batches/1/snapshots -d '{"candidate_id":1,"note":"复核结论"}'
curl -X POST localhost:8080/api/snapshots/1/share
curl -X POST localhost:8080/api/snapshots/1/freeze
curl -X POST localhost:8080/api/batches/1/publish
curl -X POST localhost:8080/api/batches/1/seal
```

## 目录结构

```
cmd/task275/         入口（--addr/--db/--smoke-test）
internal/model/      实体与状态机
internal/store/      SQLite 持久化（9 张表）
internal/geometry/   标尺校正与交叉覆盖几何判定
internal/order/      偏序图、笔顺重建与一致性校验
internal/consensus/  多扫描层证据聚合、伪影判定与冲突检测
internal/service/    业务编排（事务与状态机）
internal/httpapi/    HTTP 层（路由前缀 /api）
internal/smoke/      --smoke-test 自检
```

## 持久化与重启恢复

SQLite 单文件持久化，`--smoke-test` 会创建实体后关闭并重新打开数据库验证状态恢复；
片段导入幂等（标签唯一），候选裁决带版本号乐观锁，冻结快照不可修改。
