import re

names = {
    'stack': [],
    'var': [],
    'arg': [],
}
pattern = {
    'stack': re.compile('esp\+0x([0-9a-f]+)'),
    'var': re.compile('ebp-0x([0-9a-f]+)\+var_(\d+)'),
    'arg': re.compile('arg_offset_x([0-9a-f]+)'),
}
compare = {
    'stack': lambda x, y: cmp(int(pattern['stack'].search(x).group(1), 16), int(pattern['stack'].search(y).group(1), 16)),
    'var': lambda x, y: cmp(int(pattern['var'].search(x).group(2)), int(pattern['var'].search(y).group(2))),
    'arg': lambda x, y: cmp(int(pattern['arg'].search(x).group(1), 16), int(pattern['arg'].search(y).group(1), 16)),
}

def get_end_of_procedure(procedure):
    end_addrs = []
    for i in range(0, procedure.getBasicBlockCount()):
        block = procedure.getBasicBlock(i)
        end_addrs.append(block.getEndingAddress())
    return max(end_addrs)

def get_var_names(addr):
    try:
        inst = seg.getInstructionAtAddress(addr)
        for i in range(0, inst.getArgumentCount()):
            var_name = inst.getFormattedArgument(i)
            for key in ['stack', 'var', 'arg']:
                if pattern[key].search(var_name):
                    names[key].append(var_name)
    except:
        pass

doc = Document.getCurrentDocument()
seg = doc.getCurrentSegment()
current_addr = doc.getCurrentAddress()
procedure = seg.getProcedureAtAddress(current_addr)
begin_addr = procedure.getEntryPoint()
end_addr = get_end_of_procedure(procedure)

for addr in range(begin_addr, end_addr+1):
    get_var_names(addr)

comment = '\n'
for key in ['stack', 'var', 'arg']:
    names[key] = list(set(names[key]))
    names[key].sort(cmp=compare[key])
    for name in names[key]:
        if key == 'stack':
            comment += name + '\n'
        elif key == 'var':
            m = pattern[key].search(name)
            offset = int(m.group(2)) - int(m.group(1), 16)
            comment += name + '= ' + str(offset) + '\n'
        elif key == 'arg':
            m = pattern[key].search(name)
            offset = int(m.group(1), 16) + 8
            comment += name + '= ' + str(offset) + '\n'

seg.setCommentAtAddress(begin_addr, comment)
