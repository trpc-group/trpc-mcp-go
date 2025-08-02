// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// MiddlewareMetrics 中间件性能指标
type MiddlewareMetrics struct {
	RequestCount    int64         `json:"request_count"`
	ErrorCount      int64         `json:"error_count"`
	TotalDuration   time.Duration `json:"total_duration"`
	AverageDuration time.Duration `json:"average_duration"`
	MaxDuration     time.Duration `json:"max_duration"`
	MinDuration     time.Duration `json:"min_duration"`
	LastRequest     time.Time     `json:"last_request"`
}

// MiddlewareMonitor 中间件监控器
type MiddlewareMonitor struct {
	metrics map[string]*MiddlewareMetrics
	mu      sync.RWMutex
}

// NewMiddlewareMonitor 创建新的中间件监控器
func NewMiddlewareMonitor() *MiddlewareMonitor {
	return &MiddlewareMonitor{
		metrics: make(map[string]*MiddlewareMetrics),
	}
}

// RecordRequest 记录请求指标
func (m *MiddlewareMonitor) RecordRequest(middlewareName string, duration time.Duration, hasError bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	metric, exists := m.metrics[middlewareName]
	if !exists {
		metric = &MiddlewareMetrics{
			MinDuration: duration,
			MaxDuration: duration,
		}
		m.metrics[middlewareName] = metric
	}
	
	atomic.AddInt64(&metric.RequestCount, 1)
	if hasError {
		atomic.AddInt64(&metric.ErrorCount, 1)
	}
	
	metric.TotalDuration += duration
	metric.AverageDuration = metric.TotalDuration / time.Duration(metric.RequestCount)
	metric.LastRequest = time.Now()
	
	if duration > metric.MaxDuration {
		metric.MaxDuration = duration
	}
	if duration < metric.MinDuration {
		metric.MinDuration = duration
	}
}

// GetMetrics 获取指定中间件的指标
func (m *MiddlewareMonitor) GetMetrics(middlewareName string) *MiddlewareMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if metric, exists := m.metrics[middlewareName]; exists {
		// 返回副本以避免并发访问问题
		return &MiddlewareMetrics{
			RequestCount:    atomic.LoadInt64(&metric.RequestCount),
			ErrorCount:      atomic.LoadInt64(&metric.ErrorCount),
			TotalDuration:   metric.TotalDuration,
			AverageDuration: metric.AverageDuration,
			MaxDuration:     metric.MaxDuration,
			MinDuration:     metric.MinDuration,
			LastRequest:     metric.LastRequest,
		}
	}
	return nil
}

// GetAllMetrics 获取所有中间件的指标
func (m *MiddlewareMonitor) GetAllMetrics() map[string]*MiddlewareMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[string]*MiddlewareMetrics)
	for name, metric := range m.metrics {
		result[name] = &MiddlewareMetrics{
			RequestCount:    atomic.LoadInt64(&metric.RequestCount),
			ErrorCount:      atomic.LoadInt64(&metric.ErrorCount),
			TotalDuration:   metric.TotalDuration,
			AverageDuration: metric.AverageDuration,
			MaxDuration:     metric.MaxDuration,
			MinDuration:     metric.MinDuration,
			LastRequest:     metric.LastRequest,
		}
	}
	return result
}

// Reset 重置指定中间件的指标
func (m *MiddlewareMonitor) Reset(middlewareName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.metrics, middlewareName)
}

// ResetAll 重置所有指标
func (m *MiddlewareMonitor) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = make(map[string]*MiddlewareMetrics)
}

// PrintReport 打印性能报告
func (m *MiddlewareMonitor) PrintReport() {
	metrics := m.GetAllMetrics()
	
	log.Println("=== 中间件性能报告 ===")
	for name, metric := range metrics {
		errorRate := float64(metric.ErrorCount) / float64(metric.RequestCount) * 100
		log.Printf("中间件: %s", name)
		log.Printf("  请求总数: %d", metric.RequestCount)
		log.Printf("  错误总数: %d (%.2f%%)", metric.ErrorCount, errorRate)
		log.Printf("  平均响应时间: %v", metric.AverageDuration)
		log.Printf("  最大响应时间: %v", metric.MaxDuration)
		log.Printf("  最小响应时间: %v", metric.MinDuration)
		log.Printf("  最后请求时间: %v", metric.LastRequest.Format("2006-01-02 15:04:05"))
		log.Println()
	}
}

// ToJSON 将指标转换为JSON格式
func (m *MiddlewareMonitor) ToJSON() (string, error) {
	metrics := m.GetAllMetrics()
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// 全局监控器实例
var globalMonitor = NewMiddlewareMonitor()

// GetGlobalMonitor 获取全局监控器
func GetGlobalMonitor() *MiddlewareMonitor {
	return globalMonitor
}

// MonitoringMiddleware 监控中间件，自动记录性能指标
func MonitoringMiddleware(middlewareName string) MiddlewareFunc {
	return func(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
		startTime := time.Now()
		
		resp, err := next(ctx, req)
		
		duration := time.Since(startTime)
		hasError := err != nil
		
		globalMonitor.RecordRequest(middlewareName, duration, hasError)
		
		// 记录详细日志
		if hasError {
			log.Printf("[MonitoringMiddleware] %s 执行失败 - 耗时: %v, 错误: %v", 
				middlewareName, duration, err)
		} else {
			log.Printf("[MonitoringMiddleware] %s 执行成功 - 耗时: %v", 
				middlewareName, duration)
		}
		
		return resp, err
	}
}

// HealthCheckMiddleware 健康检查中间件
func HealthCheckMiddleware(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
	// 检查系统健康状态
	metrics := globalMonitor.GetAllMetrics()
	
	var unhealthyMiddlewares []string
	for name, metric := range metrics {
		// 检查错误率是否过高（超过50%）
		if metric.RequestCount > 0 {
			errorRate := float64(metric.ErrorCount) / float64(metric.RequestCount)
			if errorRate > 0.5 {
				unhealthyMiddlewares = append(unhealthyMiddlewares, name)
			}
		}
		
		// 检查是否长时间没有请求（超过5分钟）
		if time.Since(metric.LastRequest) > 5*time.Minute && metric.RequestCount > 0 {
			log.Printf("[HealthCheckMiddleware] 中间件 %s 长时间无请求", name)
		}
	}
	
	if len(unhealthyMiddlewares) > 0 {
		log.Printf("[HealthCheckMiddleware] 检测到不健康的中间件: %v", unhealthyMiddlewares)
	}
	
	return next(ctx, req)
}

// AlertingMiddleware 告警中间件
func AlertingMiddleware(errorThreshold int64, responseTimeThreshold time.Duration) MiddlewareFunc {
	return func(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
		startTime := time.Now()
		resp, err := next(ctx, req)
		duration := time.Since(startTime)
		
		// 检查响应时间告警
		if duration > responseTimeThreshold {
			log.Printf("[AlertingMiddleware] 响应时间过长告警: %v > %v", duration, responseTimeThreshold)
		}
		
		// 检查错误计数告警
		if err != nil {
			metrics := globalMonitor.GetAllMetrics()
			for name, metric := range metrics {
				if metric.ErrorCount > errorThreshold {
					log.Printf("[AlertingMiddleware] 错误计数过高告警: %s 中间件错误数 %d > %d", 
						name, metric.ErrorCount, errorThreshold)
				}
			}
		}
		
		return resp, err
	}
}

// SamplingMiddleware 采样中间件，只处理部分请求
func SamplingMiddleware(sampleRate float64) MiddlewareFunc {
	if sampleRate < 0 || sampleRate > 1 {
		sampleRate = 1.0 // 默认处理所有请求
	}
	
	var counter int64
	
	return func(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
		currentCount := atomic.AddInt64(&counter, 1)
		
		// 简单的采样算法：每 1/sampleRate 个请求处理一个
		if sampleRate == 1.0 || float64(currentCount%int64(1/sampleRate)) == 0 {
			log.Printf("[SamplingMiddleware] 处理采样请求 #%d", currentCount)
			return next(ctx, req)
		}
		
		// 跳过的请求返回默认响应
		log.Printf("[SamplingMiddleware] 跳过请求 #%d (采样率: %.2f)", currentCount, sampleRate)
		return fmt.Sprintf("sampled_response_%d", currentCount), nil
	}
}

// LoadBalancingMiddleware 负载均衡中间件
func LoadBalancingMiddleware(handlers []Handler) MiddlewareFunc {
	if len(handlers) == 0 {
		panic("LoadBalancingMiddleware: 至少需要一个处理器")
	}
	
	var counter int64
	
	return func(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
		// 轮询选择处理器
		index := atomic.AddInt64(&counter, 1) % int64(len(handlers))
		selectedHandler := handlers[index]
		
		log.Printf("[LoadBalancingMiddleware] 选择处理器 #%d", index)
		
		return selectedHandler(ctx, req)
	}
}
