#!/bin/bash

PORT=4000
GDBPORT=1234
SOCAT="socat"
STRACE="strace"
GDBSERVER="gdbserver"
WRAPPER="command-wrapper" # clean up ENV

SOCAT_OPTION=""
STRACE_OPTION="-ivf -s 4096 -x"

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
        -E)
            COMMAND_WRAPPER_ENV="$2"
            shift 2
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
            SOCAT_OPTION="$SOCAT_OPTION,pty,raw,echo=0"
            #SOCAT_OPTION="$SOCAT_OPTION,pty,raw,echo=0,stderr"
            shift
            ;;
        -l)
            # ltrace mode
            STRACE="/usr/bin/ltrace"
            STRACE_OPTION="-ifC -s 100"
            shift
            ;;
        -g)
            WITH_GDBSERVER=1
            NOSTRACE=1
            shift
            ;;
        -h)
            echo "Usage: d [-p <port>] [-n] [-e <event>] [-w] [-q] [-l] [-g] [-h] <binary>"
            echo "  -p <port>   listen port number"
            echo "  -n          no trace"
            echo "  -E <...>    set environment variable when using command-wrapper"
            echo "  -e <event>  event (e.g. execve,read,write)"
            echo "  -w          no wrapper"
            echo "  -q          socat option: pty,raw,echo=0"
            echo "  -l          use ltrace"
            echo "  -g          gdbserver"
            exit 0
            ;;
        --) shift
            break
            ;;
    esac
done

if [ $WITH_GDBSERVER ]; then
    CMD="$GDBSERVER localhost\:$GDBPORT $@"
else
    if [ $NOWRAPPER ]; then
        CMD="$@"
    else
        export COMMAND_WRAPPER_ENV
        CMD="$WRAPPER $@"
    fi
fi

echo "listening on :$PORT"
if [ $NOSTRACE ]; then
    $SOCAT tcp-l:"$PORT,reuseaddr,fork" "exec:$CMD"$SOCAT_OPTION
else
    $SOCAT tcp-l:"$PORT,reuseaddr,fork" "exec:$STRACE $STRACE_OPTION '$CMD'"$SOCAT_OPTION
fi
