package shellcode

import (
	"bufio"
	_ "embed"
	"fmt"
	"io"
	"os"
	"syscall"
	"unsafe"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "shellcode",
	Short: "",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

const size = 4096

func run() error {
	r := bufio.NewReader(os.Stdin)
	src, err := io.ReadAll(io.LimitReader(r, size))
	if err != nil {
		return err
	}

	addr, _, errno := syscall.Syscall6(
		syscall.SYS_MMAP,
		0x20000000,
		uintptr(size),
		syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC,
		syscall.MAP_PRIVATE|syscall.MAP_ANON,
		uintptr(^uint(0)),
		0,
	)
	if errno != 0 {
		return fmt.Errorf("mmap failed: %v", errno)
	}

	shellcode := unsafe.Pointer(addr)
	copy((*[size]byte)(shellcode)[:], src)
	p := unsafe.Pointer(&shellcode)
	(*(*func())(unsafe.Pointer(&p)))()

	return nil
}
