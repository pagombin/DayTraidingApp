#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ╔══════════════════════════════════════════════╗"
echo "  ║     Trading Platform — Setup Wizard          ║"
echo "  ║     Estimated time: 5 minutes                ║"
echo "  ╚══════════════════════════════════════════════╝"
echo ""

# Step 1: Check prerequisites
echo "Checking prerequisites..."

if ! command -v docker &> /dev/null; then
    echo ""
    echo "Docker is required but not installed."
    echo "Would you like to install Docker now? (y/n)"
    read -r install_docker
    if [ "$install_docker" = "y" ]; then
        if [[ "$OSTYPE" == "darwin"* ]]; then
            echo "Please install Docker Desktop from https://docker.com/products/docker-desktop"
            echo "Then run this script again."
            exit 1
        elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
            curl -fsSL https://get.docker.com | sh
            sudo usermod -aG docker $USER
            echo "Docker installed. You may need to log out and back in for group changes."
        fi
    else
        echo "Docker is required. Exiting."
        exit 1
    fi
fi

if ! docker compose version &> /dev/null; then
    echo "Docker Compose V2 is required but not found."
    echo "Please update Docker Desktop or install the compose plugin."
    exit 1
fi

echo "✓ Docker is installed"
echo "✓ Docker Compose is available"
echo ""

# Step 2: Check Python 3
if ! command -v python3 &> /dev/null; then
    echo "Python 3 is needed for the setup wizard."
    echo "Installing minimal Python environment..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install python3 2>/dev/null || {
            echo "Please install Python 3: https://python.org/downloads"
            exit 1
        }
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        sudo apt-get update && sudo apt-get install -y python3 python3-pip
    fi
fi

# Step 3: Create virtual environment and install wizard dependencies
VENV_DIR=".venv"
if [ ! -d "$VENV_DIR" ]; then
    echo "Creating Python virtual environment..."
    python3 -m venv "$VENV_DIR"
fi
source "$VENV_DIR/bin/activate"
pip install rich questionary pyyaml requests --quiet || {
    echo "Failed to install Python dependencies."
    exit 1
}
echo "✓ Python dependencies installed"

# Step 4: Run the interactive wizard
python3 wizard/wizard.py
deactivate 2>/dev/null || true

# Step 5: Start the platform
echo ""
echo "Starting Trading Platform..."
docker compose up -d --build

# Step 6: Wait for services to be healthy
echo "Waiting for services to start..."
sleep 10

for i in {1..30}; do
    if curl -sf http://localhost:8080/healthz > /dev/null 2>&1; then
        echo "✓ API Gateway is healthy"
        break
    fi
    sleep 2
done

for i in {1..30}; do
    if curl -sf http://localhost:3000 > /dev/null 2>&1; then
        echo "✓ Dashboard is ready"
        break
    fi
    sleep 2
done

echo ""
echo "  ╔══════════════════════════════════════════════╗"
echo "  ║     ✅ Trading Platform is running!           ║"
echo "  ║                                              ║"
echo "  ║     Dashboard:  http://localhost:3000         ║"
echo "  ║     API:        http://localhost:8080         ║"
echo "  ║                                              ║"
echo "  ║     Paper trading mode is active.            ║"
echo "  ║     No real money will be used.              ║"
echo "  ╚══════════════════════════════════════════════╝"
echo ""

if command -v open &> /dev/null; then
    open http://localhost:3000
elif command -v xdg-open &> /dev/null; then
    xdg-open http://localhost:3000
fi
