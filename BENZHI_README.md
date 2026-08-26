# BENZHI 评测说明

基于 Go 实现的法庭文书笔迹笔顺证据复核后端服务，一款后端服务，完成笔迹片段导入与扫描层标尺校正、墨迹交叉覆盖书写先后判定与笔顺候选重建、多扫描层一致性裁决与不可变鉴定快照发布。

## 启动

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/task275 --addr :8080 --db task275-inkorder.db
```

## 自检（不启动长驻服务）

```bash
go run ./cmd/task275 --smoke-test
```

`--smoke-test` 会真实创建鉴定批次、扫描层与 A/B/C 三片段并校正、登记交叉覆盖证据与停顿观测、重建并确认笔顺候选、冻结鉴定快照、发布并封存批次，随后关闭数据库并重开同一文件，验证批次/候选/快照状态与片段数量、几何交叉点位置全部恢复，以 0 退出码结束。

## 构建门禁

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/task275 --smoke-test
```

## HTTP API（前缀 /api）

批次：`POST /api/batches`、`GET /api/batches`、`GET /api/batches/{id}`、`POST /api/batches/{id}/rebuild`、`/review`、`/publish`、`/seal`
扫描层：`POST /api/batches/{id}/layers`、`GET /api/batches/{id}/layers`、`POST /api/layers/{id}/ruler`
片段：`POST /api/batches/{id}/fragments`、`GET /api/batches/{id}/fragments`、`GET /api/fragments/{id}`、`POST /api/batches/{id}/calibrate`、`POST /api/fragments/{id}/calibrate`、`/artifact`、`/exclude`
观测点：`POST /api/fragments/{id}/observations`、`GET /api/fragments/{id}/observations`
交叉与笔顺：`POST /api/batches/{id}/crossings`、`POST /api/batches/{id}/crossings/auto`、`GET /api/batches/{id}/crossings`、`POST /api/batches/{id}/candidates`、`GET /api/batches/{id}/candidates`、`GET /api/candidates/{id}`、`POST /api/candidates/{id}/confirm`、`/reject`、`POST /api/candidates/{id}/objections`、`GET /api/candidates/{id}/objections`
快照：`POST /api/batches/{id}/snapshots`、`GET /api/batches/{id}/snapshots`、`GET /api/snapshots/{id}`、`POST /api/snapshots/{id}/share`、`/freeze`、`/supersede`
其它：`GET /api/health`、`GET /api/stats`

## 持久化

SQLite（modernc.org/sqlite，CGO 无关）。建表：batches、layers、fragments、observations、crossings、order_candidates、candidate_edges、objections、snapshots。候选裁决用版本号乐观锁；冻结快照写入 evidence_json，之后 live 标尺变更不得改写已冻结证据。
