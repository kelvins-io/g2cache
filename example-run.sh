rm -rf vendor
rm -rf go.mod
rm -rf go.sum
go mod init
go mod tidy
go mod vendor
cd ./example || exit
rm -rf g2cache-example
go build -o g2cache-example main.go
./g2cache-example