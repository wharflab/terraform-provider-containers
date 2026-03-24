default: build

build:
	go build -o terraform-provider-containers

install: build
	mkdir -p ~/.terraform.d/plugins/registry.terraform.io/wharflab/containers/0.1.0/$$(go env GOOS)_$$(go env GOARCH)
	cp terraform-provider-containers ~/.terraform.d/plugins/registry.terraform.io/wharflab/containers/0.1.0/$$(go env GOOS)_$$(go env GOARCH)/

test:
	go test ./... -v -count=1

testacc:
	TF_ACC=1 go test ./... -v -count=1 -timeout 120m

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .

vet:
	go vet ./...

generate:
	go generate ./...

clean:
	rm -f terraform-provider-containers

.PHONY: build install test testacc lint fmt vet generate clean
