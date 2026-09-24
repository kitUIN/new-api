# 确保脚本在遇到错误时立即停止执行
$ErrorActionPreference = "Stop"

git pull

Set-Location web
bun install
bun run build

Set-Location ..
go build -o nachoai.exe