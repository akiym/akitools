# Linux i386, x86_64
import re

syscall32 = ['restart_syscall', 'exit', 'fork', 'read', 'write', 'open', 'close', 'waitpid', 'creat', 'link', 'unlink', 'execve', 'chdir', 'time', 'mknod', 'chmod', 'lchown16', 'not implemented', 'stat', 'lseek', 'getpid', 'mount', 'oldumount', 'setuid16', 'getuid16', 'stime', 'ptrace', 'alarm', 'fstat', 'pause', 'utime', 'not implemented', 'not implemented', 'access', 'nice', 'not implemented', 'sync', 'kill', 'rename', 'mkdir', 'rmdir', 'dup', 'pipe', 'times', 'not implemented', 'brk', 'setgid16', 'getgid16', 'signal', 'geteuid16', 'getegid16', 'acct', 'umount', 'not implemented', 'ioctl', 'fcntl', 'not implemented', 'setpgid', 'not implemented', 'olduname', 'umask', 'chroot', 'ustat', 'dup2', 'getppid', 'getpgrp', 'setsid', 'sigaction', 'sgetmask', 'ssetmask', 'setreuid16', 'setregid16', 'sigsuspend', 'sigpending', 'sethostname', 'setrlimit', 'old_getrlimit', 'getrusage', 'gettimeofday', 'settimeofday', 'getgroups16', 'setgroups16', 'old_select', 'symlink', 'lstat', 'readlink', 'uselib', 'swapon', 'reboot', 'old_readdir', 'old_mmap', 'munmap', 'truncate', 'ftruncate', 'fchmod', 'fchown16', 'getpriority', 'setpriority', 'not implemented', 'statfs', 'fstatfs', 'ioperm', 'socketcall', 'syslog', 'setitimer', 'getitimer', 'newstat', 'newlstat', 'newfstat', 'uname', 'iopl', 'vhangup', 'not implemented', 'vm86old', 'wait4', 'swapoff', 'sysinfo', 'ipc', 'fsync', 'sigreturn', 'clone', 'setdomainname', 'newuname', 'modify_ldt', 'adjtimex', 'mprotect', 'sigprocmask', 'not implemented', 'init_module', 'delete_module', 'not implemented', 'quotactl', 'getpgid', 'fchdir', 'bdflush', 'sysfs', 'personality', 'not implemented', 'setfsuid16', 'setfsgid16', 'llseek', 'getdents', 'select', 'flock', 'msync', 'readv', 'writev', 'getsid', 'fdatasync', 'sysctl', 'mlock', 'munlock', 'mlockall', 'munlockall', 'sched_setparam', 'sched_getparam', 'sched_setscheduler', 'sched_getscheduler', 'sched_yield', 'sched_get_priority_max', 'sched_get_priority_min', 'sched_rr_get_interval', 'nanosleep', 'mremap', 'setresuid16', 'getresuid16', 'vm86', 'not implemented', 'poll', 'nfsservctl', 'setresgid16', 'getresgid16', 'prctl', 'rt_sigreturn', 'rt_sigaction', 'rt_sigprocmask', 'rt_sigpending', 'rt_sigtimedwait', 'rt_sigqueueinfo', 'rt_sigsuspend', 'pread64', 'pwrite64', 'chown16', 'getcwd', 'capget', 'capset', 'sigaltstack', 'sendfile', 'not implemented', 'not implemented', 'vfork', 'getrlimit', 'mmap_pgoff', 'truncate64', 'ftruncate64', 'stat64', 'lstat64', 'fstat64', 'lchown', 'getuid', 'getgid', 'geteuid', 'getegid', 'setreuid', 'setregid', 'getgroups', 'setgroups', 'fchown', 'setresuid', 'getresuid', 'setresgid', 'getresgid', 'chown', 'setuid', 'setgid', 'setfsuid', 'setfsgid', 'pivot_root', 'mincore', 'madvise', 'getdents64', 'fcntl64', 'not implemented', 'not implemented', 'gettid', 'readahead', 'setxattr', 'lsetxattr', 'fsetxattr', 'getxattr', 'lgetxattr', 'fgetxattr', 'listxattr', 'llistxattr', 'flistxattr', 'removexattr', 'lremovexattr', 'fremovexattr', 'tkill', 'sendfile64', 'futex', 'sched_setaffinity', 'sched_getaffinity', 'set_thread_area', 'get_thread_area', 'io_setup', 'io_destroy', 'io_getevents', 'io_submit', 'io_cancel', 'fadvise64', 'not implemented', 'exit_group', 'lookup_dcookie', 'epoll_create', 'epoll_ctl', 'epoll_wait', 'remap_file_pages', 'set_tid_address', 'timer_create', 'timer_settime', 'timer_gettime', 'timer_getoverrun', 'timer_delete', 'clock_settime', 'clock_gettime', 'clock_getres', 'clock_nanosleep', 'statfs64', 'fstatfs64', 'tgkill', 'utimes', 'fadvise64_64', 'not implemented', 'mbind', 'get_mempolicy', 'set_mempolicy', 'mq_open', 'mq_unlink', 'mq_timedsend', 'mq_timedreceive', 'mq_notify', 'mq_getsetattr', 'kexec_load', 'waitid', 'not implemented', 'add_key', 'request_key', 'keyctl', 'ioprio_set', 'ioprio_get', 'inotify_init', 'inotify_add_watch', 'inotify_rm_watch', 'migrate_pages', 'openat', 'mkdirat', 'mknodat', 'fchownat', 'futimesat', 'fstatat64', 'unlinkat', 'renameat', 'linkat', 'symlinkat', 'readlinkat', 'fchmodat', 'faccessat', 'pselect6', 'ppoll', 'unshare', 'set_robust_list', 'get_robust_list', 'splice', 'sync_file_range', 'tee', 'vmsplice', 'move_pages', 'getcpu', 'epoll_pwait', 'utimensat', 'signalfd', 'timerfd_create', 'eventfd', 'fallocate', 'timerfd_settime', 'timerfd_gettime', 'signalfd4', 'eventfd2', 'epoll_create1', 'dup3', 'pipe2', 'inotify_init1', 'preadv', 'pwritev', 'rt_tgsigqueueinfo', 'perf_event_open', 'recvmmsg']
syscall64 = ['read', 'write', 'open', 'close', 'stat', 'fstat', 'lstat', 'poll', 'lseek', 'mmap', 'mprotect', 'munmap', 'brk', 'rt_sigaction', 'rt_sigprocmask', 'rt_sigreturn', 'ioctl', 'pread64', 'pwrite64', 'readv', 'writev', 'access', 'pipe', 'select', 'sched_yield', 'mremap', 'msync', 'mincore', 'madvise', 'shmget', 'shmat', 'shmctl', 'dup', 'dup2', 'pause', 'nanosleep', 'getitimer', 'alarm', 'setitimer', 'getpid', 'sendfile', 'socket', 'connect', 'accept', 'sendto', 'recvfrom', 'sendmsg', 'recvmsg', 'shutdown', 'bind', 'listen', 'getsockname', 'getpeername', 'socketpair', 'setsockopt', 'getsockopt', 'clone', 'fork', 'vfork', 'execve', 'exit', 'wait4', 'kill', 'uname', 'semget', 'semop', 'semctl', 'shmdt', 'msgget', 'msgsnd', 'msgrcv', 'msgctl', 'fcntl', 'flock', 'fsync', 'fdatasync', 'truncate', 'ftruncate', 'getdents', 'getcwd', 'chdir', 'fchdir', 'rename', 'mkdir', 'rmdir', 'creat', 'link', 'unlink', 'symlink', 'readlink', 'chmod', 'fchmod', 'chown', 'fchown', 'lchown', 'umask', 'gettimeofday', 'getrlimit', 'getrusage', 'sysinfo', 'times', 'ptrace', 'getuid', 'syslog', 'getgid', 'setuid', 'setgid', 'geteuid', 'getegid', 'setpgid', 'getppid', 'getpgrp', 'setsid', 'setreuid', 'setregid', 'getgroups', 'setgroups', 'setresuid', 'getresuid', 'setresgid', 'getresgid', 'getpgid', 'setfsuid', 'setfsgid', 'getsid', 'capget', 'capset', 'rt_sigpending', 'rt_sigtimedwait', 'rt_sigqueueinfo', 'rt_sigsuspend', 'sigaltstack', 'utime', 'mknod', 'not implemented', 'personality', 'ustat', 'statfs', 'fstatfs', 'sysfs', 'getpriority', 'setpriority', 'sched_setparam', 'sched_getparam', 'sched_setscheduler', 'sched_getscheduler', 'sched_get_priority_max', 'sched_get_priority_min', 'sched_rr_get_interval', 'mlock', 'munlock', 'mlockall', 'munlockall', 'vhangup', 'modify_ldt', 'pivot_root', '_sysctl', 'prctl', 'arch_prctl', 'adjtimex', 'setrlimit', 'chroot', 'sync', 'acct', 'settimeofday', 'mount', 'umount2', 'swapon', 'swapoff', 'reboot', 'sethostname', 'setdomainname', 'iopl', 'ioperm', 'create_module', 'init_module', 'delete_module', 'get_kernel_syms', 'query_module', 'quotactl', 'not implemented', 'not implemented', 'not implemented', 'not implemented', 'not implemented', 'not implemented', 'gettid', 'readahead', 'setxattr', 'lsetxattr', 'fsetxattr', 'getxattr', 'lgetxattr', 'fgetxattr', 'listxattr', 'llistxattr', 'flistxattr', 'removexattr', 'lremovexattr', 'fremovexattr', 'tkill', 'time', 'futex', 'sched_setaffinity', 'sched_getaffinity', 'set_thread_area', 'io_setup', 'io_destroy', 'io_getevents', 'io_submit', 'io_cancel', 'get_thread_area', 'lookup_dcookie', 'epoll_create', 'not implemented', 'not implemented', 'remap_file_pages', 'getdents64', 'set_tid_address', 'restart_syscall', 'semtimedop', 'fadvise64', 'timer_create', 'timer_settime', 'timer_gettime', 'timer_getoverrun', 'timer_delete', 'clock_settime', 'clock_gettime', 'clock_getres', 'clock_nanosleep', 'exit_group', 'epoll_wait', 'epoll_ctl', 'tgkill', 'utimes', 'not implemented', 'mbind', 'set_mempolicy', 'get_mempolicy', 'mq_open', 'mq_unlink', 'mq_timedsend', 'mq_timedreceive', 'mq_notify', 'mq_getsetattr', 'kexec_load', 'waitid', 'add_key', 'request_key', 'keyctl', 'ioprio_set', 'ioprio_get', 'inotify_init', 'inotify_add_watch', 'inotify_rm_watch', 'migrate_pages', 'openat', 'mkdirat', 'mknodat', 'fchownat', 'futimesat', 'newfstatat', 'unlinkat', 'renameat', 'linkat', 'symlinkat', 'readlinkat', 'fchmodat', 'faccessat', 'pselect6', 'ppoll', 'unshare', 'set_robust_list', 'get_robust_list', 'splice', 'tee', 'sync_file_range', 'vmsplice', 'move_pages', 'utimensat', 'epoll_pwait', 'signalfd', 'timerfd_create', 'eventfd', 'fallocate', 'timerfd_settime', 'timerfd_gettime', 'accept4', 'signalfd4', 'eventfd2', 'epoll_create1', 'dup3', 'pipe2', 'inotify_init1', 'preadv', 'pwritev', 'rt_tgsigqueueinfo', 'perf_event_open', 'recvmmsg', 'fanotify_init', 'fanotify_mark', 'prlimit64', 'name_to_handle_at', 'open_by_handle_at', 'clock_adjtime', 'syncfs', 'sendmmsg', 'setns', 'getcpu', 'process_vm_readv', 'process_vm_writev']

# some code were taken from https://github.com/zTrix/magic

TYPE_EQUAL = 'equal'
TYPE_BITOR = 'bitor'

MAGICS = {   'fseek': [   {   },
                 {   },
                 {   'flags': [   (0, 'SEEK_SET'),
                                  (1, 'SEEK_CUR'),
                                  (2, 'SEEK_END')],
                     'type': TYPE_EQUAL}],
    'mmap': [   {   },
                {   },
                {   'flags': [   (1, 'PROT_READ'),
                                 (2, 'PROT_WRITE'),
                                 (4, 'PROT_EXEC')],
                    'type': TYPE_BITOR},
                {   'flags': [   (1, 'MAP_SHARED'),
                                 (2, 'MAP_PRIVATE'),
                                 (16, 'MAP_FIXED'),
                                 (32, 'MAP_ANONYMOUS'),
                                 (256, 'MAP_GROWSDOWN'),
                                 (2048, 'MAP_DENYWRITE'),
                                 (4096, 'MAP_EXECUTABLE'),
                                 (8192, 'MAP_LOCKED'),
                                 (16384, 'MAP_NORESERVE')],
                    'type': TYPE_BITOR}],
    'open': [   {   },
                {   'flags': [   (1, 'O_WRONLY'),
                                 (2, 'O_RDWR'),
                                 (64, 'O_CREAT'),
                                 (128, 'O_EXCL'),
                                 (256, 'O_NOCTTY'),
                                 (512, 'O_TRUNC'),
                                 (1024, 'O_APPEND'),
                                 (2048, 'O_NDELAY'),
                                 (2048, 'O_NONBLOCK'),
                                 (4096, 'O_DSYNC'),
                                 (8192, 'O_ASYNC'),
                                 (65536, 'O_DIRECTORY'),
                                 (131072, 'O_NOFOLLOW'),
                                 (1052672, 'O_SYNC')],
                    'type': TYPE_BITOR}],
    'prctl': [   {   'flags': [   (23, 'PR_CAPBSET_READ'),
                                  (24, 'PR_CAPBSET_DROP'),
                                  (36, 'PR_SET_CHILD_SUBREAPER'),
                                  (37, 'PR_GET_CHILD_SUBREAPER'),
                                  (4, 'PR_SET_DUMPABLE'),
                                  (3, 'PR_GET_DUMPABLE'),
                                  (20, 'PR_SET_ENDIAN'),
                                  (19, 'PR_GET_ENDIAN'),
                                  (10, 'PR_SET_FPEMU'),
                                  (9, 'PR_GET_FPEMU'),
                                  (12, 'PR_SET_FPEXC'),
                                  (11, 'PR_GET_FPEXC'),
                                  (8, 'PR_SET_KEEPCAPS'),
                                  (7, 'PR_GET_KEEPCAPS'),
                                  (15, 'PR_SET_NAME'),
                                  (16, 'PR_GET_NAME'),
                                  (38, 'PR_SET_NO_NEW_PRIVS'),
                                  (39, 'PR_GET_NO_NEW_PRIVS'),
                                  (1, 'PR_SET_PDEATHSIG'),
                                  (2, 'PR_GET_PDEATHSIG'),
                                  (1499557217, 'PR_SET_PTRACER'),
                                  (22, 'PR_SET_SECCOMP'),
                                  (21, 'PR_GET_SECCOMP'),
                                  (28, 'PR_SET_SECUREBITS'),
                                  (27, 'PR_GET_SECUREBITS'),
                                  (40, 'PR_GET_TID_ADDRESS'),
                                  (29, 'PR_SET_TIMERSLACK'),
                                  (30, 'PR_GET_TIMERSLACK'),
                                  (14, 'PR_SET_TIMING'),
                                  (13, 'PR_GET_TIMING'),
                                  (31, 'PR_TASK_PERF_EVENTS_DISABLE'),
                                  (32, 'PR_TASK_PERF_EVENTS_ENABLE'),
                                  (26, 'PR_SET_TSC'),
                                  (25, 'PR_GET_TSC'),
                                  (6, 'PR_SET_UNALIGN'),
                                  (5, 'PR_GET_UNALIGN'),
                                  (33, 'PR_MCE_KILL'),
                                  (34, 'PR_MCE_KILL_GET'),
                                  (35, 'PR_SET_MM')],
                     'type': TYPE_EQUAL}],
    'ptrace': [   {   'flags': [   (0, 'PTRACE_TRACEME'),
                                   (1, 'PTRACE_PEEKTEXT'),
                                   (2, 'PTRACE_PEEKDATA'),
                                   (3, 'PTRACE_PEEKUSER'),
                                   (4, 'PTRACE_POKETEXT'),
                                   (5, 'PTRACE_POKEDATA'),
                                   (6, 'PTRACE_POKEUSER'),
                                   (12, 'PTRACE_GETREGS'),
                                   (14, 'PTRACE_GETFPREGS'),
                                   (16900, 'PTRACE_GETREGSET'),
                                   (16898, 'PTRACE_GETSIGINFO'),
                                   (13, 'PTRACE_SETREGS'),
                                   (15, 'PTRACE_SETFPREGS'),
                                   (16901, 'PTRACE_SETREGSET'),
                                   (16899, 'PTRACE_SETSIGINFO'),
                                   (16896, 'PTRACE_SETOPTIONS'),
                                   (1048576, 'PTRACE_O_EXITKILL'),
                                   (8, 'PTRACE_O_TRACECLONE'),
                                   (16, 'PTRACE_O_TRACEEXEC'),
                                   (64, 'PTRACE_O_TRACEEXIT'),
                                   (2, 'PTRACE_O_TRACEFORK'),
                                   (1, 'PTRACE_O_TRACESYSGOOD'),
                                   (4, 'PTRACE_O_TRACEVFORK'),
                                   (32, 'PTRACE_O_TRACEVFORKDONE'),
                                   (16897, 'PTRACE_GETEVENTMSG'),
                                   (7, 'PTRACE_CONT'),
                                   (24, 'PTRACE_SYSCALL'),
                                   (9, 'PTRACE_SINGLESTEP'),
                                   (16904, 'PTRACE_LISTEN'),
                                   (8, 'PTRACE_KILL'),
                                   (16903, 'PTRACE_INTERRUPT'),
                                   (16, 'PTRACE_ATTACH'),
                                   (16902, 'PTRACE_SEIZE'),
                                   (17, 'PTRACE_DETACH'),
                                   (31, 'PTRACE_SYSEMU'),
                                   (32, 'PTRACE_SYSEMU_SINGLESTEP')],
                      'type': TYPE_EQUAL}],
    'signal': [   {   'flags': [   (1, 'SIGHUP'),
                                   (2, 'SIGINT'),
                                   (3, 'SIGQUIT'),
                                   (4, 'SIGILL'),
                                   (5, 'SIGTRAP'),
                                   (6, 'SIGABRT'),
                                   (6, 'SIGIOT'),
                                   (7, 'SIGBUS'),
                                   (8, 'SIGFPE'),
                                   (9, 'SIGKILL'),
                                   (10, 'SIGUSR1'),
                                   (11, 'SIGSEGV'),
                                   (12, 'SIGUSR2'),
                                   (13, 'SIGPIPE'),
                                   (14, 'SIGALRM'),
                                   (15, 'SIGTERM'),
                                   (17, 'SIGCHLD'),
                                   (18, 'SIGCONT'),
                                   (19, 'SIGSTOP'),
                                   (20, 'SIGTSTP'),
                                   (21, 'SIGTTIN'),
                                   (22, 'SIGTTOU'),
                                   (23, 'SIGURG'),
                                   (24, 'SIGXCPU'),
                                   (25, 'SIGXFSZ'),
                                   (26, 'SIGVTALRM'),
                                   (27, 'SIGPROF'),
                                   (28, 'SIGWINCH'),
                                   (29, 'SIGIO'),
                                   (31, 'SIGSYS')],
                      'type': TYPE_EQUAL}],
    'sigaction': [   {   'flags': [   (1, 'SIGHUP'),
                                      (2, 'SIGINT'),
                                      (3, 'SIGQUIT'),
                                      (4, 'SIGILL'),
                                      (5, 'SIGTRAP'),
                                      (6, 'SIGABRT'),
                                      (6, 'SIGIOT'),
                                      (7, 'SIGBUS'),
                                      (8, 'SIGFPE'),
                                      (9, 'SIGKILL'),
                                      (10, 'SIGUSR1'),
                                      (11, 'SIGSEGV'),
                                      (12, 'SIGUSR2'),
                                      (13, 'SIGPIPE'),
                                      (14, 'SIGALRM'),
                                      (15, 'SIGTERM'),
                                      (17, 'SIGCHLD'),
                                      (18, 'SIGCONT'),
                                      (19, 'SIGSTOP'),
                                      (20, 'SIGTSTP'),
                                      (21, 'SIGTTIN'),
                                      (22, 'SIGTTOU'),
                                      (23, 'SIGURG'),
                                      (24, 'SIGXCPU'),
                                      (25, 'SIGXFSZ'),
                                      (26, 'SIGVTALRM'),
                                      (27, 'SIGPROF'),
                                      (28, 'SIGWINCH'),
                                      (29, 'SIGIO'),
                                      (31, 'SIGSYS')],
                      'type': TYPE_EQUAL}],
    'socket': [   {   'flags': [   (1, 'AF_UNIX'),
                                   (2, 'AF_INET'),
                                   (17, 'AF_ROUTE'),
                                   (29, 'AF_KEY'),
                                   (30, 'AF_INET6'),
                                   (32, 'AF_SYSTEM'),
                                   (27, 'AF_NDRV')],
                      'type': TYPE_EQUAL},
                  {   'flags': [   (1, 'SOCK_STREAM'),
                                   (2, 'SOCK_DGRAM'),
                                   (3, 'SOCK_RAW'),
                                   (5, 'SOCK_SEQPACKET'),
                                   (4, 'SOCK_RDM')],
                      'type': TYPE_EQUAL},
                  {   'flags': [   (0, 'IPPROTO_IP'),
                                   (1, 'IPPROTO_ICMP'),
                                   (2, 'IPPROTO_IGMP'),
                                   (4, 'IPPROTO_IPIP'),
                                   (6, 'IPPROTO_TCP'),
                                   (8, 'IPPROTO_EGP'),
                                   (12, 'IPPROTO_PUP'),
                                   (17, 'IPPROTO_UDP'),
                                   (22, 'IPPROTO_IDP'),
                                   (29, 'IPPROTO_TP'),
                                   (33, 'IPPROTO_DCCP'),
                                   (41, 'IPPROTO_IPV6'),
                                   (46, 'IPPROTO_RSVP'),
                                   (47, 'IPPROTO_GRE'),
                                   (50, 'IPPROTO_ESP'),
                                   (51, 'IPPROTO_AH'),
                                   (92, 'IPPROTO_MTP'),
                                   (94, 'IPPROTO_BEETPH'),
                                   (98, 'IPPROTO_ENCAP'),
                                   (103, 'IPPROTO_PIM'),
                                   (108, 'IPPROTO_COMP'),
                                   (132, 'IPPROTO_SCTP'),
                                   (136, 'IPPROTO_UDPLITE'),
                                   (255, 'IPPROTO_RAW')],
                      'type': TYPE_EQUAL}]}

def magic(func, arg, number):
    flags = []
    try:
      obj = MAGICS[func][arg-1]
      for flag in obj['flags']:
          if obj['type'] == TYPE_BITOR:
              if flag[0] & number:
                  flags.append(flag[1])
          else:
              if flag[0] == number:
                  return [flag[1]]
    except:
      pass
    return flags

def guess_arguments(inst, syscall=False):
    try:
        arch = inst.getArchitecture()
        arg2 = int(inst.getRawArgument(1), 16)
        if arch == 1:
            # i386
            if syscall:
                reg = ['[re]?a[xl]', 'e?b[xl]', 'e?c[xl]', 'e?d[xl]', 'e?sil?', 'e?dil?', 'e?bpl?']
                for i, r in enumerate(reg):
                    if re.match(r, arg1):
                        return i, arg2
            else:
                m = re.search('esp(?:\+(0x[0-9a-f]+))?', inst.getRawArgument(0))
                if m:
                    arg1 = m.group(1)
                    if arg1:
                        return int(arg1, 16) / 4, arg2
                    else:
                        return 0, arg2
        elif arch == 2:
            # x86_64
            reg = ['[re]?a[xl]', '[re]?dil?', '[re]?sil?', '[re]?d[xl]', 'r10[dwb]?', 'r8[dwb]?', 'r9[dwb]?']
            arg1 = inst.getRawArgument(0)
            for i, r in enumerate(reg):
                if re.match(r, arg1):
                    return i, arg2
    except:
        pass

    # fail
    return -1, None

doc = Document.getCurrentDocument()
seg = doc.getCurrentSegment()

MAX_FIND_INSTS = 10

current_address = doc.getCurrentAddress()
current_inst = seg.getInstructionAtAddress(current_address)
if current_inst.getInstructionString() == 'mov':
    # call
    arg, value = guess_arguments(current_inst)
    if arg != -1:
        address = current_address
        for i in range(MAX_FIND_INSTS):
            inst = seg.getInstructionAtAddress(address)
            opcode = inst.getInstructionString()
            if opcode == 'call':
                func = inst.getFormattedArgument(0)
                func = func.replace('@PLT', '')
                flags = magic(func, arg, value)
                if len(flags) > 0:
                        comment = ' | '.join(flags)
                        seg.setInlineCommentAtAddress(current_address, comment)
                break
            address += inst.getInstructionLength()

    # syscall
    arg, value = guess_arguments(current_inst, syscall=True)
    if arg == 0:
        address = current_address
        for i in range(MAX_FIND_INSTS):
            inst = seg.getInstructionAtAddress(address)
            opcode = inst.getInstructionString()
            if (opcode == 'int' and inst.getRawArgument(1) == '0x80'):
                comment = syscall32[value]
                seg.setInlineCommentAtAddress(current_address, comment)
                break
            elif opcode == 'syscall':
                comment = syscall64[value]
                seg.setInlineCommentAtAddress(current_address, comment)
                break
            address += inst.getInstructionLength()
