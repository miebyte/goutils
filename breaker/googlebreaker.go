package breaker

import (
	"time"

	"github.com/miebyte/goutils/utils"
	"github.com/miebyte/goutils/utils/rollingwindow"
)

const (
	// 250ms for bucket duration
	window            = time.Second * 10
	buckets           = 40
	forcePassDuration = time.Second
	k                 = 1.5
	minK              = 1.1
	protection        = 5
)

// googleBreaker is a netflixBreaker pattern from google.
// see Client-Side Throttling section in https://landing.google.com/sre/sre-book/chapters/handling-overload/
type (
	googleBreaker struct {
		k        float64
		stat     *rollingwindow.RollingWindow[int64, *bucket]
		proba    *Proba
		lastPass *utils.AtomicDuration
	}

	windowResult struct {
		accepts        int64
		total          int64
		failingBuckets int64
		workingBuckets int64
	}
)

func newGoogleBreaker() *googleBreaker {
	bucketDuration := time.Duration(int64(window) / int64(buckets))
	st := rollingwindow.NewRollingWindow(func() *bucket {
		return new(bucket)
	}, buckets, bucketDuration)
	return &googleBreaker{
		stat:     st,
		k:        k,
		proba:    NewProba(),
		lastPass: utils.NewAtomicDuration(),
	}
}

// accept 基于滑动窗口统计与 SRE 公式计算当前丢弃比例，决定是否放行请求。
func (b *googleBreaker) accept() error {
	var w float64
	history := b.history()
	// 动态权重：失败桶越多，权重从 k 线性收敛至 minK
	w = b.k - (b.k-minK)*float64(history.failingBuckets)/buckets
	// 加权接受量
	weightedAccepts := AtLeast(w, minK) * float64(history.accepts)
	// https://landing.google.com/sre/sre-book/chapters/handling-overload/#eq2101
	// for better performance, no need to care about the negative ratio
	// 丢弃比例（含保护项），<=0 则无需限流
	dropRatio := (float64(history.total-protection) - weightedAccepts) / float64(history.total+1)
	if dropRatio <= 0 {
		return nil
	}

	// 强制通过：超过探测间隔至少放行一次
	lastPass := b.lastPass.Load()
	if lastPass > 0 && utils.Since(lastPass) > forcePassDuration {
		b.lastPass.Set(utils.Now())
		return nil
	}

	// 近期工作调节：连续仅成功越多，丢弃率越低
	dropRatio *= float64(buckets-history.workingBuckets) / buckets

	// 概率丢弃
	if b.proba.TrueOnProba(dropRatio) {
		return ErrServiceUnavailable
	}

	// 放行时更新最近放行时间
	b.lastPass.Set(utils.Now())

	return nil
}

func (b *googleBreaker) allow() (internalPromise, error) {
	if err := b.accept(); err != nil {
		b.markDrop()
		return nil, err
	}

	return googlePromise{
		b: b,
	}, nil
}

func (b *googleBreaker) doReq(req func() error, fallback Fallback, acceptable Acceptable) error {
	if err := b.accept(); err != nil {
		b.markDrop()
		if fallback != nil {
			return fallback(err)
		}

		return err
	}

	var succ bool
	defer func() {
		// if req() panic, success is false, mark as failure
		if succ {
			b.markSuccess()
		} else {
			b.markFailure()
		}
	}()

	err := req()
	if acceptable(err) {
		succ = true
	}

	return err
}

func (b *googleBreaker) markDrop() {
	b.stat.Add(drop)
}

func (b *googleBreaker) markFailure() {
	b.stat.Add(fail)
}

func (b *googleBreaker) markSuccess() {
	b.stat.Add(success)
}

func (b *googleBreaker) history() windowResult {
	var result windowResult

	b.stat.Reduce(func(b *bucket) {
		result.accepts += b.Success
		result.total += b.Sum
		if b.Failure > 0 {
			result.workingBuckets = 0
		} else if b.Success > 0 {
			result.workingBuckets++
		}
		if b.Success > 0 {
			result.failingBuckets = 0
		} else if b.Failure > 0 {
			result.failingBuckets++
		}
	})

	return result
}

type googlePromise struct {
	b *googleBreaker
}

func (p googlePromise) Accept() {
	p.b.markSuccess()
}

func (p googlePromise) Reject() {
	p.b.markFailure()
}
