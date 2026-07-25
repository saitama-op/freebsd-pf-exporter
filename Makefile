BINARY_NAME=freebsd-pf-exporter

all: build

build:
	GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ${BINARY_NAME} cmd/exporter/main.go

clean:
	go clean
	rm -f ${BINARY_NAME}
