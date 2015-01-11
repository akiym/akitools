# requirement: websocket-client
# install the library to /Library/Python/2.7/site-packages: `sudo /usr/bin/easy_install websocket-client`
import websocket

ws = websocket.create_connection('ws://127.0.0.1:3003')

def update_address(t, addr):
    ws.send('@set%s 0x%x' % (t, addr))

doc = Document.getCurrentDocument()
seg = doc.getCurrentSegment()
addr = doc.getCurrentAddress()

if seg.getTypeAtAddress(addr) in (10, 11): # code, procedure
    update_address('iaddr', addr)
else:
    update_address('daddr', addr)
