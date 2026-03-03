#!/bin/bash
set -e
echo "Installing kubesentinel..."
mkdir -p ~/.local/bin
curl -sL "https://github.com/dablon/kubesentinel/releases/download/v1.0.0/kubesentinel-linux-amd64" -o ~/.local/bin/kubesentinel
chmod +x ~/.local/bin/kubesentinel
echo "Installed to ~/.local/bin/kubesentinel"
