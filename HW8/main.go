package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"
)

type LogEntry struct {
	Timestamp time.Time
	Level     string
	Context   string
	Message   string
}

func (e LogEntry) String() string {

	return fmt.Sprintf("[%s] [%s] [%s] %s\n",
		e.Timestamp.Format("2006-01-02 15:04:05"),
		e.Level,
		e.Context,
		e.Message,
	)
}

type Logger interface {
	Log(entry LogEntry) error
	Close() error
}

// Naive Logger 
// No synchronization. fsync after every write.
type NaiveLogger struct {
	f  *os.File
	bw *bufio.Writer
}

func NewNaiveLogger(path string) (*NaiveLogger, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &NaiveLogger{
		f:  f,
		bw: bufio.NewWriterSize(f, 64*1024),
	}, nil
}

func (l *NaiveLogger) Log(entry LogEntry) error {
	// UNSAFE: multiple goroutines will call this at once
	if _, err := l.bw.WriteString(entry.String()); err != nil {
		return err
	}
	if err := l.bw.Flush(); err != nil {
		return err
	}
	// fsync after every write
	return l.f.Sync()
}

func (l *NaiveLogger) Close() error {
	_ = l.bw.Flush()
	return l.f.Close()
}

// Mutex Logger 
// Mutex around file writes. Batching: fsync every 10 entries.
type MutexLogger struct {
	f        *os.File
	bw       *bufio.Writer
	mu       sync.Mutex
	batchN   int
	pending  int
}

func NewMutexLogger(path string, batchN int) (*MutexLogger, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	if batchN <= 0 {
		batchN = 1
	}
	return &MutexLogger{
		f:      f,
		bw:     bufio.NewWriterSize(f, 64*1024),
		batchN: batchN,
	}, nil
}

func (l *MutexLogger) Log(entry LogEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := l.bw.WriteString(entry.String()); err != nil {
		return err
	}
	// write can be buffered; flush so it reaches OS 
	if err := l.bw.Flush(); err != nil {
		return err
	}

	l.pending++
	if l.pending >= l.batchN {
		l.pending = 0
		return l.f.Sync() // fsync batched
	}
	return nil
}

func (l *MutexLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	_ = l.bw.Flush()
	_ = l.f.Sync() // final durability
	return l.f.Close()
}

// Channel Logger 
// Goroutines send entries to a channel
// Batching: fsync every 10 entries.
type ChannelLogger struct {
	f       *os.File
	bw      *bufio.Writer
	ch      chan LogEntry
	done    chan struct{}
	errMu   sync.Mutex
	lastErr error

	batchN  int
}

func NewChannelLogger(path string, batchN int, chanBuf int) (*ChannelLogger, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	if batchN <= 0 {
		batchN = 1
	}
	if chanBuf <= 0 {
		chanBuf = 100
	}

	l := &ChannelLogger{
		f:      f,
		bw:     bufio.NewWriterSize(f, 64*1024),
		ch:     make(chan LogEntry, chanBuf),
		done:   make(chan struct{}),
		batchN: batchN,
	}

	go l.writerLoop()
	return l, nil
}

func (l *ChannelLogger) setErr(err error) {
	l.errMu.Lock()
	defer l.errMu.Unlock()
	if l.lastErr == nil {
		l.lastErr = err
	}
}

func (l *ChannelLogger) getErr() error {
	l.errMu.Lock()
	defer l.errMu.Unlock()
	return l.lastErr
}

func (l *ChannelLogger) writerLoop() {
	defer close(l.done)

	pending := 0
	for entry := range l.ch {
		if _, err := l.bw.WriteString(entry.String()); err != nil {
			l.setErr(err)
			continue
		}
		if err := l.bw.Flush(); err != nil {
			l.setErr(err)
			continue
		}

		pending++
		if pending >= l.batchN {
			pending = 0
			if err := l.f.Sync(); err != nil {
				l.setErr(err)
			}
		}
	}

	_ = l.bw.Flush()
	_ = l.f.Sync()
	_ = l.f.Close()
}

func (l *ChannelLogger) Log(entry LogEntry) error {
	// If writer hit an error, stop accepting logs
	if err := l.getErr(); err != nil {
		return err
	}
	l.ch <- entry
	return nil
}

func (l *ChannelLogger) Close() error {
	close(l.ch)
	<-l.done
	return l.getErr()
}

// Benchmark Driver 

var levels = []string{"INFO", "WARN", "ERROR"}

func randEntry(gid, i int) LogEntry {
	level := levels[rand.Intn(len(levels))]
	ctx := fmt.Sprintf("req-%d-%d", gid, i)
	msg := fmt.Sprintf("Message number %d from goroutine %d", i, gid)
	return LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Context:   ctx,
		Message:   msg,
	}
}

func runBenchmark(name string, logger Logger, goroutines int, entriesPerG int) time.Duration {
	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		gid := g
		go func() {
			defer wg.Done()
			for i := 0; i < entriesPerG; i++ {
				_ = logger.Log(randEntry(gid, i))
			}
		}()
	}

	wg.Wait()
	_ = logger.Close()

	d := time.Since(start)
	fmt.Printf("%s: goroutines=%d entriesEach=%d total=%d time=%v\n",
		name, goroutines, entriesPerG, goroutines*entriesPerG, d)
	return d
}

func main() {
	rand.Seed(time.Now().UnixNano())

	goroutines := 8
	entriesPerG := 50
	batchN := 10

	// 1) Naive
	naive, err := NewNaiveLogger("naive.log")
	if err != nil {
		panic(err)
	}
	runBenchmark("NaiveLogger (fsync every write)", naive, goroutines, entriesPerG)

	// 2) Mutex
	mutexLogger, err := NewMutexLogger("mutex.log", batchN)
	if err != nil {
		panic(err)
	}
	runBenchmark("MutexLogger (fsync every 10)", mutexLogger, goroutines, entriesPerG)

	// 3) Channel
	channelLogger, err := NewChannelLogger("channel.log", batchN, 200)
	if err != nil {
		panic(err)
	}
	runBenchmark("ChannelLogger (fsync every 10)", channelLogger, goroutines, entriesPerG)

	fmt.Println("\nTip: run `go run -race main.go` and inspect naive.log for interleaving/corruption.")
}
