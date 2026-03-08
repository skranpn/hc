deps:
	go install github.com/Songmu/gocredits/cmd/gocredits@latest

credits: deps
	gocredits -skip-missing -w .
	git add CHANGELOG.md CREDITS go.mod go.sum
