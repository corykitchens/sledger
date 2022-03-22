IMG ?= sledger
all: build

fmt:
	go fmt ./...
vet:
	go vet ./...

install:
	go install

build:
	go build -o sledger

clean:
	rm sledger

docker-build:
	docker build -t ${IMG} .

docker-push:
	docker push ${IMG}

docker-run:
	docker run --rm ${IMG} ${IMG}