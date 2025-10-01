/*
	EECE 4811 - Operating Systems
	HW2 - Question 2
	Himadri Saha, Ashwin Srinivasan, Yaritza Sanchez

    Replacing line 12 of Figure 28.6 with "lock->flag = lock->flag - 1;" violates MUTUAL EXCLUSION
    2 Threads can be running at the same time in the critical section



*/

The algorithm relies on the assumption that flags can only take values 0 or 1.
But if we decrement instead of assigning, a malicious scheduler can interleave instructions and cause incorrect states:
Execution Scenario:

- Thread A holds the lock (flag = 1).

- Thread A executes lock->flag = lock->flag - 1; It first reads flag = 1.

- Before it writes back, Thread B tries to acquire the lock: It spins until it sees flag = 0. 
Because of timing, it might read a stale flag or overlap with A’s modification.

- Thread A writes back flag = 0

-  Property Violation: If another unlock happens incorrectly (bug or double-unlock), flag could become -1.
Now the flag (that only accepts 0 or 1) is broken.




1 void lock(lock_t *lock) {
2 while (1) {
3 while (LoadLinked(&lock->flag) == 1)
4 ; // spin until it’s zero
5 if (StoreConditional(&lock->flag, 1) == 1)
6 return; // if set-to-1 was success: done
7 // otherwise: try again
8 }
9 }
10
11 void unlock(lock_t *lock) {
12 lock->flag = 0;
13 }






/*
idk wut on earth dis is but ima leave it here incase im wrong
#include <stdio.h>
#include <pthread.h>
#include <unistd.h>

typedef struct {
    int flag; // 0 = unlocked, 1 = locked (but could become -1 with broken unlock)
} lock_t;

lock_t mylock = { .flag = 0 }; // global lock

// Broken unlock function (decrements instead of setting to 0)
void broken_unlock(lock_t *lock) {
    printf("[BROKEN UNLOCK] flag before: %d\n", lock->flag);
    lock->flag = lock->flag - 1;
    printf("[BROKEN UNLOCK] flag after: %d\n", lock->flag);
}

// Simulated Load-Linked and Store-Conditional
int LoadLinked(int *addr) {
    return *addr;
}

int StoreConditional(int *addr, int val) {
    // Simulate always successful SC (for simplicity)
    *addr = val;
    return 1;
}

// Lock using LL/SC
void lock(lock_t *lock) {
    while (1) {
        while (LoadLinked(&lock->flag) == 1)
            ; // spin until it's free

        if (StoreConditional(&lock->flag, 1) == 1)
            return; // lock acquired
    }
}

// Simulated critical section
void* thread_func_A(void *arg) {
    printf("[Thread A] Trying to acquire lock...\n");
    lock(&mylock);
    printf("[Thread A] Acquired lock.\n");

    // Simulate buggy double-unlock (1st valid, 2nd invalid)
    sleep(1);
    broken_unlock(&mylock); // 1st unlock (flag: 1 -> 0)
    sleep(1);
    broken_unlock(&mylock); // 2nd unlock (flag: 0 -> -1) 

    return NULL;
}

void* thread_func_B(void *arg) {
    sleep(3); // Let A "unlock" first
    printf("[Thread B] Trying to acquire lock...\n");
    lock(&mylock); // Should not work if lock was working properly
    printf("[Thread B] Acquired lock (but should NOT have!) \n");
    return NULL;
}

int main() {
    pthread_t t1, t2;

    pthread_create(&t1, NULL, thread_func_A, NULL);
    pthread_create(&t2, NULL, thread_func_B, NULL);

    pthread_join(t1, NULL);
    pthread_join(t2, NULL);

    printf("[Main] Final lock flag: %d\n", mylock.flag);
    return 0;
}
*/
