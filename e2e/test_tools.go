package e2e

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/streamable-mcp/schema"
	"github.com/modelcontextprotocol/streamable-mcp/server"
)

// RegisterTestTools 注册所有测试工具到服务器
func RegisterTestTools(s *server.Server) {
	// 注册基本的问候工具
	if err := s.RegisterTool(NewBasicTool()); err != nil {
		panic(fmt.Sprintf("注册 BasicTool 失败: %v", err))
	}

	// 注册流式工具
	if err := s.RegisterTool(NewStreamingTool()); err != nil {
		panic(fmt.Sprintf("注册 StreamingTool 失败: %v", err))
	}

	// 注册故障工具
	if err := s.RegisterTool(NewErrorTool()); err != nil {
		panic(fmt.Sprintf("注册 ErrorTool 失败: %v", err))
	}

	// 注册延迟工具
	if err := s.RegisterTool(NewDelayTool()); err != nil {
		panic(fmt.Sprintf("注册 DelayTool 失败: %v", err))
	}

	// 注册 SSE 进度工具
	if err := s.RegisterTool(NewSSEProgressTool()); err != nil {
		panic(fmt.Sprintf("注册 SSEProgressTool 失败: %v", err))
	}
}

// NewBasicTool 创建一个简单的问候工具
func NewBasicTool() *schema.Tool {
	return schema.NewTool("basic-greet",
		func(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
			// 检查上下文是否已取消
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				// 继续执行
			}

			// 提取名称参数
			name := "世界"
			if nameArg, ok := req.Params.Arguments["name"]; ok {
				if nameStr, ok := nameArg.(string); ok && nameStr != "" {
					name = nameStr
				}
			}

			// 创建问候消息
			greeting := fmt.Sprintf("你好，%s！这是一个测试消息。", name)

			// 创建工具结果
			return &schema.CallToolResult{
				Content: []schema.ToolContent{
					schema.NewTextContent(greeting),
				},
			}, nil
		},
		schema.WithDescription("一个简单的问候工具，返回问候消息"),
		schema.WithString("name",
			schema.Description("要问候的名称"),
		),
	)
}

// NewStreamingTool 创建一个产生多条消息的流式工具
func NewStreamingTool() *schema.Tool {
	return schema.NewTool("streaming-greet",
		func(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
			// 提取参数
			name := "世界"
			if nameArg, ok := req.Params.Arguments["name"]; ok {
				if nameStr, ok := nameArg.(string); ok && nameStr != "" {
					name = nameStr
				}
			}

			count := 3
			if countArg, ok := req.Params.Arguments["count"]; ok {
				if countInt, ok := countArg.(float64); ok && countInt > 0 {
					count = int(countInt)
				}
			}

			// 创建多条消息
			content := make([]schema.ToolContent, 0, count)
			for i := 1; i <= count; i++ {
				select {
				case <-ctx.Done():
					return &schema.CallToolResult{Content: content}, ctx.Err()
				default:
					// 继续执行
				}

				// 创建问候消息
				greeting := fmt.Sprintf("流式消息 %d/%d: 你好，%s！", i, count, name)
				content = append(content, schema.NewTextContent(greeting))

				// 添加一个简单的延迟模拟流式传输
				time.Sleep(100 * time.Millisecond)
			}

			return &schema.CallToolResult{Content: content}, nil
		},
		schema.WithDescription("一个流式问候工具，返回多条问候消息"),
		schema.WithString("name",
			schema.Description("要问候的名称"),
		),
		schema.WithNumber("count",
			schema.Description("生成的消息数量"),
			schema.Default(3),
		),
	)
}

// NewErrorTool 创建一个总是返回错误的工具
func NewErrorTool() *schema.Tool {
	return schema.NewTool("error-tool",
		func(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
			// 提取错误消息
			errorMsg := "这是一个故意的错误"
			if msgArg, ok := req.Params.Arguments["error_message"]; ok {
				if msgStr, ok := msgArg.(string); ok && msgStr != "" {
					errorMsg = msgStr
				}
			}

			// 直接返回错误
			return nil, errors.New(errorMsg)
		},
		schema.WithDescription("一个总是返回错误的工具"),
		schema.WithString("error_message",
			schema.Description("要返回的错误消息"),
			schema.Default("这是一个故意的错误"),
		),
	)
}

// NewDelayTool 创建一个会延迟指定时间的工具
func NewDelayTool() *schema.Tool {
	return schema.NewTool("delay-tool",
		func(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
			// 提取延迟时间
			delayMs := 1000
			if delayArg, ok := req.Params.Arguments["delay_ms"]; ok {
				if delayInt, ok := delayArg.(float64); ok && delayInt > 0 {
					delayMs = int(delayInt)
				}
			}

			// 提取消息
			message := "延迟结束"
			if msgArg, ok := req.Params.Arguments["message"]; ok {
				if msgStr, ok := msgArg.(string); ok && msgStr != "" {
					message = msgStr
				}
			}

			// 创建定时器
			timer := time.NewTimer(time.Duration(delayMs) * time.Millisecond)
			defer timer.Stop()

			// 等待定时器或上下文取消
			select {
			case <-timer.C:
				// 定时器到期，返回结果
				return &schema.CallToolResult{
					Content: []schema.ToolContent{
						schema.NewTextContent(fmt.Sprintf("%s（延迟%dms）", message, delayMs)),
					},
				}, nil
			case <-ctx.Done():
				// 上下文取消，返回错误
				return nil, ctx.Err()
			}
		},
		schema.WithDescription("一个会延迟指定时间的工具"),
		schema.WithNumber("delay_ms",
			schema.Description("延迟的毫秒数"),
			schema.Default(1000),
		),
		schema.WithString("message",
			schema.Description("延迟后返回的消息"),
			schema.Default("延迟结束"),
		),
	)
}

// NewSSEProgressTool 创建一个支持发送进度通知的 SSE 测试工具
func NewSSEProgressTool() *schema.Tool {
	return schema.NewTool("sse-progress-tool",
		func(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
			// 提取参数
			steps := 5
			if stepsArg, ok := req.Params.Arguments["steps"]; ok {
				if stepsFloat, ok := stepsArg.(float64); ok && stepsFloat > 0 {
					steps = int(stepsFloat)
				}
			}

			delayMs := 100
			if delayArg, ok := req.Params.Arguments["delay_ms"]; ok {
				if delayFloat, ok := delayArg.(float64); ok && delayFloat > 0 {
					delayMs = int(delayFloat)
				}
			}

			message := "SSE 进度测试完成"
			if msgArg, ok := req.Params.Arguments["message"]; ok {
				if msgStr, ok := msgArg.(string); ok && msgStr != "" {
					message = msgStr
				}
			}

			// 发送进度通知
			for i := 1; i <= steps; i++ {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
					// 计算进度百分比
					progress := float64(i) / float64(steps)
					// 发送进度通知
					if sender, ok := schema.GetNotificationSender(ctx); ok {
						err := sender.SendProgress(progress, fmt.Sprintf("步骤 %d/%d", i, steps))
						if err != nil {
							return nil, fmt.Errorf("发送进度通知失败: %v", err)
						}
						// 发送日志消息
						sender.SendLogMessage("info", fmt.Sprintf("完成步骤 %d", i))
					}
					// 等待指定延迟
					time.Sleep(time.Duration(delayMs) * time.Millisecond)
				}
			}

			// 返回最终结果
			return &schema.CallToolResult{
				Content: []schema.ToolContent{
					schema.NewTextContent(message),
				},
			}, nil
		},
		schema.WithDescription("一个 SSE 进度工具，使用 SSE 发送进度通知"),
		schema.WithNumber("steps",
			schema.Description("进度步骤数量"),
			schema.Default(5),
		),
		schema.WithNumber("delay_ms",
			schema.Description("每个步骤之间的延迟毫秒数"),
			schema.Default(100),
		),
		schema.WithString("message",
			schema.Description("响应消息"),
			schema.Default("SSE 进度测试完成"),
		),
	)
}

// HandleCustomNotification 处理自定义通知
func (nc *NotificationCollector) HandleCustomNotification(notification *schema.Notification) error {
	nc.addNotification(notification)
	return nil
}

// HandleNotification 处理标准通知
func (nc *NotificationCollector) HandleNotification(notification *schema.Notification) error {
	nc.addNotification(notification)
	return nil
}
