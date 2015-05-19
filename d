#!/bin/bash

PORT=4000
SOCAT="/usr/bin/socat"
WRAPPER='command-wrapper'

for OPT in $*
do
    case $OPT in
        -e)
            EVENTS=$2
            shift 2
            ;;
        -p)
            PORT=$2
            shift 2
            ;;
        -n)
            NOSTRACE=1
            shift
            ;;
        --) shift
            break
            ;;
    esac
done

if [ ! -x $1 ]; then
    echo "couldn't exec: $1"
    exit -1
fi

CMD="$WRAPPER $@"

#$SOCAT "tcp-l:$PORT,reuseaddr,fork" exec:"/usr/bin/strace -ivf -s100 $CMD",pty

echo "listening on :$PORT"
if [ $NOSTRACE ]; then
    $SOCAT tcp-l:"$PORT,reuseaddr,fork" exec:"$CMD"
else
    if [ $EVENTS ]; then
        $SOCAT tcp-l:"$PORT,reuseaddr,fork" exec:"/usr/bin/strace -ivf -e'$EVENTS' -s100 $CMD"
    else
        $SOCAT tcp-l:"$PORT,reuseaddr,fork" exec:"/usr/bin/strace -ivf -s100 $CMD"
    fi
fi
