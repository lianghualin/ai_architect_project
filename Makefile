.PHONY: generate run run-backend run-frontend test clean dev stop

# 生成代码
generate:
	cd api/v1 && ~/go/bin/oapi-codegen --config=cfg.yaml openapi.yaml

# 运行服务 (backend + frontend)
run:
	@echo "Starting backend on http://localhost:8080"
	@echo "Starting frontend on http://localhost:8000"
	@echo "Press Ctrl+C to stop both servers"
	@trap 'kill 0' EXIT; \
		go run cmd/server/main.go & \
		python3 web/serve.py & \
		wait

# 只运行后端
run-backend:
	go run cmd/server/main.go

# 只运行前端
run-frontend:
	python3 web/serve.py

# 运行测试
test:
	go test ./...

# 清理生成文件
clean:
	rm -f api/v1/gen.go

# 一键重新生成并运行
dev: generate run
