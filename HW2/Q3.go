package main

import (
	"flag"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

/* =========================
   Lock interface
   ========================= */

type Lock interface {
	Lock()
	Unlock()
}

/* =========================
   CAS Spin Lock (unfair)
   ========================= */

type CASLock struct {
	state int32 // 0 = unlocked, 1 = locked
}

func (l *CASLock) Lock() {
	for {
		if atomic.CompareAndSwapInt32(&l.state, 0, 1) {
			return
		}
		runtime.Gosched()
	}
}

func (l *CASLock) Unlock() {
	atomic.StoreInt32(&l.state, 0)
}

/* =========================
   Ticket Lock (FIFO / fair)
   ========================= */

type TicketLock struct {
	next       uint64 // next ticket to hand out
	nowServing uint64 // ticket currently allowed to enter
}

func (l *TicketLock) Lock() {
	my := atomic.AddUint64(&l.next, 1) - 1
	for atomic.LoadUint64(&l.nowServing) != my {
		runtime.Gosched()
	}
}

func (l *TicketLock) Unlock() {
	atomic.AddUint64(&l.nowServing, 1)
}

/* =========================
   Critical-section "work"
   ========================= */

func busyUS(us int) {
	if us <= 0 {
		return
	}
	end := time.Now().Add(time.Duration(us) * time.Microsecond)
	for time.Now().Before(end) {
	}
}

/* =========================
   Stats helpers
   ========================= */

type Summary struct {
	N      int
	MeanNS float64
	P50NS  float64
	P95NS  float64
	MaxNS  float64
}

func summarize(ds []time.Duration) Summary {
	n := len(ds)
	if n == 0 {
		return Summary{}
	}

	nums := make([]float64, n)
	var sum, max float64
	for i, d := range ds {
		v := float64(d.Nanoseconds())
		nums[i] = v
		sum += v
		if v > max {
			max = v
		}
	}
	sort.Float64s(nums)

	pct := func(q float64) float64 {
		if n == 1 {
			return nums[0]
		}
		pos := q * float64(n-1)
		lo := int(math.Floor(pos))
		hi := int(math.Ceil(pos))
		if lo == hi {
			return nums[lo]
		}
		f := pos - float64(lo)
		return nums[lo]*(1-f) + nums[hi]*f
	}

	return Summary{
		N:      n,
		MeanNS: sum / float64(n),
		P50NS:  pct(0.50),
		P95NS:  pct(0.95),
		MaxNS:  max,
	}
}

/* =========================
   Benchmark runner
   ========================= */

func run(lock Lock, goroutines, iters, csUS int, progressEvery int) Summary {
	var wg sync.WaitGroup
	wg.Add(goroutines)

	ch := make(chan []time.Duration, goroutines)
	startGate := make(chan struct{})

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			local := make([]time.Duration, 0, iters)
			<-startGate
			for i := 0; i < iters; i++ {
				if progressEvery > 0 && i%progressEvery == 0 {
					fmt.Print(".")
				}
				t0 := time.Now()
				lock.Lock()
				wait := time.Since(t0)
				busyUS(csUS)
				lock.Unlock()
				local = append(local, wait)
			}
			ch <- local
		}()
	}

	close(startGate)
	wg.Wait()
	close(ch)

	all := make([]time.Duration, 0, goroutines*iters)
	for s := range ch {
		all = append(all, s...)
	}
	if progressEvery > 0 {
		fmt.Println() // newline after dots
	}
	return summarize(all)
}

/* =========================
   main + flags
   ========================= */

func main() {
	var (
		lockType      = flag.String("type", "ticket", "lock type: ticket | cas")
		goroutines    = flag.Int("goroutines", 8, "number of goroutines (threads) contending")
		iters         = flag.Int("iters", 100000, "lock acquisitions per goroutine")
		csUS          = flag.Int("csus", 2, "critical-section busy time in microseconds")
		gmp           = flag.Int("gomaxprocs", runtime.NumCPU(), "GOMAXPROCS (logical CPUs)")
		progressEvery = flag.Int("progressEvery", 10000, "print a dot every N iterations per goroutine (0 = off)")
	)
	flag.Parse()

	runtime.GOMAXPROCS(*gmp)

	var l Lock
	switch *lockType {
	case "ticket":
		l = &TicketLock{}
	case "cas":
		l = &CASLock{}
	default:
		panic("unknown -type (use 'ticket' or 'cas')")
	}

	_ = run(l, 2, 2000, 1, 0) // quick warmup, no dots

	s := run(l, *goroutines, *iters, *csUS, *progressEvery)

	fmt.Printf("Lock: %s | G=%d | iters=%d | cs=%dus | GOMAXPROCS=%d\n",
		*lockType, *goroutines, *iters, *csUS, *gmp)
	fmt.Printf("Wait stats (ns): mean=%.0f  p50=%.0f  p95=%.0f  max=%.0f  (N=%d)\n",
		s.MeanNS, s.P50NS, s.P95NS, s.MaxNS, s.N)
}
