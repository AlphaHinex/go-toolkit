#!/bin/bash

rm -rf build/
dirs=`ls -F | grep "/$"`
for dir in $dirs; do
  name=${dir/\//}
  GOOS=windows GOARCH=amd64 go build -o build/${name}_win_amd64.exe $name/$name.go
  GOOS=darwin GOARCH=amd64 go build -o build/${name}_darwin_amd64 $name/$name.go
  GOOS=darwin GOARCH=arm64 go build -o build/${name}_darwin_arm64 $name/$name.go
  GOOS=linux GOARCH=amd64 go build -o build/${name}_linux_amd64 $name/$name.go
  GOOS=linux GOARCH=arm64 go build -o build/${name}_linux_arm64 $name/$name.go
done
