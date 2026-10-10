# Общие команды форматирования и проверки стиля проекта.
GOIMPORTS = go run golang.org/x/tools/cmd/goimports@v0.36.0
GOLINES = go run github.com/segmentio/golines@v0.13.0
GO_LINE_FLAGS = --base-formatter=gofmt --no-reformat-tags --max-len=100

.PHONY: format format-go format-check

format: format-go
	npm --prefix frontend run format

format-go:
	$(GOIMPORTS) -local workshop -w backend
	$(GOLINES) $(GO_LINE_FLAGS) --write-output backend

format-check:
	@set -e; files="$$($(GOIMPORTS) -local workshop -l backend)"; \
		if [ -n "$$files" ]; then printf '%s\n' "$$files"; exit 1; fi
	@set -e; files="$$($(GOLINES) $(GO_LINE_FLAGS) --list-files backend)"; \
		if [ -n "$$files" ]; then printf '%s\n' "$$files"; exit 1; fi
	npm --prefix frontend run format:check
