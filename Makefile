.PHONY: all build build-all web clean run install test lint docker

APP_NAME     := yunxi-home
VERSION      := 3.0.0
BUILD_TIME   := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS      := -s -w \
                -X 'main.Version=$(VERSION)' \
                -X 'main.BuildTime=$(BUILD_TIME)' \
                -X 'main.GitCommit=$(GIT_COMMIT)'

OUTPUT_DIR   := ./build
WEB_DIR      := ./web

# ==================== 默认目标 ====================
all: build

# ==================== 前端构建 ====================
web:
	@echo "Building frontend..."
	cd $(WEB_DIR) && npm ci --silent && npm run build
	@echo "Copying frontend to embed directory..."
	rm -rf internal/web/static/
	mkdir -p internal/web/static/
	cp -r $(WEB_DIR)/dist/* internal/web/static/
	@echo "Frontend build complete"

# ==================== 后端构建 ====================
build: web
	@echo "Building $(APP_NAME) v$(VERSION)..."
	@mkdir -p $(OUTPUT_DIR)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/$(APP_NAME) ./cmd/yunxi-home/
	@echo "Build complete: $(OUTPUT_DIR)/$(APP_NAME)"

build-no-web:
	@echo "Building $(APP_NAME) v$(VERSION) (without frontend)..."
	@mkdir -p $(OUTPUT_DIR)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/$(APP_NAME) ./cmd/yunxi-home/
	@echo "Build complete: $(OUTPUT_DIR)/$(APP_NAME)"

# ==================== 发布打包 (Linux + Windows) ====================
release:
	@bash scripts/release.sh $(VERSION)

# ==================== 跨平台编译（全部平台） ====================
build-all:
	@echo "Cross-compiling $(APP_NAME) v$(VERSION)..."
	@mkdir -p $(OUTPUT_DIR)
	@bash scripts/build.sh $(VERSION) "$(BUILD_TIME)" $(GIT_COMMIT)
	@echo "All platforms complete"

# ==================== 运行 ====================
run: build
	@echo "Starting $(APP_NAME)..."
	$(OUTPUT_DIR)/$(APP_NAME) -config ./configs/config.yaml

dev:
	@echo "Starting in dev mode..."
	CGO_ENABLED=0 go run ./cmd/yunxi-home/ -config ./configs/config.yaml

# ==================== 测试 ====================
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "Coverage:"
	@go tool cover -func=coverage.out | tail -1

test-unit:
	go test -v -short ./internal/...

test-integration:
	go test -v -tags=integration ./tests/...

# ==================== 代码检查 ====================
lint:
	@echo "Linting..."
	go vet ./...

# ==================== 清理 ====================
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(OUTPUT_DIR)/
	rm -rf ./release/
	rm -rf internal/web/static/
	rm -f coverage.out
	@echo "Clean complete"

# ==================== Docker ====================
docker:
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .

docker-run:
	docker run -d --name $(APP_NAME) \
		-p 8080:8080 \
		-v $$(pwd)/configs:/app/configs \
		-v $$(pwd)/data:/app/data \
		-v $$(pwd)/log:/app/log \
		$(APP_NAME):latest

docker-push:
	docker tag $(APP_NAME):latest your-registry/$(APP_NAME):$(VERSION)
	docker push your-registry/$(APP_NAME):$(VERSION)

# ==================== 部署 ====================
deploy:
	bash scripts/deploy.sh

deploy-dry:
	bash scripts/deploy.sh --dry-run

deploy-rollback:
	bash scripts/deploy.sh --rollback