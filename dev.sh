#!/bin/bash

# Accounter 本地一键启动脚本（SQLite + H5）
# 用法：./dev.sh

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Accounter 本地开发环境启动中...${NC}"

# 环境变量（SQLite + 开发模式）
export DB_DRIVER=sqlite
export JWT_SECRET=dev-secret
export SERVER_PORT=8080

# 检查后端依赖
echo -e "${YELLOW}📦 检查后端依赖...${NC}"
cd backend
export SQLITE_PATH=dev.db
go mod tidy
cd ..

# 检查前端依赖
echo -e "${YELLOW}📦 检查前端依赖...${NC}"
cd frontend
if [ ! -d "node_modules" ]; then
    echo "首次运行，安装前端依赖..."
    pnpm install
fi
cd ..

# 启动后端
echo -e "${GREEN}🔧 启动后端 (Go + SQLite)...${NC}"
cd backend
go run ./cmd/server > dev-server.log 2>&1 &
BACKEND_PID=$!
cd ..
echo -e "${GREEN}✅ 后端 PID: $BACKEND_PID${NC}"
echo -e "   日志: backend/dev-server.log"
echo -e "   地址: http://localhost:$SERVER_PORT"

# 启动前端
echo -e "${GREEN}🎨 启动前端 (uni-app H5)...${NC}"
cd frontend
pnpm run dev:h5 &
FRONTEND_PID=$!
cd ..
echo -e "${GREEN}✅ 前端 PID: $FRONTEND_PID${NC}"
echo -e "   地址: http://localhost:5173"

echo ""
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}✅ 启动完成！${NC}"
echo -e "${GREEN}=========================================${NC}"
echo -e "后端 API:  http://localhost:$SERVER_PORT/health"
echo -e "前端页面:  http://localhost:5173"
echo -e "数据库:    backend/dev.db (SQLite)"
echo ""
echo -e "${YELLOW}按 Ctrl+C 停止所有服务${NC}"

# 捕获退出信号，清理进程
cleanup() {
    echo ""
    echo -e "${YELLOW}🛑 停止服务...${NC}"
    kill $BACKEND_PID 2>/dev/null || true
    kill $FRONTEND_PID 2>/dev/null || true
    echo -e "${GREEN}✅ 已停止${NC}"
    exit 0
}

trap cleanup SIGINT SIGTERM

# 等待进程
wait
