#!/bin/bash

echo "=== Middleware Integration Test ==="
echo ""

# 测试正常的工具调用
echo "1. Testing normal tool call..."
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "test_tool",
      "arguments": {
        "message": "Hello Integration Test"
      }
    }
  }'

echo ""
echo ""

# 测试错误场景
echo "2. Testing error scenario..."
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "error_tool",
      "arguments": {
        "message": "Trigger Error"
      }
    }
  }'

echo ""
echo ""

# 测试工具列表
echo "3. Testing tools list..."
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/list",
    "params": {}
  }'

echo ""
echo ""

echo "=== Test Complete ==="