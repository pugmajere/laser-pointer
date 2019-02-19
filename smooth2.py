#!/usr/bin/env python

import RPi.GPIO as GPIO
import code
import datetime
import math
import pantilthat
import readline
import rlcompleter
import time


GPIO_LASER = 18

while False:
    # Get the time in seconds
    t = time.time()

    # Generate an angle using a sine wave (-1 to 1) multiplied by 90 (-90 to 90)
    a = math.sin(t * 0.5) * 90
    
    # Cast a to int for v0.0.2
    a = int(a)

    # pantilthat.pan(a)
    pantilthat.tilt(a)

    # Two decimal places is quite enough!
    print(round(a,2))

    # Sleep for a bit so we're not hammering the HAT with updates
    time.sleep(0.05)


def initLaser():
    GPIO.setmode(GPIO.BCM)
    GPIO.setup(GPIO_LASER, GPIO.OUT)

    
def shutdownLaser():
    GPIO.output(GPIO_LASER, 0)
    GPIO.cleanup()

def shootLaser():
    GPIO.output(GPIO_LASER, 1)

def shootCircle():
    shootLaser()

    while True:
        for i in xrange(90):
            pan_angle = i - 90  # Offset around center
            pantilthat.pan(pan_angle)

            tilt_angle = i / 2 + 45  #  Range from 15-45e
            pantilthat.tilt(tilt_angle)

            time.sleep(0.05)

        for i in xrange(90):
            pan_angle = 0 - i
            tilt_angle = 90 - i / 2
            pantilthat.pan(pan_angle)
            pantilthat.tilt(tilt_angle)

            time.sleep(0.05)
            


if __name__ == '__main__':
    initLaser()

    try:
        injected = locals().copy()
        readline.set_completer(rlcompleter.Completer(injected).complete)
        readline.parse_and_bind('tab: complete')
        code.InteractiveConsole(injected).interact('Laser console')
 
    finally:
        shutdownLaser()
