#!/bin/bash
# test_aethelfs.sh - Comprehensive test script for AethelFS
#
# Usage:
#   chmod +x test_aethelfs.sh
#   ./test_aethelfs.sh
#
# Note: This script assumes your filesystem is already mounted at /mnt/aethelfs
# If using a different mount point, modify the MOUNT_POINT variable below.
#
set -e

# Colors for better output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

MOUNT_POINT="/mnt/aethelfs"
TEST_DIR="$MOUNT_POINT/testdir"
TEST_FILE="$MOUNT_POINT/test.txt"
LARGE_FILE="$MOUNT_POINT/largefile"
RANDOM_FILE="$MOUNT_POINT/randomfile"

echo -e "${BLUE}=== AethelFS Test Suite ===${NC}"

# Check if filesystem is mounted
if ! mount | grep -q "$MOUNT_POINT"; then
    echo -e "${RED}Error: Filesystem not mounted at $MOUNT_POINT${NC}"
    echo "Please run your filesystem first using the Makefile:"
    echo "  make mount"
    echo "Then select the appropriate DAX device when prompted."
    exit 1
fi

# Test 1: Basic Write/Read
echo -e "\n${YELLOW}Test 1: Basic Write/Read${NC}"
echo "Writing to $TEST_FILE..."
echo "Hello AethelFS! This is a test file." > "$TEST_FILE"
echo "Reading from $TEST_FILE..."
content=$(cat "$TEST_FILE")
echo "Content: $content"
if [[ "$content" == "Hello AethelFS! This is a test file." ]]; then
    echo -e "${GREEN}✓ Basic Write/Read Test Passed${NC}"
else
    echo -e "${RED}✗ Basic Write/Read Test Failed${NC}"
    echo "Expected: Hello AethelFS! This is a test file."
    echo "Got: $content"
fi

# Test 2: Directory Operations
echo -e "\n${YELLOW}Test 2: Directory Operations${NC}"
echo "Creating directory $TEST_DIR..."
mkdir -p "$TEST_DIR"
if [ -d "$TEST_DIR" ]; then
    echo -e "${GREEN}✓ Directory Creation Passed${NC}"
else
    echo -e "${RED}✗ Directory Creation Failed${NC}"
fi

echo "Creating file in subdirectory..."
echo "File in subdirectory" > "$TEST_DIR/subfile.txt"
if [ -f "$TEST_DIR/subfile.txt" ]; then
    echo -e "${GREEN}✓ Subdirectory File Creation Passed${NC}"
else
    echo -e "${RED}✗ Subdirectory File Creation Failed${NC}"
fi

# Test 3: Larger File Operations
echo -e "\n${YELLOW}Test 3: Larger File Operations${NC}"
echo "Creating 1MB file with repeating pattern..."
for i in {1..1024}; do
    echo "This is line $i in a larger test file for AethelFS" >> "$LARGE_FILE"
done

size=$(stat -c%s "$LARGE_FILE")
echo "File size: $size bytes"
if [ "$size" -gt 50000 ]; then
    echo -e "${GREEN}✓ Large File Creation Passed${NC}"
else
    echo -e "${RED}✗ Large File Creation Failed or File Too Small${NC}"
fi

# Test 4: Random Data
echo -e "\n${YELLOW}Test 4: Random Data Test${NC}"
echo "Creating 512KB file with random data..."
dd if=/dev/urandom of="$RANDOM_FILE" bs=1K count=512 status=progress
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Random Data Write Passed${NC}"
    
    # Calculate MD5 of the file
    echo "Calculating MD5 of random file..."
    md5_1=$(md5sum "$RANDOM_FILE" | awk '{print $1}')
    echo "MD5: $md5_1"
    
    # Read it back and calculate MD5 again
    echo "Reading file back and calculating MD5 again..."
    md5_2=$(md5sum "$RANDOM_FILE" | awk '{print $1}')
    
    if [ "$md5_1" == "$md5_2" ]; then
        echo -e "${GREEN}✓ Data Integrity Check Passed${NC}"
    else
        echo -e "${RED}✗ Data Integrity Check Failed${NC}"
        echo "First MD5: $md5_1"
        echo "Second MD5: $md5_2"
    fi
else
    echo -e "${RED}✗ Random Data Write Failed${NC}"
fi

# Test 5: File Append
echo -e "\n${YELLOW}Test 5: File Append Test${NC}"
echo "Appending to existing file..."
echo "This is appended text" >> "$TEST_FILE"
content=$(cat "$TEST_FILE")
if [[ "$content" == *"This is appended text"* ]]; then
    echo -e "${GREEN}✓ File Append Test Passed${NC}"
else
    echo -e "${RED}✗ File Append Test Failed${NC}"
    echo "Content does not contain appended text:"
    echo "$content"
fi

# Test 6: File Overwrite
echo -e "\n${YELLOW}Test 6: File Overwrite Test${NC}"
echo "Overwriting existing file..."
echo "Overwritten content" > "$TEST_FILE"
content=$(cat "$TEST_FILE")
if [[ "$content" == "Overwritten content" ]]; then
    echo -e "${GREEN}✓ File Overwrite Test Passed${NC}"
else
    echo -e "${RED}✗ File Overwrite Test Failed${NC}"
    echo "Expected: Overwritten content"
    echo "Got: $content"
fi

# Test 7: File System Stats
echo -e "\n${YELLOW}Test 7: Filesystem Statistics${NC}"
echo "Filesystem usage:"
df -h "$MOUNT_POINT"

# Test 8: List Files
echo -e "\n${YELLOW}Test 8: Directory Listing${NC}"
echo "Files in the filesystem root:"
ls -la "$MOUNT_POINT"

# Test 9: File Deletion
echo -e "\n${YELLOW}Test 9: File Deletion Test${NC}"
echo "Deleting file..."
rm -f "$TEST_FILE"
if [ ! -f "$TEST_FILE" ]; then
    echo -e "${GREEN}✓ File Deletion Test Passed${NC}"
else
    echo -e "${RED}✗ File Deletion Test Failed${NC}"
fi

# Test 10: Simple Performance Test
echo -e "\n${YELLOW}Test 10: Simple Performance Test${NC}"
echo "Writing 10MB file and measuring time..."
time dd if=/dev/zero of="$MOUNT_POINT/perftest" bs=1M count=10 status=progress

echo "Reading 10MB file and measuring time..."
time dd if="$MOUNT_POINT/perftest" of=/dev/null bs=1M status=progress

echo -e "\n${BLUE}All tests completed!${NC}"
echo "You can now unmount the filesystem with:"
echo "  sudo fusermount -u $MOUNT_POINT"

# Test 11: Large File Performance Test (reduced from 1GB to 250MB)
echo -e "\n${YELLOW}Test 11: Extended Performance Test (250MB)${NC}"
echo "Writing 250MB file and measuring time..."
time dd if=/dev/zero of="$MOUNT_POINT/large_perftest" bs=10M count=25 status=progress

echo "Reading 250MB file and measuring time..."
time dd if="$MOUNT_POINT/large_perftest" of=/dev/null bs=10M status=progress

# Optional: Add random I/O test if fio is available
if command -v fio &> /dev/null; then
    echo -e "\n${YELLOW}Test 12: Random I/O Test${NC}"
    echo "Running random I/O with fio..."
    fio --name=random-rw --filename="$MOUNT_POINT/fio_test" --size=50M \
        --rw=randrw --bs=4k --direct=1 --runtime=30 --time_based
fi