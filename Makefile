.PHONY: generate run test clean dev

# 生成代码
generate:
	cd api/v1 && ~/go/bin/oapi-codegen --config=cfg.yaml openapi.yaml

# 运行服务
run:
	go run cmd/server/main.go

# 运行测试
test:
	go test ./...

# 清理生成文件
clean:
	rm -f api/v1/gen.go

# 一键重新生成并运行
dev: generate run
