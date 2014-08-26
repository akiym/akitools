import gdb

class SocatCommand(gdb.Command):
    "Listen with socat ^..^"

    def __init__(self):
        super(SocatCommand, self).__init__("socat",
                                           gdb.COMMAND_SUPPORT,
                                           gdb.COMPLETE_FILENAME)

    # taken from gdb-peda -----

    def execute_redirect(self, gdb_command, silent=False):
        """
        Execute a gdb command and capture its output

        Args:
            - gdb_command (String)
            - silent: discard command's output, redirect to /dev/null (Bool)

        Returns:
            - output of command (String)
        """
        result = None
        #init redirection
        if silent:
            logfd = open(os.path.devnull, "rw")
        else:
            logfd = tmpfile()
        logname = logfd.name
        gdb.execute('set logging off') # prevent nested call
        gdb.execute('set height 0') # disable paging
        gdb.execute('set logging file %s' % logname)
        gdb.execute('set logging overwrite on')
        gdb.execute('set logging redirect on')
        gdb.execute('set logging on')
        try:
            gdb.execute(gdb_command)
            gdb.flush()
            gdb.execute('set logging off')
            if not silent:
                logfd.flush()
                result = logfd.read()
            logfd.close()
        except Exception as e:
            gdb.execute('set logging off') #to be sure
            if config.Option.get("debug") == "on":
                msg('Exception (%s): %s' % (gdb_command, e), "red")
                traceback.print_exc()
            logfd.close()
        if config.Option.get("verbose") == "on":
            msg(result)
        return result

    def getfile(self):
        """
        Get exec file of debugged program

        Returns:
            - full path to executable file (String)
        """
        result = None
        out = self.execute_redirect('info files')
        if out and '"' in out:
            p = re.compile(".*exec file:\s*`(.*)'")
            m = p.search(out)
            if m:
                result = m.group(1)
            else: # stripped file, get symbol file
                p = re.compile("Symbols from \"([^\"]*)")
                m = p.search(out)
                if m:
                    result = m.group(1)

        return result

    # -----

    def invoke(self, arg, from_tty):
        if arg:
            port = int(arg)
        else:
            port = 4000
        filename = self.getfile()
        gdb.execute('printf "socat: listening on :%d\n"' % port)
        gdb.execute('exec-file socat')
        gdb.execute('run tcp-l:%d,reuseaddr exec:%s' % (port, filename))

        gdb.execute('exec-file %s' % filename)

SocatCommand()
