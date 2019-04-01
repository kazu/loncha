test: prepare-test
	go test ./example

prepare-test:
	go build -o gentmpl cmd/gen/main.go
	./gentmpl example/sample_def.go example Sample container_list/container_list.gtpl > example/sample_list.go
