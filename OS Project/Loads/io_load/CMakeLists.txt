cmake_minimum_required(VERSION 3.20.0)
find_package(Zephyr REQUIRED HINTS $ENV{ZEPHYR_BASE})
project(rtos_io_demo)
target_sources(app PRIVATE src/main.c src/io_heavy.c)
