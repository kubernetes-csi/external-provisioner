# We use the http://plantuml.com/plantuml server to generate
# images. That way nothing needs to be installed besides Go.
# The client tool has no license, so we can't vendor it.
# Instead we "go get" it if (and only if) needed.
DOC_PLANTUML_GO = $(shell go env GOPATH)/bin/plantuml-go

%.png: %.puml $(DOC_PLANTUML_GO)
	$(DOC_PLANTUML_GO) -format png $<

# Builds the binary in GOPATH/bin. Changing into / first avoids
# modifying the project's go.mod file.
$(DOC_PLANTUML_GO):
	cd / && go get github.com/acarlson99/plantuml-go
