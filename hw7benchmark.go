package main

import (
    "fmt"
    "time"
    "raid"
    "math/rand"
)

const Blocks = 25000

func runBenchmark(name string, r raid.RAID) {
    fmt.Println("=== Benchmark:", name, "===")

    data := make([]byte, raid.BlockSize)
    rand.Read(data)

    startW := time.Now()
    for i := 0; i < Blocks; i++ {
        r.Write(i, data)
    }
    writeTime := time.Since(startW)

    startR := time.Now()
    for i := 0; i < Blocks; i++ {
        r.Read(i)
    }
    readTime := time.Since(startR)

    fmt.Printf("Write Time: %v\n", writeTime)
    fmt.Printf("Read Time:  %v\n", readTime)
    fmt.Printf("Per-block write: %v\n", writeTime/Blocks)
    fmt.Printf("Per-block read:  %v\n\n", readTime/Blocks)
}
