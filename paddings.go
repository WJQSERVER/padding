package padding

import (
	"log"
	"net/http"
	"sync"

	"github.com/infinite-iroha/touka"
)

// paddingResponseWriter 是一个内部的 ResponseWriter 包装器，用于实现 padding
// 它通过嵌入 touka.ResponseWriter 自动代理了所有未覆盖的方法
type paddingResponseWriter struct {
	touka.ResponseWriter
	opts        *PaddingOptions
	wroteHeader bool
	mu          sync.Mutex // 保护 wroteHeader 标志的并发访问
}

// WriteHeader 在写入 HTTP 头部之前，添加随机长度的 padding 头部
// 这是添加 padding 的核心逻辑所在
func (prw *paddingResponseWriter) WriteHeader(statusCode int) {
	prw.mu.Lock()
	if prw.wroteHeader {
		prw.mu.Unlock()
		return
	}
	prw.wroteHeader = true
	prw.mu.Unlock()

	paddingLen, err := randInt(prw.opts.Profile.MinLength, prw.opts.Profile.MaxLength)
	if err != nil {
		// 随机数生成失败是一个罕见的内部错误，记录日志但不中断请求
		log.Printf("toukaPadding: failed to generate random padding length: %v", err)
	} else if paddingLen > 0 {
		paddingData := getPaddingSlice(paddingLen)
		prw.Header().Set(prw.opts.HeaderName, string(paddingData))
	}

	prw.ResponseWriter.WriteHeader(statusCode)
}

// Write 确保在第一次写入数据前头部（包括 padding）已被发送
// 如果 WriteHeader 尚未被调用，它会隐式地以 200 OK 状态调用它
func (prw *paddingResponseWriter) Write(data []byte) (int, error) {
	// 使用双重检查锁定模式来减少锁的竞争开销
	if !prw.wroteHeader {
		prw.mu.Lock()
		// 再次检查，防止在获取锁期间其他 goroutine 已写入头部
		if !prw.wroteHeader {
			prw.mu.Unlock()
			prw.WriteHeader(http.StatusOK)
		} else {
			prw.mu.Unlock()
		}
	}
	return prw.ResponseWriter.Write(data)
}

// ToukaPaddingS 返回一个 HTTP Padding 中间件
// 此中间件通过在 HTTP 响应头中添加一个具有随机长度和内容的头部（默认为 "T-Padding"），
// 来改变每个响应的加密后总长度这旨在干扰基于流量大小的审查和指纹识别系统
func ToukaPaddingS(opts PaddingOptions) touka.HandlerFunc {
	// --- 验证和设置配置默认值 ---
	if opts.HeaderName == "" {
		opts.HeaderName = "T-Padding"
	}
	if opts.Profile == nil {
		opts.Profile = &ProfileDefault
	}
	if opts.Profile.MaxLength > maxPaddingSize {
		log.Printf("toukaPadding: Warning - Profile.MaxLength (%d) exceeds maxPaddingSize (%d). It will be capped.",
			opts.Profile.MaxLength, maxPaddingSize)
		opts.Profile.MaxLength = maxPaddingSize
	}
	if opts.Profile.MinLength < 0 {
		opts.Profile.MinLength = 0
	}
	if opts.Profile.MinLength > opts.Profile.MaxLength {
		log.Printf("toukaPadding: Warning - Profile.MinLength (%d) is greater than MaxLength (%d). Adjusting to be equal.",
			opts.Profile.MinLength, opts.Profile.MaxLength)
		opts.Profile.MinLength = opts.Profile.MaxLength
	}

	return func(c *touka.Context) {
		originalWriter := c.Writer
		prw := &paddingResponseWriter{
			ResponseWriter: originalWriter,
			opts:           &opts,
		}
		c.Writer = prw

		// 不需要 defer 恢复 c.Writer，因为 c.Writer 是请求作用域的
		// Touka 框架的 Context.reset 会在下一个请求中处理 ResponseWriter 的重置或替换
		c.Next()
	}
}

// 确保 paddingResponseWriter 实现了 Touka 的 ResponseWriter 接口
// 这是一个编译时检查，通过嵌入接口自动满足
var _ touka.ResponseWriter = &paddingResponseWriter{}
