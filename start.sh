#!/bin/bash

# ═══════════════════════════════════════════════════════════════
# NOFX AI Trading System - Docker Quick Start Script
# Usage: ./start.sh [command]
# ═══════════════════════════════════════════════════════════════

set -e

# ------------------------------------------------------------------------
# Color Definitions
# ------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ------------------------------------------------------------------------
# Utility Functions: Colored Output
# ------------------------------------------------------------------------
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# ------------------------------------------------------------------------
# Detection: Docker Compose Command (Backward Compatible)
# ------------------------------------------------------------------------
detect_compose_cmd() {
    if command -v docker compose &> /dev/null; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        print_error "Docker Compose is not installed! Please install Docker Compose first"
        exit 1
    fi
    print_info "Using Docker Compose command: $COMPOSE_CMD"
}

# ------------------------------------------------------------------------
# Validation: Docker Installation
# ------------------------------------------------------------------------
check_docker() {
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed! Please install Docker first: https://docs.docker.com/get-docker/"
        exit 1
    fi

    detect_compose_cmd
    print_success "Docker and Docker Compose are installed"
}

# ------------------------------------------------------------------------
# Validation: Environment File (.env)
# ------------------------------------------------------------------------
check_env() {
    if [ ! -f ".env" ]; then
        print_warning ".env does not exist, copying from template..."
        cp .env.example .env
        print_info "✓ Created .env with default environment variables"
        print_info "💡 To modify settings like ports, edit the .env file"
    fi
    print_success "Environment variable file exists"
}

# ------------------------------------------------------------------------
# Validation: Configuration File (config.json) - BASIC SETTINGS ONLY
# ------------------------------------------------------------------------
check_config() {
    if [ ! -f "config.json" ]; then
        print_warning "config.json does not exist, copying from template..."
        cp config.json.example config.json
        print_info "✓ Created config.json with default configuration"
        print_info "💡 To modify basic settings (leverage size, trading pairs, admin mode, JWT secret, etc.), edit config.json"
        print_info "💡 Model/exchange/trader configuration should be done via the Web interface"
    fi
    print_success "Configuration file exists"
}

# ------------------------------------------------------------------------
# Utility: Read Environment Variables
# ------------------------------------------------------------------------
read_env_vars() {
    if [ -f ".env" ]; then
        # Read port configuration, set default values
        NOFX_FRONTEND_PORT=$(grep "^NOFX_FRONTEND_PORT=" .env 2>/dev/null | cut -d'=' -f2 || echo "3000")
        NOFX_BACKEND_PORT=$(grep "^NOFX_BACKEND_PORT=" .env 2>/dev/null | cut -d'=' -f2 || echo "8080")
        
        # Remove possible quotes and spaces
        NOFX_FRONTEND_PORT=$(echo "$NOFX_FRONTEND_PORT" | tr -d '"'"'" | tr -d ' ')
        NOFX_BACKEND_PORT=$(echo "$NOFX_BACKEND_PORT" | tr -d '"'"'" | tr -d ' ')
        
        # Use default value if empty
        NOFX_FRONTEND_PORT=${NOFX_FRONTEND_PORT:-3000}
        NOFX_BACKEND_PORT=${NOFX_BACKEND_PORT:-8080}
    else
        # If .env does not exist, use default ports
        NOFX_FRONTEND_PORT=3000
        NOFX_BACKEND_PORT=8080
    fi
}

# ------------------------------------------------------------------------
# Validation: Database File (config.db)
# ------------------------------------------------------------------------
check_database() {
    if [ ! -f "config.db" ]; then
        print_warning "Database file does not exist, creating empty database file..."
        # Create empty file to prevent Docker from creating a directory
        touch config.db
        print_info "✓ Created empty database file, system will initialize on startup"
    else
        print_success "Database file exists"
    fi
}

# ------------------------------------------------------------------------
# Build: Frontend (Node.js Based)
# ------------------------------------------------------------------------
# build_frontend() {
#     print_info "Checking frontend build environment..."

#     if ! command -v node &> /dev/null; then
#         print_error "Node.js is not installed! Please install Node.js first"
#         exit 1
#     fi

#     if ! command -v npm &> /dev/null; then
#         print_error "npm is not installed! Please install npm first"
#         exit 1
#     fi

#     print_info "Building frontend..."
#     cd web

#     print_info "Installing Node.js dependencies..."
#     npm install

#     print_info "Building frontend application..."
#     npm run build

#     cd ..
#     print_success "Frontend build completed"
# }

# ------------------------------------------------------------------------
# Service Management: Start
# ------------------------------------------------------------------------
start() {
    print_info "Starting NOFX AI Trading System..."

    # Read environment variables
    read_env_vars

    # Ensure necessary files and directories exist (fix Docker volume mount issues)
    if [ ! -f "config.db" ]; then
        print_info "Creating database file..."
        touch config.db
    fi
    if [ ! -d "decision_logs" ]; then
        print_info "Creating logs directory..."
        mkdir -p decision_logs
    fi

    # Auto-build frontend if missing or forced
    # if [ ! -d "web/dist" ] || [ "$1" == "--build" ]; then
    #     build_frontend
    # fi

    # Rebuild images if flag set
    if [ "$1" == "--build" ]; then
        print_info "Rebuilding images..."
        $COMPOSE_CMD up -d --build
    else
        print_info "Starting containers..."
        $COMPOSE_CMD up -d
    fi

    print_success "Services started!"
    print_info "Web interface: http://localhost:${NOFX_FRONTEND_PORT}"
    print_info "API endpoint: http://localhost:${NOFX_BACKEND_PORT}"
    print_info ""
    print_info "View logs: ./start.sh logs"
    print_info "Stop services: ./start.sh stop"
}

# ------------------------------------------------------------------------
# Service Management: Stop
# ------------------------------------------------------------------------
stop() {
    print_info "Stopping services..."
    $COMPOSE_CMD stop
    print_success "Services stopped"
}

# ------------------------------------------------------------------------
# Service Management: Restart
# ------------------------------------------------------------------------
restart() {
    print_info "Restarting services..."
    $COMPOSE_CMD restart
    print_success "Services restarted"
}

# ------------------------------------------------------------------------
# Monitoring: Logs
# ------------------------------------------------------------------------
logs() {
    if [ -z "$2" ]; then
        $COMPOSE_CMD logs -f
    else
        $COMPOSE_CMD logs -f "$2"
    fi
}

# ------------------------------------------------------------------------
# Monitoring: Status
# ------------------------------------------------------------------------
status() {
    # Read environment variables
    read_env_vars
    
    print_info "Service status:"
    $COMPOSE_CMD ps
    echo ""
    print_info "Health check:"
    curl -s "http://localhost:${NOFX_BACKEND_PORT}/api/health" | jq '.' || echo "Backend not responding"
}

# ------------------------------------------------------------------------
# Maintenance: Clean (Destructive)
# ------------------------------------------------------------------------
clean() {
    print_warning "This will delete all containers and data!"
    read -p "Confirm deletion? (yes/no): " confirm
    if [ "$confirm" == "yes" ]; then
        print_info "Cleaning up..."
        $COMPOSE_CMD down -v
        print_success "Cleanup completed"
    else
        print_info "Cancelled"
    fi
}

# ------------------------------------------------------------------------
# Maintenance: Update
# ------------------------------------------------------------------------
update() {
    print_info "Updating..."
    git pull
    $COMPOSE_CMD up -d --build
    print_success "Update completed"
}

# ------------------------------------------------------------------------
# Help: Usage Information
# ------------------------------------------------------------------------
show_help() {
    echo "NOFX AI Trading System - Docker Management Script"
    echo ""
    echo "Usage: ./start.sh [command] [options]"
    echo ""
    echo "Commands:"
    echo "  start [--build]    Start services (optional: rebuild)"
    echo "  stop               Stop services"
    echo "  restart            Restart services"
    echo "  logs [service]     View logs (optional: specify service name backend/frontend)"
    echo "  status             View service status"
    echo "  clean              Clean all containers and data"
    echo "  update             Update code and restart"
    echo "  help               Show this help message"
    echo ""
    echo "Examples:"
    echo "  ./start.sh start --build    # Build and start"
    echo "  ./start.sh logs backend     # View backend logs"
    echo "  ./start.sh status           # View status"
}

# ------------------------------------------------------------------------
# Main: Command Dispatcher
# ------------------------------------------------------------------------
main() {
    check_docker

    case "${1:-start}" in
        start)
            check_env
            check_config
            check_database
            start "$2"
            ;;
        stop)
            stop
            ;;
        restart)
            restart
            ;;
        logs)
            logs "$@"
            ;;
        status)
            status
            ;;
        clean)
            clean
            ;;
        update)
            update
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "Unknown command: $1"
            show_help
            exit 1
            ;;
    esac
}

# Execute Main
main "$@"