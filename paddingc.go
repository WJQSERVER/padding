package padding

import (
	"log"
	"net/http"

	"github.com/WJQSERVER-STUDIO/httpc"
)

// ToukaPadding 返回一个 httpc 的客户端中间件。
// 此中间件通过在每个出站 HTTP 请求中添加一个具有随机长度和内容的头部，
func ToukaPadding(opts PaddingOptions) httpc.MiddlewareFunc {
	// --- 验证和设置配置默认值 ---
	if opts.HeaderName == "" {
		opts.HeaderName = "T-Padding"
	}
	if opts.Profile == nil {
		opts.Profile = &ProfileDefault
	}
	// 验证 Profile 范围的逻辑，与服务端版本一致
	if opts.Profile.MaxLength > maxPaddingSize {
		log.Printf("httpc.ToukaPadding: Warning - Profile.MaxLength (%d) exceeds maxPaddingSize (%d). It will be capped.",
			opts.Profile.MaxLength, maxPaddingSize)
		opts.Profile.MaxLength = maxPaddingSize
	}
	if opts.Profile.MinLength < 0 {
		opts.Profile.MinLength = 0
	}
	if opts.Profile.MinLength > opts.Profile.MaxLength {
		log.Printf("httpc.ToukaPadding: Warning - Profile.MinLength (%d) is greater than MaxLength (%d). Adjusting to be equal.",
			opts.Profile.MinLength, opts.Profile.MaxLength)
		opts.Profile.MinLength = opts.Profile.MaxLength
	}

	// 返回中间件函数
	return func(next http.Handler) http.Handler {
		// 返回一个 http.HandlerFunc，它实现了 http.Handler 接口
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// --- 在发送请求前，修改请求 ---

			// 计算随机的 padding 长度
			paddingLen, err := randInt(opts.Profile.MinLength, opts.Profile.MaxLength)
			if err != nil {
				// 随机数生成失败是一个罕见的内部错误，记录日志但不中断请求。
				log.Printf("httpc.ToukaPadding: failed to generate random padding length: %v", err)
			} else if paddingLen > 0 {
				// 从预计算的池中获取随机内容
				paddingData := getPaddingSlice(paddingLen)
				// 设置 padding 头部到出站请求 `r`
				// r.Header 是一个引用，可以直接修改
				if r.Header == nil {
					r.Header = make(http.Header)
				}
				r.Header.Set(opts.HeaderName, string(paddingData))
			}

			// 调用链中的下一个处理器。
			// 在 httpc 中，最终的 `next` 是一个将请求发送出去的处理器。
			next.ServeHTTP(w, r)
		})
	}
}
