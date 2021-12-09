DEV_IMAGE=dev-go:latest
NAME=emt

docker stop $NAME
docker rm $NAME

docker run \
	--name $NAME \
	--rm \
	-it \
	-e "GO111MODULE=on" \
	-e "GOPROXY=https://goproxy.cn" \
	-p 7654:7654 \
	-v $PWD:/go/src/$NAME -w /go/src/$NAME \
	$DEV_IMAGE \
	/bin/bash
