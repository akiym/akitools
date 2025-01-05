#!/usr/bin/env python
import json
import subprocess
from datetime import datetime

p = subprocess.run(['step', 'crypto', 'jwt', 'inspect', '--insecure'], capture_output=True)
p.check_returncode()

j = json.loads(p.stdout.decode())
for k, v in j['payload'].items():
    if isinstance(v, int) and 1000000000 <= v <= 2100000000:
        j['payload'][k] = f'[*] {datetime.fromtimestamp(v).astimezone().strftime("%Y-%m-%dT%H:%M:%S%z")} ({v})'
del j['signature']

subprocess.run(['jq', '.'], input=json.dumps(j, indent=2).encode())
