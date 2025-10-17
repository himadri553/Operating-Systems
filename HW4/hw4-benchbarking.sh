#!/usr/bin/env bash
set -euo pipefail

qtypes=(lock ms)
cases=(
  "-producers=1  -consumers=1  -work=20000 -gomaxprocs=2   -dur=5s   # W1"
  "-producers=8  -consumers=8  -work=0     -gomaxprocs=16  -dur=5s   # W2"
  "-producers=16 -consumers=2  -work=0     -gomaxprocs=18  -dur=5s   # W3"
  "-producers=2  -consumers=16 -work=0     -gomaxprocs=18  -dur=5s   # W4"
  "-producers=8  -consumers=8  -workP=0 -workC=0 -gomaxprocs=16 -burstP=100 -burstPauseUs=200 -dur=5s # W5"
)

for q in "${qtypes[@]}"; do
  for args in "${cases[@]}"; do
    echo "==== $q | $args ===="
    ./qbench -q "$q" $args
    echo
  done
done
