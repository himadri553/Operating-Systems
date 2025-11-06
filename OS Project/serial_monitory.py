"""
    Himadri Saha

    serial_monitor.py
    Is the serial monitor for the Raspberry Pi Pico's print and other messages 

"""
import serial
import time

# Make sure to adjust 
PORT = "COM7"
BAUD = 115200

try:
    with serial.Serial(PORT, BAUD, timeout=0.5) as ser:
        print(f"Connected to {PORT} at {BAUD} baud.\n--- Serial Monitor ---")
        while True:
            if ser.in_waiting:
                line = ser.readline().decode(errors='ignore').strip()
                if line:
                    print(line)
            time.sleep(0.01)

except serial.SerialException as e:
    print(f"Error: {e}")
except KeyboardInterrupt:
    print("\nExiting serial monitor.")
