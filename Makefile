.PHONY: init grpc install download

init:
	go install github.com/golang/protobuf/protoc-gen-go@latest

grpc:
	protoc --go_out=plugins=grpc:. proto/*.proto

