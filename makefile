
build: build-src run

build-src:
	go mod tidy
	go build

run:
	fuse-demo ./mnt
    fusermount3 -u ./mnt
