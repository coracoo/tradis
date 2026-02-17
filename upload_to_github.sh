#!/bin/bash
set -euo pipefail

# 设置颜色
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== 自动上传代码脚本 (SSH模式) ===${NC}"

commit_msg=""
repo_url=""
branch="main"
ssh_key=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -m|--message)
            commit_msg="$2"
            shift 2
            ;;
        -r|--repo)
            repo_url="$2"
            shift 2
            ;;
        -b|--branch)
            branch="$2"
            shift 2
            ;;
        -k|--key)
            ssh_key="$2"
            shift 2
            ;;
        -h|--help)
            echo "用法: $0 [-m 提交信息] [-r 仓库地址] [-b 分支] [-k SSH私钥路径]"
            exit 0
            ;;
        *)
            echo -e "${RED}未知参数: $1${NC}"
            exit 1
            ;;
    esac
done

# 1. 检查 git 环境
if ! command -v git &> /dev/null; then
    echo -e "${RED}错误: 未找到 git 命令，请先安装 git。${NC}"
    exit 1
fi

# 2. 初始化/检查仓库
if [ ! -d ".git" ]; then
    echo -e "${YELLOW}正在初始化 git 仓库...${NC}"
    git init
    git branch -M "$branch"
else
    echo -e "${GREEN}Git 仓库已存在。${NC}"
fi

if [ ! -f ".gitignore" ]; then
    echo -e "${RED}警告: 未找到 .gitignore 文件，可能会上传无关文件。${NC}"
fi

git_name=$(git config user.name || true)
git_email=$(git config user.email || true)
if [ -z "${git_name}" ] || [ -z "${git_email}" ]; then
    echo -e "${YELLOW}未检测到 Git 用户信息，将仅在本仓库内设置。${NC}"
    read -p "请输入 user.name: " git_name
    read -p "请输入 user.email: " git_email
    if [ -z "${git_name}" ] || [ -z "${git_email}" ]; then
        echo -e "${RED}用户信息不能为空，已终止。${NC}"
        exit 1
    fi
    git config user.name "${git_name}"
    git config user.email "${git_email}"
fi

echo -e "${YELLOW}正在添加文件到暂存区...${NC}"
git add .

status=$(git status --porcelain)
if [ -z "$status" ]; then
    echo -e "${GREEN}没有检测到新的更改，无需提交。${NC}"
else
    echo -e "${YELLOW}正在提交更改...${NC}"
    if [ -z "${commit_msg}" ]; then
        timestamp=$(date "+%Y-%m-%d %H:%M:%S")
        commit_msg="Auto backup: ${timestamp}"
    fi
    git commit -m "${commit_msg}"
    echo -e "${GREEN}本地提交完成。${NC}"
fi

current_remote=$(git remote get-url origin 2>/dev/null)

if [ -z "${ssh_key}" ]; then
    if [ -f "$HOME/.ssh/github/id_rsa" ]; then
        ssh_key="$HOME/.ssh/github/id_rsa"
    elif [ -f "$HOME/.ssh/id_rsa" ]; then
        ssh_key="$HOME/.ssh/id_rsa"
    fi
fi

if [ -n "${ssh_key}" ] && [ -f "${ssh_key}" ]; then
    echo -e "${GREEN}检测到 SSH Key: ${ssh_key}${NC}"
    key_perm=$(stat -c "%a" "${ssh_key}")
    if [ "${key_perm}" != "600" ]; then
        chmod 600 "${ssh_key}"
    fi
    known_hosts_path="$HOME/.ssh/known_hosts"
    if [ -e "${known_hosts_path}" ] && [ ! -w "${known_hosts_path}" ]; then
        echo -e "${YELLOW}known_hosts 不可写，将临时跳过主机指纹写入。${NC}"
        export GIT_SSH_COMMAND="ssh -i ${ssh_key} -o IdentitiesOnly=yes -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"
    else
        export GIT_SSH_COMMAND="ssh -i ${ssh_key} -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"
    fi
else
    echo -e "${RED}警告: 未找到 SSH Key，将使用默认 SSH 配置。${NC}"
fi

if [ -z "$current_remote" ]; then
    echo -e "${YELLOW}未配置远程仓库 (origin)。${NC}"
    if [ -z "${repo_url}" ]; then
        echo -e "请输入您的 GitHub 仓库地址 (建议格式: git@github.com:user/repo.git)"
        read -p "地址: " repo_url
    fi
    
    if [ -n "$repo_url" ]; then
        git remote add origin "$repo_url"
        echo -e "${GREEN}已添加远程仓库: $repo_url${NC}"
    else
        echo -e "${RED}未输入地址，跳过推送步骤。${NC}"
        exit 0
    fi
else
    echo -e "${GREEN}当前远程仓库: $current_remote${NC}"
    if [[ "$current_remote" == https://* ]]; then
        echo -e "${YELLOW}检测到 HTTPS 协议，正在转换为 SSH 协议...${NC}"
        clean_url=$(echo "$current_remote" | sed -E 's/https?:\/\/(.*@)?//')
        ssh_url="git@${clean_url/\//:}"
        
        git remote set-url origin "$ssh_url"
        echo -e "${GREEN}已转换为 SSH 地址: $ssh_url${NC}"
    fi
fi

echo -e "${YELLOW}正在尝试通过 SSH 推送代码...${NC}"

if git push -u origin "$branch"; then
    echo -e "${GREEN}✅ 代码上传成功！${NC}"
    exit 0
else
    echo -e "${RED}❌ 推送失败。${NC}"
    echo -e "${YELLOW}=== 故障排查 ===${NC}"
    
    echo -e "${YELLOW}尝试拉取远程更改并变基 (git pull --rebase)...${NC}"
    if git pull origin "$branch" --rebase; then
        echo -e "${GREEN}合并成功，正在重试推送...${NC}"
        if git push -u origin "$branch"; then
            echo -e "${GREEN}✅ 代码上传成功！${NC}"
            exit 0
        fi
    else
        echo -e "${RED}自动合并失败。请手动解决冲突或检查 SSH 权限。${NC}"
        echo -e "提示: 确保您的公钥已添加到 GitHub 仓库的 Deploy Keys 或个人 SSH Keys 中。"
        if [ -f "${SSH_KEY_PATH}.pub" ]; then
             echo -e "公钥内容 ($SSH_KEY_PATH.pub):"
             cat "${SSH_KEY_PATH}.pub"
        fi
        exit 1
    fi
fi
