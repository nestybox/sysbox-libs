#!/bin/bash

#
# Runs the pidmonitor unit tests for a given number of iterations.
#
# Usage: test <iterations>
#

iter=$1

for i in `seq 1 $iter`; do
  result=$(go test -v)
  echo $result

  failed=$(echo $result | grep -c "FAIL")
  if [ $failed -ne 0 ]; then
    echo "TEST FAILED"
    exit 1
  fi
done

printf "TEST PASSED (%d iterations)\n" $iter
exit 0
