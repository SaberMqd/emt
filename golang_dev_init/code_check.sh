#bin/bash

echo "begin gofmt"

for f in `ls`;do
	if [ "$f" = "vendor" ];then
		echo "gofmt skip $f"
	elif [ "${f##*.}" = "go" ];then
		`gofmt -l -d -w $f >> gofmt.log`
	elif [ -d "$f" ];then
		`gofmt -l -d -w $f >> gofmt.log`
	fi
done

cat gofmt.log
rm gofmt.log

echo "begin go vet"
go vet ./...
 
echo "begin golangci-lint"

golangci-lint run --enable-all -D testpackage -D funlen -D exhaustivestruct -D staticcheck -D lll -D gochecknoinits -D interfacer -D gosec

