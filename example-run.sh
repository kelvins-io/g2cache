go mod init
go mod tidy
go mod vendor
cd ./example || exit
echo "[g2cache.example] 使用默认的redis配置"
go run main.go