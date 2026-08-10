package w3m

import (
	"bufio"
	"html"
	"io"
	"os"
	"os/exec"
)

// Original code:
// https://shinh.hatenablog.com/entry/20070429/1177827792

func W3mWrapEach(cmd string, args []string, lineMax int, handler func(line string) string) error {
	return w3m(func(w io.WriteCloser) error {
		defer w.Close()
		return wrapEach(cmd, args, lineMax, func(line *string) {
			if line != nil {
				_, _ = w.Write([]byte(handler(html.EscapeString(*line)) + "<br>\n"))
			} else {
				_, _ = w.Write([]byte("<hr>too many lines\n"))
			}
		})
	})
}

func w3m(handler func(w io.WriteCloser) error) error {
	cmd := exec.Command("w3m", "-T", "text/html")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()

	if err := cmd.Start(); err != nil {
		return err
	}
	if err := handler(stdin); err != nil {
		return err
	}
	return cmd.Wait()
}

func wrapEach(cmd string, args []string, lineMax int, handler func(line *string)) error {
	proc := exec.Command(cmd, args...)
	proc.Stderr = os.Stderr
	stdout, err := proc.StdoutPipe()
	if err != nil {
		return err
	}
	defer stdout.Close()

	if err := proc.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	// grep hits can exceed bufio.Scanner's 64KB default, which would
	// otherwise truncate the output silently.
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	killed := false
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		handler(&line)
		lineCount++

		if lineMax > 0 && lineCount >= lineMax {
			_ = proc.Process.Kill()
			killed = true
			handler(nil)
			break
		}
	}
	if err := scanner.Err(); err != nil && !killed {
		return err
	}

	// Reaping a process we killed on purpose always reports a signal; that
	// is the documented lineMax behaviour, not a failure.
	if err := proc.Wait(); err != nil && !killed {
		return err
	}
	return nil
}
