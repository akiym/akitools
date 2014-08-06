import re

def get_end_of_procedure(procedure):
    end_addrs = []
    for i in range(0, procedure.getBasicBlockCount()):
        block = procedure.getBasicBlock(i)
        end_addrs.append(block.getEndingAddress())
    return max(end_addrs)

arg_pattern = re.compile('arg_offset_')
var_pattern = re.compile('var_')
stack_pattern = re.compile('esp\+0x')
var_names = []
arg_names = []
stack_names = []
def get_var_names(addr):
    try:
        inst = seg.getInstructionAtAddress(addr)
        for i in range(0, inst.getArgumentCount()):
            var_name = inst.getFormattedArgument(i)
            if arg_pattern.search(var_name):
                arg_names.append(var_name)
            elif var_pattern.search(var_name):
                var_names.append(var_name)
            elif stack_pattern.search(var_name):
                stack_names.append(var_name)
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

def var_compare(x, y):
    pattern = re.compile('var_(\d+)')
    m = pattern.search(x)
    n = pattern.search(y)
    return cmp(int(m.group(1)), int(n.group(1)))

def stack_compare(x, y):
    pattern = re.compile('esp\+0x([0-9a-f]+)')
    m = pattern.search(x)
    n = pattern.search(y)
    return cmp(int(m.group(1), 16), int(n.group(1), 16))

def arg_compare(x, y):
    pattern = re.compile('arg_offset_x([0-9a-f]+)')
    m = pattern.search(x)
    n = pattern.search(y)
    return cmp(int(m.group(1), 16), int(n.group(1), 16))

var_names = list(set(var_names))
var_names.sort(cmp=var_compare)
stack_names = list(set(stack_names))
stack_names.sort(cmp=stack_compare, reverse=True)
arg_names = list(set(arg_names))
arg_names.sort(cmp=arg_compare)

comment = '\n'
for name in var_names:
    comment += name + '\n'
for name in stack_names:
    comment += name + '\n'
for name in arg_names:
    comment += name + '\n'

seg.setCommentAtAddress(begin_addr, comment)
