#!/usr/bin/env bash
set -e

APP_NAME="cluebot"
INSTALL_DIR="/usr/local/bin"
LIB_DIR="/var/lib/cluebot"
CONFIG_DIR="$LIB_DIR/configs"
BIN_PATH="build/$APP_NAME"

echo "=== ClueBot Build & Install ==="
echo ""

# Build
echo "[1/5] Building release binary..."
go build -ldflags="-s -w" -o "$BIN_PATH" ./cmd/cluebot
echo "      Built: $BIN_PATH"

# Install binary
echo "[2/5] Installing binary to $INSTALL_DIR..."
sudo cp "$BIN_PATH" "$INSTALL_DIR/$APP_NAME"
sudo chmod +x "$INSTALL_DIR/$APP_NAME"
echo "      Installed: $INSTALL_DIR/$APP_NAME"

# Create data directories
echo "[3/5] Creating data directories..."
sudo mkdir -p "$LIB_DIR/logs"
sudo mkdir -p "$LIB_DIR/incidents"
sudo mkdir -p "$CONFIG_DIR"
echo "      Created: $LIB_DIR/logs"
echo "      Created: $LIB_DIR/incidents"
echo "      Created: $CONFIG_DIR"

# Install config (don't overwrite existing)
echo "[4/5] Installing config..."
if [ -f "$CONFIG_DIR/default.yaml" ]; then
    echo "      Config already exists at $CONFIG_DIR/default.yaml (not overwritten)"
else
    sudo cp configs/default.yaml "$CONFIG_DIR/default.yaml"
    echo "      Installed: $CONFIG_DIR/default.yaml"
fi

# Install systemd service
echo "[5/5] Installing systemd service..."
sudo cp scripts/cluebot.service /etc/systemd/system/cluebot.service
sudo systemctl daemon-reload
echo "      Installed: /etc/systemd/system/cluebot.service"

echo ""
echo "============================================="
echo "  ClueBot installed successfully!"
echo "============================================="
echo ""
echo "  Binary:    $INSTALL_DIR/$APP_NAME"
echo "  Config:    $CONFIG_DIR/default.yaml"
echo "  Data dir:  $LIB_DIR"
echo "  Logs:      $LIB_DIR/logs/"
echo "  Incidents: $LIB_DIR/incidents/"
echo ""
echo "  --- Quick Start ---"
echo ""
echo "  Edit config:"
echo "    sudo nano $CONFIG_DIR/default.yaml"
echo ""
echo "  Enable and start:"
echo "    sudo systemctl enable cluebot"
echo "    sudo systemctl start cluebot"
echo ""
echo "  Check status:"
echo "    sudo systemctl status cluebot"
echo ""
echo "  Stop:"
echo "    sudo systemctl stop cluebot"
echo ""
echo "  View dashboard:"
echo "    Open http://<your-server-ip>:8090"
echo "    Default login: admin / admin"
echo ""
echo "  View logs:"
echo "    sudo journalctl -u cluebot -f"
echo ""
