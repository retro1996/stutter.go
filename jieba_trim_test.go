package gojieba

import (
	"runtime"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"
)

var longText = "南京市长江大桥欢迎你，" +
	"这是一个非常长的句子，用于模拟真实场景下的分词负载。" +
	"我们在评估 malloc_trim 带来的性能损耗是否在可接受范围内。"

func BenchmarkTrimCost_Amortized(b *testing.B) {
	// 对真实使用场景的模拟，不论任务量大小，GC的成本是可接受的。
	// 对于执行FreeWithTrim的延迟问题：
	// 1. Trim的延迟本身非常容易被稀释，相比大量的Cut的延迟，Trim的延迟非常低。
	// 2. 如果对延迟非常敏感的环境，可以考虑异步调用Trim，来避免直接阻断关键路径.
	// 因此可以选择按需开启Trim，对于内存比较宝贵的场景，可以NewJieba().WithTrim()来自动为回收函数注册Trim

	// 场景 A: 短任务
	b.Run("LightLoad_10ops", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			x := NewJieba()
			b.StartTimer()
			for j := 0; j < 10; j++ {
				x.Cut(longText, false)
			}
			// 对比点：这里开启 Trim
			x.FreeWithTrim()
		}
	})

	// 场景 A: 短任务对照组
	b.Run("LightLoad_10ops_NoTrim", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			x := NewJieba()
			b.StartTimer()
			for j := 0; j < 10; j++ {
				x.Cut(longText, false)
			}
			x.Free()
		}
	})

	// 场景 B: 重任务
	b.Run("HeavyLoad_1000ops", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			x := NewJieba()
			b.StartTimer()
			for j := 0; j < 1000; j++ {
				x.Cut(longText, false)
			}
			// 对比点：这里开启 Trim
			x.FreeWithTrim()
		}
	})

	// 场景 B: 重任务对照组
	b.Run("HeavyLoad_1000ops_NoTrim", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			x := NewJieba()
			b.StartTimer()
			for j := 0; j < 1000; j++ {
				x.Cut(longText, false)
			}
			x.Free()
		}
	})
}

func TestTrimLatencyImpact(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping latency test in short mode")
	}

	service := NewJieba()
	defer service.Free()

	var latencies []time.Duration
	var mu sync.Mutex

	totalRequests := 50000
	concurrency := 10
	trimInterval := 100 * time.Millisecond // 每 100ms 触发一次 Trim (极其频繁，高压测试)

	var wg sync.WaitGroup
	wg.Add(concurrency)

	// 1. 干扰协程：定期触发 malloc_trim
	stopTrim := make(chan struct{})
	go func() {
		ticker := time.NewTicker(trimInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				Trim()
			case <-stopTrim:
				return
			}
		}
	}()

	// 2. 业务协程：并发处理请求
	start := time.Now()
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < totalRequests/concurrency; j++ {
				t0 := time.Now()
				service.CutAll(longText)
				cost := time.Since(t0)

				mu.Lock()
				latencies = append(latencies, cost)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	close(stopTrim) // 停止干扰

	totalTime := time.Since(start)

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p50 := latencies[len(latencies)*50/100]
	p99 := latencies[len(latencies)*99/100]
	max := latencies[len(latencies)-1]

	t.Logf("Total Requests: %d", len(latencies))
	t.Logf("Total Time: %v", totalTime)
	t.Logf("QPS: %.2f", float64(len(latencies))/totalTime.Seconds())
	t.Logf("P50 Latency: %v", p50)
	t.Logf("P99 Latency: %v", p99)
	t.Logf("Max Latency: %v", max)
}

// TestTrimLatency 测试真实负载下的Trim耗时
func TestTrimLatency(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}

	// 1. 准备环境：制造大量 C 侧内存垃圾
	t.Log("Preparing environment (allocating memory)...")
	x := NewJieba()
	// 模拟大量分词请求，产生大量 vector 扩容缩容留下的碎片
	for i := 0; i < 100000; i++ {
		x.CutAll("这是一个用来制造内存碎片的非常长的句子，我们需要让分配器忙碌起来" + strconv.Itoa(i)) // 使用 CutAll 或长句子，制造更多内存活动
	}

	// 2. 释放对象
	x.Free()

	// 3. 测量 Trim 的真实耗时
	start := time.Now()
	Trim() // 触发 malloc_trim
	duration := time.Since(start)

	t.Logf("------------------------------------------------")
	t.Logf("Time cost of malloc_trim: %v", duration)
	t.Logf("------------------------------------------------")

	if duration > 100*time.Millisecond {
		t.Error("Trim is taking too long! Consider running it asynchronously.")
	}
}

// TestRSSRecovery 验证 malloc_trim 对RSS回收的效果
func TestRSSRecovery(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("RSS check and malloc_trim are Linux only")
	}

	runtime.GC()
	initialRSS := getRSS()
	t.Logf("Step 1: Initial RSS: %.2f MB", initialRSS)

	t.Log("Step 2: Allocating massive C++ objects...")
	var jiebas []*Jieba
	for i := 0; i < 10; i++ {
		// 每次 NewJieba 都会加载大字典，占用大量 C 内存
		j := NewJieba()
		j.Cut("南京市长江大桥", false) // 做一些操作产生中间碎片
		jiebas = append(jiebas, j)
	}

	peakRSS := getRSS()
	t.Logf("Step 3: Peak RSS (After Alloc): %.2f MB", peakRSS)

	t.Log("Step 4: Freeing all instances (without trim)...")
	for _, j := range jiebas {
		j.Free()
	}
	jiebas = nil // 丢弃引用
	runtime.GC() // 确保 Go 层不持有引用

	afterFreeRSS := getRSS()
	t.Logf("Step 5: RSS after Free(): %.2f MB", afterFreeRSS)
	t.Logf("   -> Released to OS: %.2f MB", peakRSS-afterFreeRSS)

	t.Log("Step 6: Calling malloc_trim(0)...")
	start := time.Now()
	Trim()
	cost := time.Since(start)

	afterTrimRSS := getRSS()
	t.Logf("Step 7: RSS after malloc_trim: %.2f MB", afterTrimRSS)
	t.Logf("   -> Further Released: %.2f MB", afterFreeRSS-afterTrimRSS)
	t.Logf("   -> Trim Latency: %v", cost)

	if afterFreeRSS-afterTrimRSS < 1.0 {
		t.Log("⚠️ Note: Glibc might have already released memory automatically, or fragmentation wasn't severe enough.")
	}
}
