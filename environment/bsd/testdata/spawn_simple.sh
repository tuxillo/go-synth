#!/bin/sh
# Spawn 3 background sleeps, then wait forever
sleep 9999 &
sleep 9999 &
sleep 9999 &
wait
