BIN_DIR := bin
BINARY_NAME := kubectl-net
ALIAS_NAME := knet

.PHONY: all build install test clean

all: build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "-s -w" -o $(BIN_DIR)/$(BINARY_NAME) .
	@ln -sf $(BINARY_NAME) $(BIN_DIR)/$(ALIAS_NAME)
	@echo "Build complete: $(BIN_DIR)/$(BINARY_NAME) (and alias $(BIN_DIR)/$(ALIAS_NAME))"

install: build
	@echo "Installing $(BINARY_NAME) and $(ALIAS_NAME) to /usr/local/bin..."
	sudo cp $(BIN_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	sudo ln -sf /usr/local/bin/$(BINARY_NAME) /usr/local/bin/$(ALIAS_NAME)
	@echo "Installed successfully! You can run 'kubectl net <subcommand>', 'kubectl-net', or 'knet'."

test:
	go test -v ./...

clean:
	rm -rf $(BIN_DIR)
