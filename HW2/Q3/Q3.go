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

/***************
 * Lock API
 ***************/
type Lock interface {
	Lock()
	Unlock()
}

/***************
 * CAS Spin Lock (unfair)
 * Correctness notes:
 * - Use CAS 0->1 to acquire.
 * - Use Store(0) to release (do NOT CAS in Unlock).
 ***************/
type CASLock struct{ state int32 }

func (l *CASLock) Lock() {
	for {
		if atomic.CompareAndSwapInt32(&l.state, 0, 1) {
			return
		}
		runtime.Gosched() // be polite; avoids starving others under Go's scheduler
	}
}
func (l *CASLock) Unlock() { atomic.StoreInt32(&l.state, 0) }

/***************
 * Ticket Lock (FIFO / Fig. 28.7 style)
 * Correctness notes:
 * - "Fetch-and-increment" for ticket handout.
 * - Spin on nowServing == myTicket.
 * - Unlock increments nowServing by 1.
 * - Both counters are unsigned and monotonically increase (wrap is OK with uint64).
 ***************/
type TicketLock struct {
	next       uint64 // next ticket to hand out
	nowServing uint64 // ticket currently served
}

func (l *TicketLock) Lock() {
	my := atomic.AddUint64(&l.next, 1) - 1 // FAA: returns new value; subtract 1 to get my ticket
	for atomic.LoadUint64(&l.nowServing) != my {
		runtime.Gosched()
	}
}
func (l *TicketLock) Unlock() { atomic.AddUint64(&l.nowServing, 1) }

/***************
 * Critical-section “work” to tune contention
 * Busy loop ~us microseconds (don’t Sleep, which deflates contention)
 ***************/
func busyUS(us int) {
	if us <= 0 {
		return
	}
	end := time.Now().Add(time.Duration(us) * time.Microsecond)
	for time.Now().Before(end) {
		// burn a little time; no allocations
	}
}

/***************
 * Stats
 ***************/
type Summary struct {
	N               int
	MeanNS, P50NS   float64
	P95NS, MaxNS    float64
}

func summarize(ds []time.Duration) Summary {
	n := len(ds)
	if n == 0 {
		return Summary{}
	}
	arr := make([]float64, n)
	var sum, mx float64
	for i, d := range ds {
		v := float64(d.Nanoseconds())
		arr[i] = v
		sum += v
		if v > mx {
			mx = v
		}
	}
	sort.Float64s(arr)

	pct := func(q float64) float64 {
		if n == 1 {
			return arr[0]
		}
		pos := q * float64(n-1)
		lo := int(math.Floor(pos))
		hi := int(math.Ceil(pos))
		if lo == hi {
			return arr[lo]
		}
		f := pos - float64(lo)
		return arr[lo]*(1-f) + arr[hi]*f
	}

	return Summary{
		N:      n,
		MeanNS: sum / float64(n),
		P50NS:  pct(0.50),
		P95NS:  pct(0.95),
		MaxNS:  mx,
	}
}

/***************
 * Benchmark runner
 * Measures wait time for each Lock() call.
 ***************/
func run(lock Lock, goroutines, iters, csUS int) Summary {
	var wg sync.WaitGroup
	wg.Add(goroutines)

	ch := make(chan []time.Duration, goroutines)
	startGate := make(chan struct{}) // release all at once to create contention

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			local := make([]time.Duration, 0, iters)
			<-startGate
			for i := 0; i < iters; i++ {
				t0 := time.Now()
				lock.Lock()
				wait := time.Since(t0)
				busyUS(csUS) // work inside the critical section
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
	return summarize(all)
}

/***************
 * Main + flags
 ***************/
func main() {
	var (
		lockType   = flag.String("type", "ticket", "lock type: ticket | cas")
		goroutines = flag.Int("goroutines", 8, "number of contending goroutines")
		iters      = flag.Int("iters", 100000, "acquisitions per goroutine")
		csUS       = flag.Int("csus", 2, "critical-section busy time (microseconds)")
		gmp        = flag.Int("gomaxprocs", runtime.NumCPU(), "GOMAXPROCS")
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

	// quick warmup (stabilizes scheduling; optional)
	_ = run(l, 2, 2000, 1)

	// real run
	s := run(l, *goroutines, *iters, *csUS)
	fmt.Printf("Lock=%s  G=%d  iters=%d  cs=%dus  GOMAXPROCS=%d\n",
		*lockType, *goroutines, *iters, *csUS, *gmp)
	fmt.Printf("Wait (ns): mean=%.0f  p50=%.0f  p95=%.0f  max=%.0f  (N=%d)\n",
		s.MeanNS, s.P50NS, s.P95NS, s.MaxNS, s.N)
}
