# requirement: websocket-client
# you have to put library in /Library/Python/2.7/site-packages.
# % sudo /usr/bin/easy_install websocket-client
import websocket

ws = websocket.create_connection('ws://127.0.0.1:3003')
ws.send('@setaddress')
dat = ws.recv().split(" ")
addr = int(dat[1][2:], 16)

doc = Document.getCurrentDocument()
doc.moveCursorAtAddress(addr)
