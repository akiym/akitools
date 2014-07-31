import os

doc = Document.getCurrentDocument()
addr = doc.getCurrentAddress()
os.system("echo '%s' | tr -d '\n' | pbcopy" % hex(addr).rstrip('L'))
