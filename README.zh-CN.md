# Sandbox Operator

> 用于通过 Kubernetes 自定义资源管理金山云 Sandbox 模板和沙箱实例的 Operator。

[English](README.md) | **中文**

---

## 概述

Sandbox Operator 将金山云 Sandbox OpenAPI 中的资源抽象为 Kubernetes 自定义资源：

| Kind | 简写 | 说明 |
| --- | --- | --- |
| `SandboxTemplate` | `stpl` | 沙箱模板，描述镜像、计算资源、网络、存储、日志和预热池配置。 |
| `Sandbox` | `sbx` | 单个沙箱实例。 |
| `SandboxClaim` | `sbxc` | 一次性批量创建多个沙箱实例的声明。 |

operator 支持以下双向同步：

- **OpenAPI → Kubernetes**：operator 周期性查询 Sandbox OpenAPI，将模板和沙箱实例状态同步为集群中的 CR。
- **Kubernetes → OpenAPI**：创建、更新、删除 `SandboxTemplate` 和 `Sandbox` CR 时，operator 调用对应的 Sandbox OpenAPI。`SandboxClaim` 是一次性创建声明，删除 Claim 不会删除已创建的实例。

## 功能

- 提供 `SandboxTemplate`、`Sandbox` 和 `SandboxClaim` 三个 CRD。
- 支持 Sandbox OpenAPI 与 Kubernetes CR 的双向同步。
- 通过 Mutating 和 Validating Admission Webhook 操作和校验 CR。
- 支持 Helm 和原生 Kubernetes Manifest 两种部署方式。
- 支持自签名或 cert-manager 管理的 webhook TLS 证书。
- 使用命名空间内的凭据 Secret 隔离不同业务账号。

## 部署前准备

- Kubernetes 集群，建议版本不低于 1.30。
- 已配置为能访问目标集群的 `kubectl`。
- Helm 3，可选，推荐用于常规安装和升级。
- Sandbox OpenAPI 的 AK/SK、账号 ID 和地域。

## 快速开始

### 1. 安装 operator

使用 Helm：

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace
```

或使用原生 Manifest：

```bash
make deploy
```

两种方式默认使用公共镜像 `hub.kce.ksyun.com/ksyun-public/sandbox-operator:v20260707`，无需镜像拉取凭据。自建镜像和私有仓库配置见 [部署说明](docs/zh-CN/deployment.md)。

### 2. 在业务命名空间创建 OpenAPI 凭据

```bash
kubectl create namespace sandbox-demo

kubectl -n sandbox-demo create secret generic sandbox-openapi-credentials \
  --from-literal=accessKeyId='<OPENAPI_ACCESS_KEY_ID>' \
  --from-literal=secretAccessKey='<OPENAPI_SECRET_ACCESS_KEY>' \
  --from-literal=accountId='<ACCOUNT_ID>' \
  --from-literal=region='cn-beijing-6'
```

完整凭据示例见 [`config/credentials/credentials.example.yaml`](config/credentials/credentials.example.yaml)。

### 3. 创建模板和沙箱

参考 [CR 示例](docs/zh-CN/cr-examples.md) 创建 YAML 并应用到业务命名空间：

```bash
kubectl apply -n sandbox-demo -f my-template.yaml
kubectl apply -n sandbox-demo -f my-sandbox.yaml
```

也可以从 [`config/samples/sandbox_v1alpha1_sample.yaml`](config/samples/sandbox_v1alpha1_sample.yaml) 开始。

## 文档

| 文档 | 说明 |
| --- | --- |
| [部署说明](docs/zh-CN/deployment.md) | Helm 和原生 Manifest 安装、镜像、连接目标集群与凭据准备。 |
| [使用说明](docs/zh-CN/usage.md) | OpenAPI 与 Kubernetes 的双向同步流程和 CR 生命周期。 |
| [CR 示例](docs/zh-CN/cr-examples.md) | Secret、`SandboxTemplate`、`Sandbox`、内联模板和 `SandboxClaim` 示例。 |
| [原生 Kubernetes 部署资源说明](docs/zh-CN/deploy-manifests.md) | `config/deploy` 中原生资源的说明。 |
| [English documentation](README.md) | English project overview and documentation entry. |

## Operator 配置

operator 通过 `sandbox-operator-system` 命名空间中的 `sandbox-operator-config` ConfigMap 配置。Helm 可通过 `values.yaml` 覆盖这些值。

| 名称 | 默认值 | 说明 |
| --- | --- | --- |
| `OPENAPI_BASE_URL` | `http://aicp.cn-beijing-6.api.ksyun.com` | Sandbox OpenAPI 地址。金山云内部账号可使用 `http://aicp.cn-beijing-6.inner.api.ksyun.com`。 |
| `OPENAPI_AUTH_MODE` | `kop-sigv4` | OpenAPI 认证模式。 |
| `OPENAPI_SERVICE` | `aicp` | KOP 服务名称。 |
| `OPENAPI_VERSION` | `2026-04-01` | OpenAPI 版本。 |
| `DEFAULT_OPENAPI_CREDENTIAL_SECRET` | `sandbox-openapi-credentials` | 业务命名空间中的默认 OpenAPI 凭据 Secret 名称。 |
| `POLL_INTERVAL` | `30s` | OpenAPI 同步轮询间隔。 |
| `POLL_PAGE_SIZE` | `100` | OpenAPI 列表请求的分页大小。 |
| `MAX_CONCURRENT_NAMESPACES` | `5` | 并发同步的命名空间最大数量。 |
| `SYNC_NAMESPACES` | 空 | 命名空间白名单；为空时自动发现具有默认 OpenAPI Secret 的命名空间。 |
| `LEADER_ELECT` | `true` | 是否启用 leader election。 |

## 项目结构

```text
api/                 CRD Go 类型定义
api/v1alpha1/        当前 API 版本
cmd/manager/         operator 启动入口
config/              Kubernetes Manifest、CRD、RBAC、示例和凭据
config/deploy/       原生 Manifest 部署资源
config/samples/      CR 示例
charts/              Helm Chart
internal/            Controller、Webhook、OpenAPI Client 和字段映射
Makefile             常用构建和部署任务
scripts/             镜像构建、部署和卸载脚本
docs/                中英文用户文档
```

## 开发

编译 manager：

```bash
make build
```

运行测试：

```bash
make test
```

运行代码检查：

```bash
make vet
# 或
make lint
```

构建容器镜像：

```bash
make docker-build IMG=my-registry.example.com/sandbox-operator:v0.1.0
```

## 许可证

Sandbox Operator 使用 [Apache License 2.0](LICENSE)。
