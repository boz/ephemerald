#!/bin/sh

# no arguments: tests various parameters
if [ -z "$1" ]; then
  runit() {
    export TIME="$2 $3 %e"
    time "$1" "$2" "$3"
  }
  runit "$0" 01 01
  runit "$0" 05 01
  runit "$0" 10 01
  runit "$0" 10 05
  runit "$0" 10 10
  runit "$0" 20 10
  runit "$0" 20 20
  exit
fi

# count:    number of docker containers to create
# parallel: number of parallel processes

count=$1
parallel=$2

args=""

for i in $(seq "$count"); do
  args="$args$i\n"
done

echo "$args" | xargs -P "$parallel" -I{} -- docker run --rm busybox

###
### RESULTS
###

## Ubuntu i7-6800K CPU @ 3.40GHz
## 
## Client:
##  Version:      17.03.1-ce
##  API version:  1.27
##  Go version:   go1.7.5
##  Git commit:   c6d412e
##  Built:        Mon Mar 27 17:17:43 2017
##  OS/Arch:      linux/amd64
## 
## Server:
##  Version:      17.03.1-ce
##  API version:  1.27 (minimum version 1.12)
##  Go version:   go1.7.5
##  Git commit:   c6d412e
##  Built:        Mon Mar 27 17:17:43 2017
##  OS/Arch:      linux/amd64
##  Experimental: false

# 01 01 0.76
# 05 01 4.29
# 10 01 8.41
# 10 05 3.48
# 10 10 3.02
# 20 10 5.64
# 20 20 5.32
