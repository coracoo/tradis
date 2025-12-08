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
        # 移除可能存在的旧 origin
        git remote remove origin 2>/dev/null
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

# 6. 处理认证与推送
echo -e "${YELLOW}正在尝试连接远程仓库...${NC}"

# 尝试首次推送
if git push -u origin main; then
    echo -e "${GREEN}✅ 代码上传成功！${NC}"
    exit 0
else
    echo -e "${RED}❌ 首次推送失败。${NC}"
    echo -e "${YELLOW}=== 故障排查 ===${NC}"
    
    # 检查是否是因为需要 Pull
    git fetch origin main 2>/dev/null
    local_hash=$(git rev-parse HEAD)
    remote_hash=$(git rev-parse origin/main 2>/dev/null)
    
    if [ -n "$remote_hash" ] && [ "$local_hash" != "$remote_hash" ]; then
        echo -e "${YELLOW}检测到远程仓库包含本地没有的更改。${NC}"
        read -p "是否尝试合并远程更改 (git pull --rebase)? [y/N] " merge_choice
        if [[ "$merge_choice" =~ ^[Yy]$ ]]; then
            if git pull origin main --rebase; then
                echo -e "${GREEN}合并成功，正在重试推送...${NC}"
                git push -u origin main && echo -e "${GREEN}✅ 代码上传成功！${NC}" && exit 0
            else
                echo -e "${RED}合并失败，请手动解决冲突。${NC}"
                exit 1
            fi
        fi
    fi

    echo -e "${YELLOW}如果是因为认证失败（Password authentication is not supported），请尝试使用 Token。${NC}"
    read -p "是否要输入 GitHub Personal Access Token 进行认证? [y/N] " token_choice
    
    if [[ "$token_choice" =~ ^[Yy]$ ]]; then
        read -p "请输入您的 GitHub 用户名: " gh_user
        read -s -p "请输入您的 GitHub Token (输入时不显示): " gh_token
        echo ""
        
        # 获取当前仓库 URL 的路径部分
        repo_url=$(git remote get-url origin)
        # 提取 github.com/user/repo.git 部分
        clean_url=$(echo "$repo_url" | sed -E 's/https?:\/\/(.*@)?//')
        
        # 构造带 Token 的 URL
        new_url="https://${gh_user}:${gh_token}@${clean_url}"
        
        git remote set-url origin "$new_url"
        echo -e "${GREEN}已更新远程仓库地址包含认证信息。${NC}"
        echo -e "${YELLOW}正在重试推送...${NC}"
        
        if git push -u origin main; then
            echo -e "${GREEN}✅ 代码上传成功！${NC}"
        else
             echo -e "${RED}❌ 仍然失败。请检查 Token 是否只有效或网络连接。${NC}"
        fi
    else
        echo -e "${YELLOW}您可以检查 SSH Key 配置或手动运行 git push。${NC}"
    fi
fi

