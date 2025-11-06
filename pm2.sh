#!/bin/bash

# NoFX Trading Bot - PM2 Management Script
# Usage: ./pm2.sh [start|stop|restart|status|logs|build]

set -e

# Automatically get script directory (supports symlinks)
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_ROOT"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function: Print colored messages
print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_header() {
    echo -e "${PURPLE}═══════════════════════════════════════${NC}"
    echo -e "${PURPLE}  🤖 NoFX Trading Bot - PM2 Manager${NC}"
    echo -e "${PURPLE}═══════════════════════════════════════${NC}"
    echo ""
}

# Function: Check if PM2 is installed
check_pm2() {
    if ! command -v pm2 &> /dev/null; then
        print_error "PM2 is not installed, please install first: npm install -g pm2"
        exit 1
    fi
}

# Function: Ensure log directories exist
ensure_log_dirs() {
    mkdir -p "$PROJECT_ROOT/logs"
    mkdir -p "$PROJECT_ROOT/web/logs"
    print_info "Log directories created"
}

# Function: Build backend
build_backend() {
    print_info "Building backend..."
    go build -o nofx
    if [ $? -eq 0 ]; then
        print_success "Backend build completed"
    else
        print_error "Backend build failed"
        exit 1
    fi
}

# Function: Build frontend (production)
build_frontend() {
    print_info "Building frontend..."
    cd web
    npm run build
    if [ $? -eq 0 ]; then
        print_success "Frontend build completed"
        cd ..
    else
        print_error "Frontend build failed"
        exit 1
    fi
}

# Function: Start services
start_services() {
    print_header
    ensure_log_dirs

    # Check if backend binary exists
    if [ ! -f "./nofx" ]; then
        print_warning "Backend binary does not exist, starting build..."
        build_backend
    fi

    print_info "Starting services..."
    pm2 start pm2.config.js

    sleep 2
    pm2 status

    echo ""
    print_success "Services started!"
    echo ""
    echo -e "${CYAN}📊 Access addresses:${NC}"
    echo -e "  ${GREEN}Frontend:${NC} http://localhost:3000"
    echo -e "  ${GREEN}Backend API:${NC} http://localhost:8080"
    echo ""
    echo -e "${CYAN}📝 View logs:${NC}"
    echo -e "  ${GREEN}Real-time logs:${NC} ./pm2.sh logs"
    echo -e "  ${GREEN}Backend logs:${NC} ./pm2.sh logs backend"
    echo -e "  ${GREEN}Frontend logs:${NC} ./pm2.sh logs frontend"
    echo ""
}

# Function: Stop services
stop_services() {
    print_header
    print_info "Stopping services..."
    pm2 stop pm2.config.js
    print_success "Services stopped"
}

# Function: Restart services
restart_services() {
    print_header
    print_info "Restarting services..."
    pm2 restart pm2.config.js
    sleep 2
    pm2 status
    print_success "Services restarted"
}

# Function: Delete services
delete_services() {
    print_header
    print_warning "Deleting PM2 services..."
    pm2 delete pm2.config.js || true
    print_success "PM2 services deleted"
}

# Function: Show status
show_status() {
    print_header
    pm2 status
    echo ""
    print_info "Detailed information:"
    pm2 info nofx-backend
    echo ""
    pm2 info nofx-frontend
}

# Function: Show logs
show_logs() {
    if [ -z "$2" ]; then
        # Show all logs
        pm2 logs
    elif [ "$2" = "backend" ]; then
        pm2 logs nofx-backend
    elif [ "$2" = "frontend" ]; then
        pm2 logs nofx-frontend
    else
        print_error "Unknown log type: $2"
        print_info "Usage: ./pm2.sh logs [backend|frontend]"
        exit 1
    fi
}

# Function: Monitor
show_monitor() {
    print_header
    print_info "Starting PM2 monitoring panel..."
    pm2 monit
}

# Function: Rebuild and restart
rebuild_and_restart() {
    print_header
    print_info "Rebuilding backend..."
    build_backend

    print_info "Restarting backend service..."
    pm2 restart nofx-backend

    sleep 2
    pm2 status
    print_success "Backend rebuilt and restarted"
}

# Function: Show help
show_help() {
    print_header
    echo -e "${CYAN}Usage:${NC}"
    echo "  ./pm2.sh [command]"
    echo ""
    echo -e "${CYAN}Available commands:${NC}"
    echo -e "  ${GREEN}start${NC}       - Start frontend and backend services"
    echo -e "  ${GREEN}stop${NC}        - Stop all services"
    echo -e "  ${GREEN}restart${NC}     - Restart all services"
    echo -e "  ${GREEN}status${NC}      - View service status"
    echo -e "  ${GREEN}logs${NC}        - View all logs (Ctrl+C to exit)"
    echo -e "  ${GREEN}logs backend${NC}  - View backend logs"
    echo -e "  ${GREEN}logs frontend${NC} - View frontend logs"
    echo -e "  ${GREEN}monitor${NC}     - Open PM2 monitoring panel"
    echo -e "  ${GREEN}build${NC}       - Build backend"
    echo -e "  ${GREEN}rebuild${NC}     - Rebuild backend and restart"
    echo -e "  ${GREEN}delete${NC}      - Delete PM2 services"
    echo -e "  ${GREEN}help${NC}        - Show this help message"
    echo ""
    echo -e "${CYAN}Examples:${NC}"
    echo "  ./pm2.sh start          # Start services"
    echo "  ./pm2.sh logs backend   # View backend logs"
    echo "  ./pm2.sh rebuild        # Rebuild backend and restart"
    echo ""
}

# Main logic
check_pm2

case "${1:-help}" in
    start)
        start_services
        ;;
    stop)
        stop_services
        ;;
    restart)
        restart_services
        ;;
    status)
        show_status
        ;;
    logs)
        show_logs "$@"
        ;;
    monitor|mon)
        show_monitor
        ;;
    build)
        build_backend
        ;;
    rebuild)
        rebuild_and_restart
        ;;
    delete|remove)
        delete_services
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        print_error "Unknown command: $1"
        echo ""
        show_help
        exit 1
        ;;
esac
