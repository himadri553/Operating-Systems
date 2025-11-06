#include <zephyr/kernel.h>
#include <zephyr/device.h>
#include <zephyr/drivers/uart.h>
#include <zephyr/usb/usb_device.h>
#include <zephyr/sys/printk.h>
#include <string.h>
#include <inttypes.h>

#define IO_PERIOD_MS    10      // how often we send
#define MSG_LEN         48      // fixed length so TX time is stable

static const struct device *cdc_dev;
static struct k_timer send_tmr, print_tmr;
static uint8_t tx_buf[MSG_LEN];

/* latency metrics (microseconds) */
static volatile uint64_t samples;
static volatile uint32_t min_us = UINT32_MAX;
static volatile uint32_t max_us = 0;
static volatile uint64_t sum_us = 0;

/* schedule target (in cycles) so ISR can measure lateness */
static volatile uint32_t next_release_cyc;

static inline uint32_t cyc_now(void) { return k_cycle_get_32(); }
static inline uint32_t cyc_to_us(uint32_t cyc) { return (uint32_t)k_cyc_to_us_floor64((uint64_t)cyc); }

static void add_sample(uint32_t us)
{
    samples++;
    sum_us += us;
    if (us < min_us) min_us = us;
    if (us > max_us) max_us = us;
}

/* UART ISR callback: runs when TX completes (UART_TX_DONE) */
static void uart_cb(const struct device *dev, struct uart_event *evt, void *user_data)
{
    ARG_UNUSED(dev); ARG_UNUSED(user_data);

    if (evt->type == UART_TX_DONE) {
        uint32_t isr_cyc = cyc_now();
        uint32_t target  = next_release_cyc;
        if (target != 0u) {
            uint32_t delta_cyc = isr_cyc - target;     // how late we are vs planned
            uint32_t us = cyc_to_us(delta_cyc);
            add_sample(us);
        }
    }
}

static void send_tmr_cb(struct k_timer *tmr)
{
    ARG_UNUSED(tmr);

    /* plan next release using exact period in cycles */
    uint32_t period_cyc = k_us_to_cyc_ceil32(IO_PERIOD_MS * 1000u);
    uint32_t target = next_release_cyc ? (next_release_cyc + period_cyc) : cyc_now();
    next_release_cyc = target;

    /* fixed-size message */
    static uint32_t seq;
    int n = snprintk((char*)tx_buf, sizeof(tx_buf), "IO seq=%" PRIu32 " cyc=%" PRIu32 "\r\n", seq++, cyc_now());
    if (n < 0) n = 0;
    if ((size_t)n < sizeof(tx_buf)) memset(tx_buf + n, ' ', sizeof(tx_buf) - (size_t)n);

    /* kick async TX; TX_DONE will fire in ISR and call uart_cb */
    (void)uart_tx(cdc_dev, tx_buf, sizeof(tx_buf), SYS_FOREVER_MS);
}

static void print_tmr_cb(struct k_timer *tmr)
{
    ARG_UNUSED(tmr);
    uint64_t n = samples;
    if (n == 0) {
        printk("I/O latency: collecting...\r\n");
        return;
    }
    uint32_t avg = (uint32_t)(sum_us / n);
    printk("USB TX ISR latency (proxy): n=%" PRIu64 ", min=%uus, avg=%uus, max=%uus\r\n",
           n, min_us, avg, max_us);
}

int io_heavy_start(void)
{
    int ret = usb_enable(NULL);
    if (ret) { printk("usb_enable failed: %d\r\n", ret); return ret; }

#ifdef DT_CHOSEN_ZEPHYR_CONSOLE
    cdc_dev = DEVICE_DT_GET(DT_CHOSEN(zephyr_console));
#else
    cdc_dev = device_get_binding("CDC_ACM_0");
#endif
    if (!cdc_dev) { printk("CDC ACM UART not ready\r\n"); return -ENODEV; }

#ifdef CONFIG_UART_LINE_CTRL
    uint32_t dtr = 0;
    /* wait for terminal to open so prints are visible */
    while (!dtr) {
        (void)uart_line_ctrl_get(cdc_dev, UART_LINE_CTRL_DTR, &dtr);
        k_sleep(K_MSEC(50));
    }
#endif

    uart_callback_set(cdc_dev, uart_cb, NULL);

    samples = 0; sum_us = 0; min_us = UINT32_MAX; max_us = 0; next_release_cyc = 0;

    k_timer_init(&send_tmr,  send_tmr_cb,  NULL);
    k_timer_init(&print_tmr, print_tmr_cb, NULL);
    k_timer_start(&send_tmr,  K_MSEC(100), K_MSEC(IO_PERIOD_MS)); // start after 100 ms, then every 10 ms
    k_timer_start(&print_tmr, K_SECONDS(1), K_SECONDS(1));        // print stats every 1 s

    printk("I/O heavy task started: %dB every %d ms\r\n", MSG_LEN, IO_PERIOD_MS);
    return 0;
}
