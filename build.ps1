# 确保脚本在遇到错误时立即停止执行
$ErrorActionPreference = "Stop"

git pull

Set-Location new_web
bun install
bun run build

Set-Location ..
if (Test-Path ./web/dist) {
    Remove-Item -Recurse -Force ./web/dist
}

Copy-Item -Recurse -Force ./new_web/dist ./web/dist

go build -o nachoai.exe