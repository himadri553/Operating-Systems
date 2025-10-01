/* Answer: No, the algorithm would no longer be correct. 
The guard acts as a spin lock protecting the shared internal state of the lock (flag and queue).
Without it, multiple threads could concurrently modify "flag" and the queue, causing race conditions.

In lock():
Without the guard, two threads might both observe m->flag == 0 and both set it to 1, 
thinking they have acquired the lock. Neither thread would be exclusive in the critical section.

In unlock():
Without the guard, a thread checking queue_empty(m->q) 
could race with another thread enqueuing itself.
This could result in the lock being dropped (m->flag = 0) even though another thread is waiting, 
violating the progress (a waiting thread might never be awakened).

Malicious Scheduler Execution:
1) Thread T1 calls lock(m). It sees m->flag == 0.
2) Before T1 sets m->flag = 1, the scheduler switches to T2.
3) T2 also calls lock(m), sees m->flag == 0, and sets it to 1.
4) Both T1 and T2 now think they own the lock and enter the critical section.

Result: Two threads are inside critical section at the same time (which is very bad...)
------------------------------------------------------------------------------------------------------

1 typedef struct __lock_t {
2 int flag;
3 int guard;
4 queue_t *q;
5 } lock_t;
6
7 void lock_init(lock_t *m) {
8 m->flag = 0;
9 m->guard = 0;
10 queue_init(m->q);
11 }
12
13 void lock(lock_t *m) {
14 while (TestAndSet(&m->guard, 1) == 1)
15 ; //acquire guard lock by spinning
16 if (m->flag == 0) {
17 m->flag = 1; // lock is acquired
18 m->guard = 0;
19 } else {
20 queue_add(m->q, gettid());
21 m->guard = 0;
22 park();
23 }
24 }
25
26 void unlock(lock_t *m) {
27 while (TestAndSet(&m->guard, 1) == 1)
28 ; //acquire guard lock by spinning
29 if (queue_empty(m->q))
30 m->flag = 0; // let go of lock; no one wants it
31 else
32 unpark(queue_remove(m->q)); // hold lock
33 // (for next thread!)
34 m->guard = 0;
35 } 
*/

#include <stdio.h>
#include <stdlib.h>
#include <pthread.h>
#include <unistd.h>
#include <sched.h>
#include <stdint.h>

typedef struct __lock_no_guard {
    volatile int flag;
} lock_no_guard_t;

void lock_init_no_guard(lock_no_guard_t *m) {
    m->flag = 0;
}

void unlock_no_guard(lock_no_guard_t *m) {
    m->flag = 0;
}

void lock_no_guard(lock_no_guard_t *m) {
    for (;;) {
        int f = m->flag;
        sched_yield();
        if (f == 0) {
            m->flag = 1;
            return;
        }
        while (m->flag) {
            sched_yield();
        }
    }
}

#define N_THREADS 2

pthread_barrier_t start_barrier;
lock_no_guard_t demo_lock;

void *thread_fn(void *arg) {
    int id = (int)(intptr_t)arg;
    pthread_barrier_wait(&start_barrier);
    lock_no_guard(&demo_lock);
    printf("Thread %d: ENTERED critical section (flag=%d)\n", id, demo_lock.flag);
    sleep(1);
    printf("Thread %d: LEAVING critical section\n", id);
    unlock_no_guard(&demo_lock);
    return NULL;
}

int main(void) {
    pthread_t th[N_THREADS];
    pthread_barrier_init(&start_barrier, NULL, N_THREADS);
    lock_init_no_guard(&demo_lock);
    for (int i = 0; i < N_THREADS; ++i) {
        if (pthread_create(&th[i], NULL, thread_fn, (void*)(intptr_t)i) != 0) {
            perror("pthread_create");
            exit(1);
        }
    }
    for (int i = 0; i < N_THREADS; ++i) {
        pthread_join(th[i], NULL);
    }
    pthread_barrier_destroy(&start_barrier);
    printf("Done\n");
    return 0;
}







