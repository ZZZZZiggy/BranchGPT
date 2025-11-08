#!/bin/bash
# PDF Processor 启动脚本

# 切换到项目根目录
cd "$(dirname "$0")"

# 设置环境变量
export PYTHONPATH=".:./protos"

# 激活虚拟环境并运行
if [ -f ".venv/bin/activate" ]; then
    echo "🚀 启动 PDF Processor 服务..."
    source .venv/bin/activate
    python main.py
else
    echo "❌ 虚拟环境不存在，请先运行: uv venv"
    exit 1
fi
