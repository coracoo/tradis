#!/bin/bash

# 设置颜色
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== 自动上传代码脚本 ===${NC}"

# 1. 检查 git 环境
if ! command -v git &> /dev/null; then
    echo -e "${RED}错误: 未找到 git 命令，请先安装 git。${NC}"
    exit 1
fi

# 2. 初始化/检查仓库
if [ ! -d ".git" ]; then
    echo -e "${YELLOW}正在初始化 git 仓库...${NC}"
    git init
    git branch -M main
else
    echo -e "${GREEN}Git 仓库已存在。${NC}"
fi

# 3. 检查 .gitignore
if [ ! -f ".gitignore" ]; then
    echo -e "${RED}警告: 未找到 .gitignore 文件！建议先创建以避免上传垃圾文件。${NC}"
    echo -e "${YELLOW}正在尝试创建默认 .gitignore...${NC}"
    # 简单的默认写入，防止完全没有
    echo "node_modules/" >> .gitignore
    echo "dist/" >> .gitignore
    echo "*.log" >> .gitignore
    echo "docker-manager-backend" >> .gitignore
fi

# 4. 添加文件并提交
echo -e "${YELLOW}正在添加文件到暂存区...${NC}"
git add .

status=$(git status --porcelain)
if [ -z "$status" ]; then
    echo -e "${GREEN}没有检测到新的更改，无需提交。${NC}"
else
    echo -e "${YELLOW}正在提交更改...${NC}"
    timestamp=$(date "+%Y-%m-%d %H:%M:%S")
    git commit -m "Auto backup: $timestamp"
    echo -e "${GREEN}本地提交完成。${NC}"
fi

# 5. 配置远程仓库
current_remote=$(git remote get-url origin 2>/dev/null)

if [ -z "$current_remote" ]; then
    echo -e "${YELLOW}未配置远程仓库 (origin)。${NC}"
    echo -e "请输入您的 GitHub 仓库地址 (例如 https://github.com/user/repo.git)"
    read -p "地址: " repo_url
    
    if [ -n "$repo_url" ]; then
        git remote add origin "$repo_url"
        echo -e "${GREEN}已添加远程仓库: $repo_url${NC}"
    else
        echo -e "${RED}未输入地址，跳过推送步骤。${NC}"
        echo -e "您可以稍后手动运行: git remote add origin <url>"
        exit 0
    fi
else
    echo -e "${GREEN}当前远程仓库: $current_remote${NC}"
fi

# 6. 推送代码
echo -e "${YELLOW}准备推送到远程仓库 (main 分支)...${NC}"
git push -u origin main

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ 代码上传成功！${NC}"
else
    echo -e "${RED}❌ 上传失败。${NC}"
    echo -e "可能原因："
    echo -e "1. 仓库地址错误"
    echo -e "2. 权限不足 (请确保已配置 SSH Key 或使用了正确的 Token)"
    echo -e "3. 网络问题"
    echo -e "4. 远程仓库包含本地没有的更改 (尝试 git pull origin main --rebase)"
fi
