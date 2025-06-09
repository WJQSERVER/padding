// Copyright 2025 Infinite-Iroha. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE file.

// padding.go 实现了 toukaPadding 中间件，用于增加流量随机性以对抗审查
package padding

import (
	"crypto/rand"
	"errors"
	"math/big"
)

// --- 预生成的随机数据池 (高性能 Padding 的基础) ---
const (
	// maxPaddingSize 定义了预生成随机数据池的大小，也是单个 padding 头的最大可能长度
	// 4KB 是一个合理的大小，可以覆盖大多数头部长度需求
	maxPaddingSize = 4096
	// paddingCharset 是用于生成随机 padding 内容的字符集
	paddingCharset = "X"
)

var (
	// precomputedPaddingData 在程序启动时生成，用于高效获取随机 padding 内容
	// 这是一个包级别的只读变量，在初始化后不会被修改，因此并发读取是安全的
	precomputedPaddingData []byte
)

func init() {
	precomputedPaddingData = make([]byte, maxPaddingSize)
	charsetLen := big.NewInt(int64(len(paddingCharset)))
	for i := 0; i < maxPaddingSize; i++ {
		randIndex, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			// 如果在初始化时无法生成随机数据，这是一个严重错误，应立即 panic
			panic("toukaPadding: failed to initialize precomputed padding data: " + err.Error())
		}
		precomputedPaddingData[i] = paddingCharset[randIndex.Int64()]
	}
}

// PaddingProfile 定义了一种特定的 padding 长度分布策略
type PaddingProfile struct {
	MinLength int // Padding 的最小长度（字节）
	MaxLength int // Padding 的最大长度（字节）
}

// 内置的 Padding 策略，模仿不同类型网站的响应大小
// 用户可以根据自己的需求定义更多策略
var (
	// ProfileDefault 是默认的 padding 策略，提供了一个通用的、中等大小的随机范围
	// 适用于大多数 Web 和 API 响应，能在不过度消耗带宽的情况下有效增加流量随机性
	ProfileDefault = PaddingProfile{MinLength: 96, MaxLength: 1024}

	// ProfileShort 模仿非常小的 API 响应或状态检查，padding 范围较小
	// 适用于那些本身响应体就很小，不希望 padding 喧宾夺主的场景
	ProfileShort = PaddingProfile{MinLength: 32, MaxLength: 256}

	// ProfileLong 模仿内容丰富的页面或包含较大元数据的响应，padding 较长
	// 用于需要更强混淆效果的场景
	ProfileLong = PaddingProfile{MinLength: 1024, MaxLength: maxPaddingSize}
)

// PaddingOptions 用于配置 toukaPadding 中间件
type PaddingOptions struct {
	// HeaderName 是要添加 padding 的 HTTP 响应头的名称
	// 默认为 "T-Padding"
	HeaderName string
	// Profile 是要使用的 padding 长度分布策略
	// 可以使用内置的 ProfileDefault, ProfileShort, ProfileLong 等，或自定义
	// 如果为 nil，将使用 ProfileDefault 作为默认值
	Profile *PaddingProfile
}

// --- 内部辅助函数 ---

// randInt 在 [min, max] 范围内生成一个加密安全的随机整数
func randInt(min, max int) (int, error) {
	if min > max {
		return 0, errors.New("min cannot be greater than max")
	}
	if min == max {
		return min, nil
	}
	n := big.NewInt(int64(max - min + 1))
	val, err := rand.Int(rand.Reader, n)
	if err != nil {
		return 0, err
	}
	return int(val.Int64()) + min, nil
}

// getPaddingSlice 从预计算的随机数据池中获取一个指定长度的切片
func getPaddingSlice(length int) []byte {
	if length <= 0 {
		return nil
	}
	if length > maxPaddingSize {
		length = maxPaddingSize
	}
	maxStart := maxPaddingSize - length
	start, err := randInt(0, maxStart)
	if err != nil {
		start = 0 // 保证功能可用性
	}
	return precomputedPaddingData[start : start+length]
}
