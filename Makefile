.PHONY: build build-frontend build-backend clean dev

# 完整构建：前端 → 拷贝产物 → 后端编译 → 输出单二进制
build: build-frontend build-backend
	@echo "Build complete: ./network-plan"

# 构建前端
build-frontend:
	cd web && npm install && npm run build
	rm -rf static/dist
	cp -r web/dist static/dist

# 编译 Go 后端（自动嵌入 static/dist 前端资源）
build-backend:
	CGO_ENABLED=0 go build -o network-plan ./cmd/main.go

# 开发模式：仅启动后端（前端另开 npm run dev）
dev:
	go run ./cmd/main.go

# 清理构建产物
clean:
	rm -f network-plan
	rm -rf static/dist web/dist
