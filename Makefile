BIN_DIR := bin
BINARY_NAME := kubectl-net
COMPLETION_HELPER := kubectl_complete-net
INSTALL_DIR ?= $(HOME)/.local/bin

.PHONY: all build install uninstall test clean

all: build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "-s -w" -o $(BIN_DIR)/$(BINARY_NAME) .
	@ln -sf $(BINARY_NAME) $(BIN_DIR)/$(COMPLETION_HELPER)
	@echo "Build complete: $(BIN_DIR)/$(BINARY_NAME)"

install: build
	@mkdir -p $(INSTALL_DIR)
	@echo "Installing $(BINARY_NAME) and $(COMPLETION_HELPER) to $(INSTALL_DIR)..."
	cp $(BIN_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	ln -sf $(BINARY_NAME) $(INSTALL_DIR)/$(COMPLETION_HELPER)
	@echo "Installed successfully to $(INSTALL_DIR)/$(BINARY_NAME)!"
	@echo "Ensure $(INSTALL_DIR) is in your PATH to run 'kubectl net <subcommand>'."

uninstall:
	@echo "Uninstalling $(BINARY_NAME) and $(COMPLETION_HELPER) from $(INSTALL_DIR)..."
	rm -f $(INSTALL_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(COMPLETION_HELPER)
	@echo "Uninstalled successfully."

test:
	go test -v ./...

clean:
	rm -rf $(BIN_DIR)
