#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DOCKERFILE="$SCRIPT_DIR/dockerfile_sim"
CONTEXT="$SCRIPT_DIR"

PUSH_LATEST=0
VERSION=""
VITE_MANAGEMENT_MODE="${VITE_MANAGEMENT_MODE:-CS}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version|-v|--tag|-t)
      VERSION="$2"
      shift 2
      ;;
    --push)
      PUSH_LATEST=1
      shift
      ;;
    --help|-h)
      echo "用法: $0 [--version <版本号>] [--push]"
      echo "示例:"
      echo "  $0 --version v1.0.0"
      echo "  $0 --version v1.0.0 --push"
      exit 0
      ;;
    *)
      echo "未知参数: $1"
      exit 1
      ;;
  esac
done

if [[ -z "$VERSION" ]]; then
  read -r -p "请输入版本号（例如 v1.0.0 或 1.0.0）: " VERSION
fi

if [[ -z "$VERSION" ]]; then
  echo "版本号不能为空"
  exit 1
fi
if [[ "$VERSION" == *:* ]]; then
  VERSION="${VERSION##*:}"
fi

TAG_DH="coracoo/tradis:${VERSION}"
TAG_ACR="crpi-xg6dfmt5h2etc7hg.cn-hangzhou.personal.cr.aliyuncs.com/cherry4nas/tradis:${VERSION}"
LATEST_TAG_DH="coracoo/tradis:latest"
LATEST_TAG_ACR="crpi-xg6dfmt5h2etc7hg.cn-hangzhou.personal.cr.aliyuncs.com/cherry4nas/tradis:latest"

echo "开始构建: $TAG_DH 和 $TAG_ACR"
docker build -f "$DOCKERFILE" -t "$TAG_DH" --build-arg http_proxy=http://192.168.0.135:7890 --build-arg https_proxy=http://192.168.0.135:7890 --build-arg VITE_MANAGEMENT_MODE="$VITE_MANAGEMENT_MODE" .

echo "推送: $TAG_DH"
docker push "$TAG_DH"
# echo "推送: $TAG_ACR"
# docker push "$TAG_ACR"

echo "重命名为: $LATEST_TAG_DH"
docker tag "$TAG_DH" "$LATEST_TAG_DH"
echo "重命名为: $LATEST_TAG_ACR"
docker tag "$TAG_ACR" "$LATEST_TAG_ACR"

if [[ "$PUSH_LATEST" -eq 1 ]]; then
  echo "推送: $LATEST_TAG_DH"
  docker push "$LATEST_TAG_DH"
  echo "推送: $LATEST_TAG_ACR"
  docker push "$LATEST_TAG_ACR"
else
  echo "使用 --push 以推送 latest 到两个仓库"
fi
