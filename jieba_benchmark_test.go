package gojieba

import (
	"os"
	"strconv"
	"strings"
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

func BenchmarkFree(b *testing.B) {
	rssBefore := getRSS()
	x := NewJieba()
	b.Run(
		"Cut",
		func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				x.CutAll("我是拖拉机学院手扶拖拉机专业的。不用多久，我就会升职加薪，当上CEO，走上人生巅峰。")
			}
		},
	)
	x.Free()
	rssAfter := getRSS()
	b.Logf("RSS before: %.2f MB, after: %.2f MB\n", rssBefore, rssAfter)
}

func BenchmarkFreeWithTrim(b *testing.B) {
	rssBefore := getRSS()
	x := NewJieba()
	b.Run(
		"Cut",
		func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				x.CutAll("我是拖拉机学院手扶拖拉机专业的。不用多久，我就会升职加薪，当上CEO，走上人生巅峰。")
			}
		},
	)
	x.FreeWithTrim()
	rssAfter := getRSS()
	b.Logf("RSS before: %.2f MB, after: %.2f MB\n", rssBefore, rssAfter)
}

// getRSS 读取 Linux/Unix 系统下的 RSS 内存占用 (单位: MB)
func getRSS() float64 {
	// 读取 /proc/self/statm 获取内存信息
	// 第二列是 RSS (以页为单位)
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0
	}
	rssPages, _ := strconv.ParseUint(fields[1], 10, 64)
	pageSize := int64(os.Getpagesize())

	// 转换为 MB
	return float64(rssPages*uint64(pageSize)) / (1024 * 1024)
}
