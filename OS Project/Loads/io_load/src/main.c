#include <zephyr/kernel.h>

int io_heavy_start(void); // from io_heavy.c

int main(void)
{
    io_heavy_start();     // start your I/O load + metrics
    while (1) {
        k_sleep(K_SECONDS(1)); // keep main alive
    }
}
