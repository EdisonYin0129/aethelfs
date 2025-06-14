#!/bin/bash
# scripts/setup_dax.sh - Helper script for DAX device setup
#
# Usage:
#   chmod +x setup_dax.sh
#   ./setup_dax.sh
#
# This script provides guidance for setting up DAX devices
# and creates a test file for development purposes if needed.

echo "AethelFS DAX Device Setup Guide"
echo "==============================="
echo
echo "System DAX devices available on this system:"
ls -l /dev/dax*

echo
echo "For testing without a real DAX device, you can create a file:"
echo "  sudo mkdir -p /mnt/pmem0"
echo "  sudo dd if=/dev/zero of=/mnt/pmem0/daxfile bs=1M count=128"
echo "  sudo chmod 666 /mnt/pmem0/daxfile"
echo
echo "Then use this file as your DAX device:"
echo "  sudo bin/aethelfsd /mnt/pmem0/daxfile /mnt/aethelfs"
echo
echo "Would you like to create a test file now? (y/n)"
read -r response
if [[ "$response" =~ ^([yY][eE][sS]|[yY])$ ]]; then
    sudo mkdir -p /mnt/pmem0
    sudo dd if=/dev/zero of=/mnt/pmem0/daxfile bs=1M count=128
    sudo chmod 666 /mnt/pmem0/daxfile
    echo "Test file created at /mnt/pmem0/daxfile"
fi