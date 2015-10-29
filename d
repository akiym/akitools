#!/bin/bash

PORT=4000
SOCAT="/usr/bin/socat"
STRACE="/usr/bin/strace"
WRAPPER="command-wrapper" # clean up ENV

SOCAT_OPTION=""
STRACE_OPTION="-ivf -s 100"

for OPT in $*
do
    case $OPT in
        -p)
            PORT=$2
            shift 2
            ;;
        -n)
            NOSTRACE=1
            shift
            ;;
        -e)
            # event: d -e execve,read,write
            STRACE_OPTION="$STRACE_OPTION -e'$2'"
            shift 2
            ;;
        -w)
            NOWRAPPER=1
            shift
            ;;
        -q)
            SOCAT_OPTION="$SOCAT_OPTION,pty,raw,echo=0,stderr"
            STRACE_OPTION="$STRACE_OPTION -o strace"
            shift
            ;;
        --) shift
            break
            ;;
    esac
done

if [ $NOWRAPPER ]; then
    CMD="$@"
else
    CMD="$WRAPPER $@"
fi

echo "listening on :$PORT"
if [ $NOSTRACE ]; then
    $SOCAT tcp-l:"$PORT,reuseaddr,fork" exec:"$CMD"$OPTION
else
    $SOCAT tcp-l:"$PORT,reuseaddr,fork" exec:"$STRACE $STRACE_OPTION '$CMD'"$OPTION
fi
