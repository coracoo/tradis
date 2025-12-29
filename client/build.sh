#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DOCKERFILE="$SCRIPT_DIR/dockerfile_sim"
CONTEXT="$SCRIPT_DIR"

PUSH_LATEST=0
PUSH_ACR=0
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
    --push-acr)
      PUSH_ACR=1
      shift
      ;;
    --help|-h)
      echo "用法: $0 [--version <版本号>] [--push]"
      echo "示例:"
      echo "  $0 --version v1.0.0"
      echo "  $0 --version v1.0.0 --push"
      echo "  $0 --version v1.0.0 --push --push-acr"
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
VERSION="${VERSION#v}"
if [[ "$VERSION" == *:* ]]; then
  VERSION="${VERSION##*:}"
fi

TAG_DH="coracoo/tradis:${VERSION}"
TAG_ACR="crpi-xg6dfmt5h2etc7hg.cn-hangzhou.personal.cr.aliyuncs.com/cherry4nas/tradis:${VERSION}"
LATEST_TAG_DH="coracoo/tradis:dev"
LATEST_TAG_ACR="crpi-xg6dfmt5h2etc7hg.cn-hangzhou.personal.cr.aliyuncs.com/cherry4nas/tradis:dev"

echo "开始构建: $TAG_DH 和 $TAG_ACR"
BUILD_ARGS=(--build-arg "VITE_MANAGEMENT_MODE=$VITE_MANAGEMENT_MODE")
BUILD_ARGS+=(--build-arg "CLIENT_VERSION=$VERSION")
if [[ -n "${http_proxy:-}" ]]; then BUILD_ARGS+=(--build-arg "http_proxy=$http_proxy"); fi
if [[ -n "${https_proxy:-}" ]]; then BUILD_ARGS+=(--build-arg "https_proxy=$https_proxy"); fi
if [[ -n "${HTTP_PROXY:-}" ]]; then BUILD_ARGS+=(--build-arg "HTTP_PROXY=$HTTP_PROXY"); fi
if [[ -n "${HTTPS_PROXY:-}" ]]; then BUILD_ARGS+=(--build-arg "HTTPS_PROXY=$HTTPS_PROXY"); fi
if [[ -n "${NO_PROXY:-}" ]]; then BUILD_ARGS+=(--build-arg "NO_PROXY=$NO_PROXY"); fi

docker build -f "$DOCKERFILE" -t "$TAG_DH" -t "$TAG_ACR" "${BUILD_ARGS[@]}" .

echo "推送: $TAG_DH"
docker push "$TAG_DH"
if [[ "$PUSH_ACR" -eq 1 ]]; then
  echo "推送: $TAG_ACR"
  docker push "$TAG_ACR"
else
  echo "使用 --push-acr 以推送 $TAG_ACR"
fi

echo "重命名为: $LATEST_TAG_DH"
docker tag "$TAG_DH" "$LATEST_TAG_DH"
echo "重命名为: $LATEST_TAG_ACR"
docker tag "$TAG_DH" "$LATEST_TAG_ACR"

if [[ "$PUSH_LATEST" -eq 1 ]]; then
  echo "推送: $LATEST_TAG_DH"
  docker push "$LATEST_TAG_DH"
  if [[ "$PUSH_ACR" -eq 1 ]]; then
    echo "推送: $LATEST_TAG_ACR"
    docker push "$LATEST_TAG_ACR"
  else
    echo "使用 --push-acr 以推送 $LATEST_TAG_ACR"
  fi
else
  echo "使用 --push 以推送 latest"
  echo "使用 --push-acr 以推送阿里云仓库"
fi
