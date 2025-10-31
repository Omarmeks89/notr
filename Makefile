LINTER_SO_PATH = 		./lib/notr.so
PATH_TO_PLUGIN_SRC = 	./plugin/main.go
SOURCE_PATH = 			./cmd/notr/main.go
PATH_TO_BIN = 			./.build/bin
LINTER_NAME = 			notr

GO_FLAGS = 				-ldflags="-s -w"

.PHONY: init build plugin install clean

init:
# create required directories
	@mkdir -v -p ./.build/bin

build:
	@go build $(GO_FLAGS) -o $(PATH_TO_BIN)/$(LINTER_NAME) $(SOURCE_PATH)

install:
# install linter to use with go vet command
	@go install ./cmd/...

plugin:
	@go build -buildmode=plugin -o ${LINTER_SO_PATH} ${PATH_TO_PLUGIN_SRC}

vet:
#
# -c=N is a number of lines before | after
# (about shell commands: https://stackoverflow.com/questions/67969635/execute-shell-commands-inside-a-target-in-makefile)
#
	@go vet -c=5 ./...
	@go vet -c=5 -vettool=$(shell which notr) ./pkg/notr/testdata/src/

clean:
	rm -rf ${LINTER_SO_PATH}/lib/notr.so $(LINTER_NAME) $(PATH_TO_BIN)/$(LINTER_NAME)

