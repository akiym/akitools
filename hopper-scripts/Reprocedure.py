doc = Document.getCurrentDocument()
if doc.message('Reprocedure?', ['Cancel','OK']) == 'OK':
    seg = doc.getCurrentSegment()
    current_addr = doc.getCurrentAddress()
    procedure = seg.getProcedureAtAddress(current_addr)
    addr = procedure.getEntryPoint()
    seg.markAsProcedure(addr)
