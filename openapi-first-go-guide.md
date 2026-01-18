# OpenAPI-first 搭建 Go 后端完整指南

## 1. 核心理念

**OpenAPI-first（Spec-first）**：先写 API 规范（openapi.yaml），把它当成"合同/单一真相源"，然后自动生成服务端骨架 + 类型模型，你只负责写业务逻辑。

> FastAPI 是"代码优先（Code-first）→ 自动生成 OpenAPI"；
> OpenAPI-first 则是反过来："规范优先 → 自动生成代码"。

### 主要优势

| 优势 | 说明 |
|------|------|
| 文档代码一致 | 规范就是文档，也是代码生成输入，不再漂移 |
| 强类型 | 请求/响应结构体自动生成，编译期发现问题 |
| 路由自动注册 | 不用手写一堆 `router.GET("/xx", handler)` |
| 团队协作顺 | 前端/测试/后端围绕同一份 spec 开发 |

---

## 2. 技术选型

| 组件 | 选择 | 理由 |
|------|------|------|
| 代码生成器 | oapi-codegen | 生态成熟、文档丰富、社区活跃 |
| Web 框架 | net/http (Go 1.22+) | 零依赖、标准库、官方维护、路由能力已足够 |
| Swagger UI | 静态文件 embed | 完全离线、版本可控、二进制自包含 |

---

## 3. 快速搭建

### Step 0：初始化项目

```bash
mkdir demo-openapi && cd demo-openapi
go mod init example.com/demo-openapi
```

> 确保 go.mod 里是 `go 1.22` 或更高版本

### Step 1：编写 OpenAPI 规范 `api.yaml`

```yaml
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Minimal ping API server
paths:
  /ping:
    get:
      operationId: GetPing
      responses:
        "200":
          description: pong response
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Pong"
components:
  schemas:
    Pong:
      type: object
      required:
        - ping
      properties:
        ping:
          type: string
          example: pong
```

> 强烈建议为每个接口写 `operationId`，这样生成的方法名稳定、可控。

### Step 2：编写生成配置 `cfg.yaml`

```yaml
package: api
output: gen.go
generate:
  std-http-server: true
  models: true
```

### Step 3：安装并运行生成器

```bash
# 安装
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

# 生成代码
oapi-codegen --config=cfg.yaml api.yaml
```

### Step 4：实现业务逻辑 `impl.go`

```go
package api

import (
    "encoding/json"
    "net/http"
)

type Server struct{}

func NewServer() Server {
    return Server{}
}

// (GET /ping)
func (Server) GetPing(w http.ResponseWriter, r *http.Request) {
    resp := Pong{Ping: "pong"}
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(resp)
}
```

### Step 5：启动服务 `main.go`

```go
package main

import (
    "log"
    "net/http"

    "example.com/demo-openapi/api"
)

func main() {
    server := api.NewServer()

    r := http.NewServeMux()
    h := api.HandlerFromMux(server, r)

    s := &http.Server{
        Handler: h,
        Addr:    "0.0.0.0:8080",
    }

    log.Fatal(s.ListenAndServe())
}
```

### Step 6：验证

```bash
go run .
curl http://127.0.0.1:8080/ping
# 输出: {"ping":"pong"}
```

---

## 4. 请求校验（类似 FastAPI/Pydantic）

oapi-codegen 生成的 server **默认不包含请求入参校验**，需要加中间件。

### 安装 nethttp-middleware

```bash
go get github.com/oapi-codegen/nethttp-middleware
```

### 使用示例

```go
package main

import (
    "log"
    "net/http"

    "github.com/oapi-codegen/nethttp-middleware"
    "example.com/demo-openapi/api"
)

func main() {
    // 加载 OpenAPI spec
    spec, err := api.GetSwagger()
    if err != nil {
        log.Fatal(err)
    }
    spec.Servers = nil // 清除 server 校验

    server := api.NewServer()

    r := http.NewServeMux()
    h := api.HandlerFromMux(server, r)

    // 添加校验中间件
    validatorMw := middleware.OapiRequestValidator(spec)
    handler := validatorMw(h)

    s := &http.Server{
        Handler: handler,
        Addr:    "0.0.0.0:8080",
    }

    log.Fatal(s.ListenAndServe())
}
```

> 这样就能得到"参数不对自动 400"的体验。

---

## 5. 工程化目录结构

```
myproject/
├── api/
│   └── v1/
│       ├── openapi.yaml      # OpenAPI 规范（单一真相源）
│       ├── cfg.yaml          # oapi-codegen 配置
│       ├── gen.go            # 自动生成（勿手动修改）
│       └── impl.go           # handler 实现
├── cmd/
│   └── server/
│       └── main.go           # 服务入口
├── internal/
│   ├── config/               # 配置管理
│   │   └── config.go
│   ├── errors/               # 统一错误处理
│   │   └── errors.go
│   ├── middleware/           # 自定义中间件
│   │   └── logging.go
│   └── service/              # 业务逻辑层
│       └── ping_service.go
├── pkg/
│   └── response/             # 通用响应封装
│       └── response.go
├── docs/
│   └── swagger-ui/           # Swagger UI 静态文件
├── Makefile
├── go.mod
└── go.sum
```

---

## 6. Makefile

```makefile
.PHONY: generate run test clean swagger-ui dev

# 生成代码
generate:
	oapi-codegen --config=api/v1/cfg.yaml api/v1/openapi.yaml

# 运行服务
run:
	go run cmd/server/main.go

# 运行测试
test:
	go test ./...

# 清理生成文件
clean:
	rm -f api/v1/gen.go

# 下载 Swagger UI
swagger-ui:
	mkdir -p docs/swagger-ui
	curl -L https://github.com/swagger-api/swagger-ui/archive/refs/tags/v5.11.0.tar.gz | \
		tar -xz --strip-components=2 -C docs/swagger-ui swagger-ui-5.11.0/dist

# 一键重新生成并运行
dev: generate run
```

---

## 7. 统一错误码处理

### 定义错误码 `internal/errors/errors.go`

```go
package errors

import (
    "encoding/json"
    "net/http"
)

// 错误码定义
const (
    CodeSuccess        = 0
    CodeBadRequest     = 40000
    CodeUnauthorized   = 40100
    CodeForbidden      = 40300
    CodeNotFound       = 40400
    CodeInternalError  = 50000
    CodeDatabaseError  = 50001
)

// 错误消息映射
var codeMessages = map[int]string{
    CodeSuccess:       "success",
    CodeBadRequest:    "bad request",
    CodeUnauthorized:  "unauthorized",
    CodeForbidden:     "forbidden",
    CodeNotFound:      "resource not found",
    CodeInternalError: "internal server error",
    CodeDatabaseError: "database error",
}

// APIError 统一错误结构
type APIError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Detail  string `json:"detail,omitempty"`
}

// NewAPIError 创建 API 错误
func NewAPIError(code int, detail string) *APIError {
    return &APIError{
        Code:    code,
        Message: codeMessages[code],
        Detail:  detail,
    }
}

// WriteError 写入错误响应
func WriteError(w http.ResponseWriter, httpStatus int, apiErr *APIError) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(httpStatus)
    json.NewEncoder(w).Encode(apiErr)
}

// 常用错误快捷方法
func BadRequest(w http.ResponseWriter, detail string) {
    WriteError(w, http.StatusBadRequest, NewAPIError(CodeBadRequest, detail))
}

func NotFound(w http.ResponseWriter, detail string) {
    WriteError(w, http.StatusNotFound, NewAPIError(CodeNotFound, detail))
}

func InternalError(w http.ResponseWriter, detail string) {
    WriteError(w, http.StatusInternalServerError, NewAPIError(CodeInternalError, detail))
}

func Unauthorized(w http.ResponseWriter, detail string) {
    WriteError(w, http.StatusUnauthorized, NewAPIError(CodeUnauthorized, detail))
}
```

### 在 handler 中使用

```go
package api

import (
    "encoding/json"
    "net/http"

    "example.com/demo-openapi/internal/errors"
)

func (s Server) GetUser(w http.ResponseWriter, r *http.Request, id string) {
    user, err := s.userService.FindByID(id)
    if err != nil {
        errors.InternalError(w, err.Error())
        return
    }
    if user == nil {
        errors.NotFound(w, "user not found")
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}
```

---

## 8. Swagger UI 离线部署

### 1. 下载 Swagger UI

```bash
make swagger-ui
```

### 2. 修改 `docs/swagger-ui/swagger-initializer.js`

```javascript
window.onload = function() {
  window.ui = SwaggerUIBundle({
    url: "/api/v1/openapi.yaml",  // 指向你的 spec 文件
    dom_id: '#swagger-ui',
    presets: [
      SwaggerUIBundle.presets.apis,
      SwaggerUIStandalonePreset
    ],
    layout: "StandaloneLayout"
  });
};
```

### 3. 在 Go 服务中托管

```go
package main

import (
    "embed"
    "io/fs"
    "log"
    "net/http"

    "example.com/demo-openapi/api/v1"
)

//go:embed docs/swagger-ui
var swaggerUI embed.FS

//go:embed api/v1/openapi.yaml
var openapiSpec []byte

func main() {
    server := api.NewServer()

    mux := http.NewServeMux()
    h := api.HandlerFromMux(server, mux)

    // 托管 OpenAPI spec
    mux.HandleFunc("/api/v1/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/x-yaml")
        w.Write(openapiSpec)
    })

    // 托管 Swagger UI
    swaggerFS, _ := fs.Sub(swaggerUI, "docs/swagger-ui")
    mux.Handle("/docs/", http.StripPrefix("/docs/", http.FileServer(http.FS(swaggerFS))))

    s := &http.Server{
        Handler: h,
        Addr:    "0.0.0.0:8080",
    }

    log.Println("Server running on http://localhost:8080")
    log.Println("Swagger UI: http://localhost:8080/docs/")
    log.Fatal(s.ListenAndServe())
}
```

---

## 9. 生成器管理

### Go 1.24+

```bash
go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
```

### Go 1.24 之前（tools.go 模式）

创建 `tools/tools.go`：

```go
//go:build tools

package tools

import (
    _ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)
```

然后：

```bash
go mod tidy
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=cfg.yaml api.yaml
```

---

## 10. 常用命令

```bash
# 安装 oapi-codegen
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

# 生成代码
oapi-codegen --config=cfg.yaml api.yaml

# 运行服务
go run .

# 运行测试
go test ./...

# 查看生成器版本
oapi-codegen -version
```

---

## 11. 最佳实践

1. **始终从 OpenAPI spec 开始** - 它是单一真相源
2. **为每个接口写 operationId** - 保证生成的方法名稳定
3. **使用请求校验中间件** - 获得类似 FastAPI 的体验
4. **版本化 API** - 使用 `/api/v1/` 路径前缀
5. **统一错误处理** - 定义清晰的错误码体系
6. **自动化生成流程** - 用 Makefile 或 go:generate
7. **勿手动修改 gen.go** - 每次生成会覆盖
8. **部署 Swagger UI** - 方便前端和测试人员查看文档

---

## 参考链接

- [oapi-codegen GitHub](https://github.com/oapi-codegen/oapi-codegen)
- [nethttp-middleware GitHub](https://github.com/oapi-codegen/nethttp-middleware)
- [Swagger UI](https://github.com/swagger-api/swagger-ui)
