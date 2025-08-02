#!/usr/bin/env powershell

Write-Host "🔧 验证中间件系统修复..."

Write-Host "1. 检查编译错误..."
$compileResult = go build 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ 编译成功" -ForegroundColor Green
} else {
    Write-Host "❌ 编译失败:" -ForegroundColor Red
    Write-Host $compileResult
    exit 1
}

Write-Host "2. 运行中间件测试..."
$testResult = go test -v -run "TestMiddlewareChain" 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ 测试通过" -ForegroundColor Green
    Write-Host $testResult
} else {
    Write-Host "❌ 测试失败:" -ForegroundColor Red
    Write-Host $testResult
}

Write-Host "3. 运行测试程序..."
Set-Location test_middleware
$runResult = go run main.go 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ 测试程序运行成功" -ForegroundColor Green
    Write-Host $runResult
} else {
    Write-Host "❌ 测试程序运行失败:" -ForegroundColor Red
    Write-Host $runResult
}
Set-Location ..

Write-Host "🎉 验证完成!"
