# RTOS Scheduling Visualizer
#### Team Members: Himadri Saha, Ashwin Srinivasan, Yaritza Sanchez
#### EECE 4811 - Operating Systems

# GETTING STARTED
*to be updated*

# Project Description 
Using a Raspberry Pi Pico running Zephyr 4.2, this project will show how a RTOS (Real time OS) behaves under various types of loads. We will also analyze metrics to illistrate its behavior and explain what is going on. Everything will be controlled by a live USB serial shell.

## Loads the OS will run (and slef report metrics using shared metrics helper functions)
Computation Heavy Load:
    - Thread that multiplies 64x64 matrices in a loop periodically (every 50ms)
    - CPU bound load and deterministic
    What Metrics means:
        - Execution Time per iteration: How long does the matrix multiplication take when the CPU is dedicated to it? 
        - Start Time jitter: How far off is the actual start time from the scheduled start?

I/O Heavy Load:
    - Thread that sends formatted messages over USB (UART) at intervals. 
    - UART is slower than CPU, scheduler is stressed when handling blocking I/O and interupt drive events
    Metircs:
        - USB Interrupt-to-Handler Latency: How quickly does the system respond to the UART TX complete interrupt? (shows how well Zephyr prioritizes I/O interrupts against heavy computation)
        - 

Periodic Real-Time Load:
    - Thread taht toggles GPIO pin (on-board LED) at 10ms
    - Simulates real-time control task 
    - Stresses RTOS cheduler’s ability to meet deadlines
    Metrics:
        - Deadline misses: deadline = next_release + PERIOD_MS
        

Noise Load:
Combination Scenarios: 

## Metrics 
- start-time jitter
- deadline misses
- execution time
- context switches
- CPU utilization per thread
- USB interrupt-to-handler latency

## Repo outline

### Recent Updates:

#### Credits
- https://repository.rit.edu/cgi/viewcontent.cgi?article=7896&context=theses


#### Random whiteboard
comp heavy exsample:
void compute_heavy_task(void *arg1, void *arg2, void *arg3) {
    static int A[64][64], B[64][64], C[64][64];

    // initialize A and B with some values
    for (int i = 0; i < 64; i++) {
        for (int j = 0; j < 64; j++) {
            A[i][j] = (i + j) % 10;
            B[i][j] = (i * j) % 10;
        }
    }

    while (1) {
        // Do full multiplication
        for (int i = 0; i < 64; i++) {
            for (int j = 0; j < 64; j++) {
                int sum = 0;
                for (int k = 0; k < 64; k++) {
                    sum += A[i][k] * B[k][j];
                }
                C[i][j] = sum;
            }
        }

        k_msleep(50); // optional pause to simulate periodic workload
    }
}

I/O heavy ex:
void io_heavy_task(void *arg1, void *arg2, void *arg3) {
    const char *msg = "IO heavy task logging...\r\n";

    while (1) {
        // Send string over USB serial
        printk("%s", msg);

        // simulate moderate I/O period
        k_msleep(10);  
    }
}

peridoic:
#include <zephyr/kernel.h>
#include <zephyr/drivers/gpio.h>

#define PERIOD_MS 10

void periodic_task(void *arg1, void *arg2, void *arg3) {
    const struct device *gpio_dev;
    gpio_dev = DEVICE_DT_GET(DT_NODELABEL(gpio0));

    gpio_pin_configure(gpio_dev, PIN_NUM, GPIO_OUTPUT_ACTIVE);

    int count = 0;
    int64_t next_release = k_uptime_get();

    while (1) {
        // Toggle pin
        gpio_pin_toggle(gpio_dev, PIN_NUM);

        // Schedule next release
        next_release += PERIOD_MS;
        int64_t now = k_uptime_get();

        // Measure jitter (deviation from ideal start)
        int64_t jitter = now - next_release;
        metrics_record_jitter(jitter);

        // Sleep until next period
        k_sleep(K_MSEC(next_release - now));

        count++;
    }
}



