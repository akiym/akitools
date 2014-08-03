doc = Document.getCurrentDocument()
current_addr = doc.getCurrentAddress()
seg = doc.getCurrentSegment()
refs = seg.getReferencesFromAddress(current_addr)
if len(refs) > 0:
    addr = refs[0]
    ref = doc.getSegmentAtAddress(addr)
    text = ''
    while True:
        byte = ref.readByte(addr)
        if not byte:
            break
        addr += 1
        text += chr(byte)
    if text != '':
        comment = '"%s"' % repr(text)[1:-1]
        seg.setInlineCommentAtAddress(current_addr, comment)
