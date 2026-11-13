# 全文内容检索版本：升级与迁移

当前分支新增了基于 SQLite FTS5 的内容检索。**本次改动只是增量功能，没有 schema 破坏性变更，不需要单独的数据迁移脚本。**

## 结论

部署流程：**拉取代码 → `make build-docker`（或 `make build`） → 用新镜像替换旧容器**。
迁移流程：**升级本身不需要迁移。** 唯一需要的动作是「开启配置」+「等待一次完整扫描把文本回填进 FTS 索引」。

## 一、升级前检查

| 项 | 说明 |
|---|---|
| 持久化目录 | 确认 `config.yaml`、`server.database.path`（用户/权限/分享库）和 `server.cacheDir` 所在的卷已挂载并持久化。升级不碰这些数据。 |
| 磁盘空间 | 内容索引会增加索引库体积（文本前 1MB + 文档前 50 页），预留若干 GB。 |
| 备份 | 备份 `config.yaml`、用户数据库文件、缓存目录下的 `sql/index_all.db`。 |

## 二、重新编译

### Docker（推荐，与官方镜像一致）

```bash
git pull
make build-docker            # 使用 _docker/Dockerfile，内置 -tags analytics,mupdf,musl
# 或指定自己的镜像名：
docker build --build-arg VERSION=$(git describe --tags --always) --build-arg REVISION=$(git rev-parse --short HEAD) \
  -t myregistry/filebrowser:fulltext -f _docker/Dockerfile .
```

> `mupdf` 标签由 Dockerfile 里 `go run ./scripts/setup-gofitz-cgo` 自动准备，PDF/Office 文本抽取依赖它。
> 用 `_docker/Dockerfile.slim` 构建的镜像不含 MuPDF：只有 txt/md/代码等纯文本会被索引，PDF/Office 仅按文件名检索。

### 本机二进制

```bash
cd backend
go run ./scripts/setup-gofitz-cgo     # 只需要一次，失败则跳过（退回纯文本索引）
CGO_ENABLED=1 go build -tags analytics,mupdf -ldflags="-w -s" -o filebrowser .
cd ../frontend && npm ci && npm run build
```

> 本机验证时 `-tags=mupdf` 需要本机有 `mupdf/fitz.h`（由 `setup-gofitz-cgo` 链接）；缺失时构建会报 `mupdf/fitz.h: No such file or directory`，此时去掉 `mupdf` 标签即可，功能降级为纯文本内容索引。

## 三、替换运行中的实例

Docker：

```bash
docker compose pull            # 或使用上面构建的本地镜像
docker compose up -d
docker logs -f <容器名> | grep -i "full-text\|cache directory"
```

独立二进制：停旧进程 → 替换 `filebrowser` → 启动。

启动时应看到旧的索引库被复用（不会重建）：日志里没有 `recreating` 也就说明 `index_all.db` 被沿用；FTS 表通过 `CREATE VIRTUAL TABLE IF NOT EXISTS` 自动创建。

## 四、迁移 / 启用步骤

1. **什么都不用做也能升级**：新版本默认 `fullTextSearch=false`，行为与旧版完全一致，FTS 表只是空表。
2. 要启用内容检索，在 `config.yaml` 的 `server.indexSqlConfig` 下加：

```yaml
server:
  indexSqlConfig:
    fullTextSearch: true
```

3. **重启一次**服务。
4. **触发一次完整扫描回填**：旧索引条目是在该开关打开之前写入的，没有正文。把 `server.indexSqlConfig.disableReuse` 设为 `true` 并重启，或对每个 source 手动触发重新扫描，让扫描器把文本写入 FTS 索引。回填完成后可以把 `disableReuse` 改回 `false`（或删除该项）。

   - 回填只会索引「扫描时能读到」的文件；被 include/exclude 规则挡掉的、或超过大小上限的不会被索引。
   - 扫描期间搜索仍可用，只是内容命中会逐步变多。

5. 用户侧：打开「高级搜索」，勾选 **Search file contents / 搜索文件内容**（与通配符搜索互斥），或调用 API：

```bash
curl -G 'http://host/api/tools/search' \
  -H 'Authorization: Bearer <API token>' \
  --data-urlencode 'query=预算' \
  --data-urlencode 'sources=mydisk' \
  --data-urlencode 'searchContent=true'
```

服务端未开启 `fullTextSearch` 时，带 `searchContent=true` 的请求会返回 400。

## 五、索引维护（新增行为）

- 扫描新增/修改文件时同步写入/更新 FTS 行（正文 = 文件名 + 抽取内容）。
- 删除文件、递归删除目录、扫描发现失效条目时，对应 FTS 行一并删除。
- 24 小时周期清理也会清理孤立的 FTS 行。
- 索引库损坏被重建时，FTS 表一起重建，下次扫描自动回填。

## 六、回滚

1. 把 `fullTextSearch` 改回 `false`（或者删掉该配置项），重启。
2. 如需彻底回到旧二进制：**先停止新版本服务，再启动旧版本**。旧版本看到 `index_all.db` 里多出的 `index_fts` 虚表会忽略它，不会报错；如果想得到完全干净的旧索引库，删除缓存目录下的 `sql/index_*.db` 让旧版重建并重新扫描即可（代价是一次全量扫描）。

> 注意：不要从**旧版本**直接降级读库后再让新版本接着用——FTS 行可能落后于 `index_items`（回滚期间的文件变更）。重新启用时做一次全量扫描即可对齐。

## 七、已知限制

- 只索引：文本/代码/Markdown（前 1 MiB）、CSV，以及启用 MuPDF 构建时的 PDF/XPS/EPUB/MOBI/FB2/CBZ/DOCX/PPTX/XLSX/HWP/HWPX（前 50 页、≤ 50 MiB、单文件 ≤ 5 秒）。
- 其他类型（图片、音视频、压缩包）只按文件名匹配。
- 搜索词按前缀匹配、多词为 AND；同样遵循 `server.minSearchLength` 与 `server.searchResultsLimit`。
- 内容命中与文件名命中遵循同一套用户 scope 与访问规则过滤，不会越权返回。
