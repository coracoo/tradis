#!/bin/bash

# ================= 配置区域 =================

# 获取脚本所在的当前目录
PROJECT_DIR=$(cd "$(dirname "$0")" && pwd)

# 定义目录路径
BACKEND_DIR="$PROJECT_DIR/client/backend"
FRONTEND_DIR="$PROJECT_DIR/client/frontend"
LOG_DIR="$PROJECT_DIR/client/logs"
PID_DIR="$PROJECT_DIR/client/.pids"

# 定义后端执行命令 (在 backend 目录下执行)
BACKEND_CMD="go run cmd/main.go"
# 定义前端执行命令
FRONTEND_CMD="npm run dev"

BACKEND_PORT="${BACKEND_PORT:-8080}"
FRONTEND_PORT="${FRONTEND_PORT:-33339}"

# ================= 工具函数 =================

# 创建必要的目录
init_dirs() {
    mkdir -p "$LOG_DIR"
    mkdir -p "$PID_DIR"
    
    # 清理旧日志：保留最近20个文件
    if [ -d "$LOG_DIR" ]; then
        # 尝试清理旧日志，忽略错误
        (cd "$LOG_DIR" && ls -t *.log 2>/dev/null | tail -n +21 | xargs -r rm -- 2>/dev/null) || true
    fi
}

# 获取当前时间戳，用于日志文件名
get_timestamp() {
    date +"%Y%m%d_%H%M%S"
}

# 检查进程是否运行
is_running() {
    local pid_file=$1
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if ps -p "$pid" > /dev/null 2>&1; then
            return 0 # 正在运行
        else
            rm -f "$pid_file" # 进程已死，清理 pid 文件
            return 1
        fi
    fi
    return 1
}

get_listen_pids() {
    local port="$1"
    local pids=""
    if command -v ss >/dev/null 2>&1; then
        pids="$(ss -H -lntp "sport = :$port" 2>/dev/null | sed -n 's/.*pid=\([0-9]\+\).*/\1/p' | sort -u)"
    fi
    if [ -z "$pids" ] && command -v lsof >/dev/null 2>&1; then
        pids="$(lsof -nP -t -iTCP:"$port" -sTCP:LISTEN 2>/dev/null | sort -u)"
    fi
    if [ -z "$pids" ] && command -v netstat >/dev/null 2>&1; then
        pids="$(netstat -nlpt 2>/dev/null | awk -v p=":$port" '$4 ~ p && $7 ~ "/" { split($7,a,"/"); print a[1] }' | sort -u)"
    fi
    if [ -z "$pids" ] && command -v fuser >/dev/null 2>&1; then
        pids="$(fuser -n tcp "$port" 2>/dev/null | tr ' ' '\n' | sed '/^$/d' | sort -u)"
    fi
    echo "$pids"
}

pid_cmdline() {
    local pid="$1"
    if [ -r "/proc/$pid/cmdline" ]; then
        tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null
    else
        echo ""
    fi
}

is_port_listening() {
    local port="$1"
    if command -v ss >/dev/null 2>&1; then
        if ss -H -lnt "sport = :$port" 2>/dev/null | grep -q .; then
            return 0
        fi
    fi
    if command -v netstat >/dev/null 2>&1; then
        if netstat -nlt 2>/dev/null | awk -v p=":$port" '$4 ~ p { found=1 } END { exit found?0:1 }'; then
            return 0
        fi
    fi
    return 1
}

kill_pids() {
    local pids="$1"
    local label="$2"
    local timeout="${3:-10}"

    if [ -z "$pids" ]; then
        return 0
    fi

    echo "🛑 正在停止 $label (PID: $pids)..."
    for pid in $pids; do
        if [ -n "$pid" ] && ps -p "$pid" >/dev/null 2>&1; then
            kill "$pid" 2>/dev/null || true
        fi
    done

    local count=0
    while [ $count -lt "$timeout" ]; do
        local alive=0
        for pid in $pids; do
            if [ -n "$pid" ] && ps -p "$pid" >/dev/null 2>&1; then
                alive=1
                break
            fi
        done
        if [ "$alive" -eq 0 ]; then
            return 0
        fi
        sleep 1
        count=$((count+1))
    done

    echo "⚠️  进程停止超时，强制杀死..."
    for pid in $pids; do
        if [ -n "$pid" ] && ps -p "$pid" >/dev/null 2>&1; then
            kill -9 "$pid" 2>/dev/null || true
        fi
    done
    return 0
}

filter_backend_pids() {
    local pids="$1"
    local out=""
    for pid in $pids; do
        local cmd
        cmd="$(pid_cmdline "$pid")"
        if echo "$cmd" | grep -q "docker-proxy"; then
            continue
        fi
        out="$out $pid"
    done
    echo "$(echo "$out" | xargs -r echo)"
}

filter_frontend_pids() {
    local pids="$1"
    local out=""
    for pid in $pids; do
        local cmd
        cmd="$(pid_cmdline "$pid")"
        if echo "$cmd" | grep -q "docker-proxy"; then
            continue
        fi
        out="$out $pid"
    done
    echo "$(echo "$out" | xargs -r echo)"
}

# ================= 后端管理 =================

start_backend() {
    local pid_file="$PID_DIR/backend.pid"
    local port_file="$PID_DIR/backend.port"

    echo "🚀 正在启动后端..."
    init_dirs
    
    # 进入后端目录
    cd "$BACKEND_DIR" || exit

    local pids="$(get_listen_pids "$BACKEND_PORT")"
    pids="$(filter_backend_pids "$pids")"
    if [ -z "$pids" ] && is_port_listening "$BACKEND_PORT" && ! is_running "$pid_file"; then
        local fallback_ports=("$BACKEND_PORT" "18080" "18081" "18082" "18083")
        for p in "${fallback_ports[@]}"; do
            if ! is_port_listening "$p"; then
                if [ "$p" != "$BACKEND_PORT" ]; then
                    echo "⚠️  端口 $BACKEND_PORT 已被占用且无法解析 PID，自动切换到端口 $p"
                    BACKEND_PORT="$p"
                fi
                break
            fi
        done
    fi

    if [ -n "$pids" ]; then
        kill_pids "$pids" "后端(端口:$BACKEND_PORT)" 12
        rm -f "$pid_file"
        rm -f "$port_file"
        sleep 1
    elif is_running "$pid_file"; then
        kill_pids "$(cat "$pid_file")" "后端" 12
        rm -f "$pid_file"
        rm -f "$port_file"
        sleep 1
    elif is_port_listening "$BACKEND_PORT"; then
        echo "❌ 端口 $BACKEND_PORT 已被占用，但无法解析 PID（请使用 root 执行 stop 或手动释放端口）"
        exit 1
    fi

    if is_port_listening "$BACKEND_PORT"; then
        local fallback_ports=("$BACKEND_PORT" "18080" "18081" "18082" "18083")
        for p in "${fallback_ports[@]}"; do
            if ! is_port_listening "$p"; then
                if [ "$p" != "$BACKEND_PORT" ]; then
                    echo "⚠️  端口 $BACKEND_PORT 已被占用，自动切换到端口 $p"
                    BACKEND_PORT="$p"
                fi
                break
            fi
        done
    fi

    # 定义日志文件
    local log_file="$LOG_DIR/backend_$(get_timestamp).log"
    
    local backend_cmd="$BACKEND_CMD"
    local docker_sock_path=""
    if [ -n "$DOCKER_SOCK" ]; then
        docker_sock_path="$DOCKER_SOCK"
    elif [ -n "$DOCKER_HOST" ]; then
        if [[ "$DOCKER_HOST" == unix://* ]]; then
            docker_sock_path="${DOCKER_HOST#unix://}"
        fi
    else
        docker_sock_path="/var/run/docker.sock"
    fi

    if [[ "$docker_sock_path" == unix://* ]]; then
        docker_sock_path="${docker_sock_path#unix://}"
    fi

    if [ -n "$docker_sock_path" ] && [ -S "$docker_sock_path" ] && [ ! -w "$docker_sock_path" ] && [ "$(id -u)" -ne 0 ]; then
        if command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
            backend_cmd="sudo -E $BACKEND_CMD"
        else
            echo "❌ 当前用户无权限访问 $docker_sock_path，请将用户加入 docker 组后重新登录或使用 sudo 启动后端"
            exit 1
        fi
    fi

    BACKEND_PORT="$BACKEND_PORT" nohup $backend_cmd > "$log_file" 2>&1 &

    local ok=0
    for i in $(seq 1 20); do
        sleep 1
        local pids_now
        pids_now="$(filter_backend_pids "$(get_listen_pids "$BACKEND_PORT")")"
        if [ -n "$pids_now" ]; then
            echo "$pids_now" | awk '{print $1}' > "$pid_file"
            echo "$BACKEND_PORT" > "$port_file"
            ok=1
            echo "✅ 后端启动成功! PID: $(cat "$pid_file")"
            echo "📝 日志路径: $log_file"
            break
        fi
    done
    if [ "$ok" -ne 1 ]; then
        echo "❌ 后端启动失败（端口:$BACKEND_PORT 未监听），请检查日志: $log_file"
        rm -f "$pid_file"
        exit 1
    fi
}

stop_backend() {
    local pid_file="$PID_DIR/backend.pid"
    local port_file="$PID_DIR/backend.port"
    local port="$BACKEND_PORT"
    if [ -f "$port_file" ]; then
        port="$(cat "$port_file" 2>/dev/null | tr -d ' \n\r\t')"
        if [ -z "$port" ]; then
            port="$BACKEND_PORT"
        fi
    fi

    local pids="$(get_listen_pids "$port")"
    pids="$(filter_backend_pids "$pids")"
    if [ -n "$pids" ]; then
        kill_pids "$pids" "后端(端口:$port)" 12
        rm -f "$pid_file"
        rm -f "$port_file"
        echo "✅ 后端已停止"
        return
    fi
    if is_port_listening "$port"; then
        echo "⚠️  后端端口 $port 正在监听，但无法解析 PID（请使用 root 执行）"
        return
    fi

    if is_running "$pid_file"; then
        kill_pids "$(cat "$pid_file")" "后端" 12
        rm -f "$pid_file"
        rm -f "$port_file"
        echo "✅ 后端已停止"
        return
    fi

    echo "⚠️  后端未运行"
}

# ================= 前端管理 =================

start_frontend() {
    local pid_file="$PID_DIR/frontend.pid"
    local port_file="$PID_DIR/frontend.port"
    local backend_port_file="$PID_DIR/backend.port"

    echo "🚀 正在启动前端..."
    init_dirs

    cd "$FRONTEND_DIR" || exit

    if [ -f "$backend_port_file" ]; then
        local bp
        bp="$(cat "$backend_port_file" 2>/dev/null | tr -d ' \n\r\t')"
        if [ -n "$bp" ]; then
            BACKEND_PORT="$bp"
        fi
    fi

    local pids="$(get_listen_pids "$FRONTEND_PORT")"
    pids="$(filter_frontend_pids "$pids")"
    if [ -z "$pids" ] && is_port_listening "$FRONTEND_PORT" && ! is_running "$pid_file"; then
        local fallback_ports=("$FRONTEND_PORT" "33340" "33341" "33342" "33343" "33344" "33345")
        for p in "${fallback_ports[@]}"; do
            if ! is_port_listening "$p"; then
                if [ "$p" != "$FRONTEND_PORT" ]; then
                    echo "⚠️  端口 $FRONTEND_PORT 已被占用且无法解析 PID，自动切换到端口 $p"
                    FRONTEND_PORT="$p"
                fi
                break
            fi
        done
    fi
    if [ -n "$pids" ]; then
        kill_pids "$pids" "前端(端口:$FRONTEND_PORT)" 12
        rm -f "$pid_file"
        rm -f "$port_file"
        sleep 1
    elif is_running "$pid_file"; then
        kill_pids "$(cat "$pid_file")" "前端" 12
        rm -f "$pid_file"
        rm -f "$port_file"
        sleep 1
    elif is_port_listening "$FRONTEND_PORT"; then
        echo "❌ 端口 $FRONTEND_PORT 已被占用，但无法解析 PID（请使用 root 执行 stop 或手动释放端口）"
        exit 1
    fi

    local log_file="$LOG_DIR/frontend_$(get_timestamp).log"

    FRONTEND_PORT="$FRONTEND_PORT" BACKEND_PORT="$BACKEND_PORT" nohup $FRONTEND_CMD -- --host 0.0.0.0 --port "$FRONTEND_PORT" --strictPort > "$log_file" 2>&1 &

    local ok=0
    for i in $(seq 1 30); do
        sleep 1
        local pids_now
        pids_now="$(filter_frontend_pids "$(get_listen_pids "$FRONTEND_PORT")")"
        if [ -n "$pids_now" ]; then
            echo "$pids_now" | awk '{print $1}' > "$pid_file"
            echo "$FRONTEND_PORT" > "$port_file"
            ok=1
            echo "✅ 前端启动成功! PID: $(cat "$pid_file")"
            echo "📝 日志路径: $log_file"
            break
        fi
    done
    if [ "$ok" -ne 1 ]; then
        echo "❌ 前端启动失败（端口:$FRONTEND_PORT 未监听），请检查日志: $log_file"
        rm -f "$pid_file"
        exit 1
    fi
}

stop_frontend() {
    local pid_file="$PID_DIR/frontend.pid"
    local port_file="$PID_DIR/frontend.port"
    local port="$FRONTEND_PORT"
    if [ -f "$port_file" ]; then
        port="$(cat "$port_file" 2>/dev/null | tr -d ' \n\r\t')"
        if [ -z "$port" ]; then
            port="$FRONTEND_PORT"
        fi
    fi

    local pids="$(get_listen_pids "$port")"
    pids="$(filter_frontend_pids "$pids")"
    if [ -n "$pids" ]; then
        kill_pids "$pids" "前端(端口:$port)" 12
        rm -f "$pid_file"
        rm -f "$port_file"
        echo "✅ 前端已停止"
        return
    fi
    if is_port_listening "$port"; then
        echo "⚠️  前端端口 $port 正在监听，但无法解析 PID（请使用 root 执行）"
        return
    fi

    if is_running "$pid_file"; then
        kill_pids "$(cat "$pid_file")" "前端" 12
        rm -f "$pid_file"
        rm -f "$port_file"
        echo "✅ 前端已停止"
        return
    fi

    echo "⚠️  前端未运行"
}

# ================= 主逻辑 =================

# 显示帮助信息
show_help() {
    echo "Usage: $0 [选项] [操作]"
    echo ""
    echo "Options:"
    echo "  -h           显示帮助信息"
    echo "  -b <action>  管理后端"
    echo "  -f <action>  管理前端"
    echo ""
    echo "Actions:"
    echo "  start        启动服务"
    echo "  stop         停止服务"
    echo ""
    echo "Examples:"
    echo "  $0 -b start    # 启动后端"
    echo "  $0 -f stop     # 停止前端"
}

# 如果没有参数，显示帮助
if [ $# -eq 0 ]; then
    show_help
    exit 0
fi

# 解析参数
while getopts "h:b:f:" opt; do
    case $opt in
        h)
            show_help
            exit 0
            ;;
        b)
            case $OPTARG in
                start) start_backend ;;
                stop)  stop_backend ;;
                *) echo "❌ 无效的后端操作: $OPTARG (仅支持 start/stop)"; exit 1 ;;
            esac
            ;;
        f)
            case $OPTARG in
                start) start_frontend ;;
                stop)  stop_frontend ;;
                *) echo "❌ 无效的前端操作: $OPTARG (仅支持 start/stop)"; exit 1 ;;
            esac
            ;;
        \?)
            echo "❌ 无效选项: -$OPTARG" >&2
            show_help
            exit 1
            ;;
        :)
            echo "❌ 选项 -$OPTARG 需要一个参数." >&2
            exit 1
            ;;
    esac
done
