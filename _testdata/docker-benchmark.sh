#!/usr/bin/zsh
#!/bin/sh

# no arguments: tests various parameters
if [ -z "$1" ]; then
  runit() {
    export TIME="$2 $3 %e"
    echo "$2 $3:"
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
#echo "$args" | xargs -P "$parallel" -I{} -- /bin/echo "hi"

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

## 01 01 0.76
## 05 01 4.29
## 10 01 8.41
## 10 05 3.48
## 10 10 3.02
## 20 10 5.64
## 20 20 5.32

## OSX 10.12.4 Core i7-4960HQ @ 2.60GHz

## Client:
##  Version:      17.03.1-ce
##  API version:  1.27
##  Go version:   go1.7.5
##  Git commit:   c6d412e
##  Built:        Tue Mar 28 00:40:02 2017
##  OS/Arch:      darwin/amd64
## 
## Server:
##  Version:      17.03.1-ce
##  API version:  1.27 (minimum version 1.12)
##  Go version:   go1.7.5
##  Git commit:   c6d412e
##  Built:        Fri Mar 24 00:00:50 2017
##  OS/Arch:      linux/amd64
##  Experimental: true

## 01 01  1.23
## 05 01  9.06
## 10 01 18.48
## 10 05  5.76
## 10 10  6.85
## 20 10 17.92
## 20 20 26.77

## Arch Core i7-4960HQ @ 2.60GHz

## Client:
##  Version:      17.04.0-ce
##  API version:  1.28
##  Go version:   go1.8
##  Git commit:   4845c567eb
##  Built:        Sat Apr  8 18:55:45 2017
##  OS/Arch:      linux/amd64
## 
## Server:
##  Version:      17.04.0-ce
##  API version:  1.28 (minimum version 1.12)
##  Go version:   go1.8
##  Git commit:   4845c567eb
##  Built:        Sat Apr  8 18:55:45 2017
##  OS/Arch:      linux/amd64
##  Experimental: false

## 01 01  0.76
## 05 01  4.65
## 10 01  8.74
## 10 05 50.29
## 10 10 27.21
## 20 10 65.05
## 20 20 44.40
