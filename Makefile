BIN_DIR := bin
BINARY_NAME := kcap
KUBECTL_PLUGIN_NAME := kubectl-cap

.PHONY: all build install test clean

all: build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "-s -w" -o $(BIN_DIR)/$(BINARY_NAME) .
	@echo "Build complete: $(BIN_DIR)/$(BINARY_NAME)"

install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BIN_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	sudo ln -sf /usr/local/bin/$(BINARY_NAME) /usr/local/bin/$(KUBECTL_PLUGIN_NAME)
	@echo "Installed successfully! You can run 'kcap' or 'kubectl cap'."

test:
	go test -v ./...

clean:
	rm -rf $(BIN_DIR)
