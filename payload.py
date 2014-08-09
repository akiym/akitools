# -*- coding: utf-8 -*-
import os
import sys
import time
import re
from pwn import *

REMOTE = False

if REMOTE:
    host = ''
    port = 0
else:
    host = '127.0.0.1'
    port = 4444

def connect():
    return remote(host, port)

s = connect()

payload = (
)
s.send(payload)

s.interactive()
