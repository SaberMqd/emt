NAME=emt
EXAMPLE_NAME_GAME=game
EXAMPLE_NAME_ECHO=echo
EXAMPLE_NAME_FRAME=frame_game

BUILD_DATE=$(shell date +%s)

TEST_DOCKER_URL=/$(NAME):latest
PROD_DOCKER_URL=/$(NAME):v0.0.1

DEV_IMAGE=dev-go:latest
GO_PROXY=https://goproxy.cn/

all: svr_demo emt_demo

git_sub_init:
	git submodule add xxx/golang_dev_init.git

git_sub_update:
	git submodule init && git submodule update
	git submodule foreach 'git pull origin master'

git_sub_update_remote:
	git submodule update --remote

image:
	cp ./config/server.json ./bin/server.json
	docker build -t $(NAME):$(BUILD_DATE) .

remote_test_image:
	cp ./config/server_test.json ./bin/server.json
	docker build -t $(TEST_DOCKER_URL) .
	docker push $(TEST_DOCKER_URL)

remote_prod_image:
	cp ./config/server_prod.json ./bin/server.json
	docker build -t $(PROD_DOCKER_URL) .
	docker push $(PROD_DOCKER_URL)

.PHONY:
cleanall:
	rm go.mod | rm go.sum | rm tmp123 -rf | rm bin -rf | rm vendor -rf 
clean:
	rm bin -rf
