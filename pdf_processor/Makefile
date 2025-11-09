.PHONY: help run dev test clean proto redis

# 默认目标
help:
	@echo "📦 PDF Processor - 可用命令:"
	@echo ""
	@echo "  make run      - 🚀 运行完整服务"
	@echo "  make dev      - 🔧 开发模式运行"
	@echo "  make test     - 🧪 运行测试"
	@echo "  make proto    - 📦 重新生成 gRPC 代码"
	@echo "  make redis    - 🐳 启动 Redis (Homebrew)"
	@echo "  make clean    - 🧹 清理缓存文件"
	@echo ""

# 运行服务
run:
	@echo "🚀 启动 PDF Processor 服务..."
	@cd . && PYTHONPATH=".:./protos" .venv/bin/python main.py

# 开发模式（带自动重载）
dev:
	@echo "🔧 开发模式启动..."
	@cd . && PYTHONPATH=".:./protos" .venv/bin/python main.py

# 运行测试
test:
	@echo "🧪 运行测试..."
	@cd . && PYTHONPATH=".:./protos" .venv/bin/pytest

# 生成 gRPC 代码
proto:
	@echo "📦 生成 gRPC 代码..."
	@.venv/bin/python -m grpc_tools.protoc \
		-I./infra/grpc_infra/protos \
		--python_out=./infra/grpc_infra/protos \
		--grpc_python_out=./infra/grpc_infra/protos \
		./infra/grpc_infra/protos/cognicore.proto
	@echo "✅ gRPC 代码生成完成"

# 启动 Redis
redis:
	@echo "🐳 启动 Redis..."
	@brew services start redis
	@echo "✅ Redis 已启动"

# 停止 Redis
redis-stop:
	@echo "🛑 停止 Redis..."
	@brew services stop redis
	@echo "✅ Redis 已停止"

# 清理缓存
clean:
	@echo "🧹 清理缓存文件..."
	@find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	@find . -type f -name "*.pyc" -delete 2>/dev/null || true
	@find . -type f -name "*.pyo" -delete 2>/dev/null || true
	@find . -type d -name "*.egg-info" -exec rm -rf {} + 2>/dev/null || true
	@echo "✅ 清理完成"

# 安装依赖
install:
	@echo "📦 安装依赖..."
	@uv pip install -e .
	@echo "✅ 依赖安装完成"

# 查看日志
logs:
	@echo "📋 查看服务日志..."
	@tail -f logs/app.log || echo "❌ 日志文件不存在"
