. (Join-Path $PSScriptRoot "bootstrap-utf8.ps1")

$ErrorActionPreference = "Stop"

$composeFile = Join-Path $PSScriptRoot "..\..\infra\dev\docker-compose.yml"
$candidateImages = @(
  "postgres:15-alpine",
  "postgres:18-alpine",
  "docker.1ms.run/library/postgres:16",
  "swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/postgres:16",
  "postgres:16"
)

$selectedImage = $null
foreach ($image in $candidateImages) {
  Write-Host "尝试拉取 PostgreSQL 镜像: $image"
  docker pull $image
  if ($LASTEXITCODE -eq 0) {
    $selectedImage = $image
    break
  }
}

if (-not $selectedImage) {
  throw "未能拉取任何可用的 PostgreSQL 镜像"
}

$env:POSTGRES_IMAGE = $selectedImage
docker compose -f $composeFile up -d postgres
