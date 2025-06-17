.PHONY: build clean mount unmount test

BUILD_DIR=bin
BINARY=aethelfsd
MOUNT_POINT=/mnt/aethelfs

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) cmd/aethelfsd/main.go

clean:
	rm -rf $(BUILD_DIR)

# Interactive mount target that lets user choose from available DAX devices
# Note: the file system is running in foreground, but can be changed to run in background
mount: build
	@mkdir -p $(MOUNT_POINT)
	@echo "Checking for available DAX devices..."
	@devices=$$(ls -1 /dev/dax* 2>/dev/null); \
	if [ -z "$$devices" ]; then \
		echo "No DAX devices found. Please ensure DAX devices are available."; \
		exit 1; \
	fi; \
	echo "Available DAX devices:"; \
	i=1; \
	for dev in $$devices; do \
		echo "$$i) $$dev"; \
		i=$$((i+1)); \
	done; \
	echo "Enter the number of the DAX device to use:"; \
	read choice; \
	chosen_dev=$$(echo "$$devices" | sed -n "$${choice}p"); \
	if [ -z "$$chosen_dev" ]; then \
		echo "Invalid selection"; \
		exit 1; \
	fi; \
	echo "Using DAX device: $$chosen_dev"; \
	sudo $(BUILD_DIR)/$(BINARY) $$chosen_dev $(MOUNT_POINT)

mount-debug: build
	@mkdir -p $(MOUNT_POINT)
	@echo "Checking for available DAX devices..."
	@devices=$$(ls -1 /dev/dax* 2>/dev/null); \
	if [ -z "$$devices" ]; then \
		echo "No DAX devices found. Please ensure DAX devices are available."; \
		exit 1; \
	fi; \
	echo "Available DAX devices:"; \
	i=1; \
	for dev in $$devices; do \
		echo "$$i) $$dev"; \
		i=$$((i+1)); \
	done; \
	echo "Enter the number of the DAX device to use:"; \
	read choice; \
	chosen_dev=$$(echo "$$devices" | sed -n "$${choice}p"); \
	if [ -z "$$chosen_dev" ]; then \
		echo "Invalid selection"; \
		exit 1; \
	fi; \
	echo "Using DAX device: $$chosen_dev"; \
	sudo $(BUILD_DIR)/$(BINARY) -debug $$chosen_dev $(MOUNT_POINT)

unmount:
	@echo "Unmounting filesystem from $(MOUNT_POINT)..."
	@sudo fusermount -u $(MOUNT_POINT) || echo "No filesystem mounted at $(MOUNT_POINT)"
	@echo "finished."

test: build
	@echo "Running Go unit tests..."
	go test ./...
	@echo "Running filesystem integration tests..."
	@./scripts/test_aethelfs.sh