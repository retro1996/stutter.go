package gojieba

import (
	"testing"
)

// 准备一段较长的文本，模拟真实场景
const benchStr = "南京市长江大桥欢迎你，这是一个非常棒的开源分词项目，性能优化非常重要。"

func BenchmarkGoJieba(b *testing.B) {
	x := NewJieba()
	defer x.FreeWithTrim()
	b.Run("Cut", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// 防止编译器优化掉函数调用
			_ = x.Cut(benchStr, false)
		}
	})
	b.Run("CutForSearch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = x.CutForSearch(benchStr, false)
		}
	})
	b.Run("CutAll", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = x.CutAll(benchStr)
		}
	})
}
