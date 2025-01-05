package tobin

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "tobin",
	Short:   "Convert hex to binary",
	Example: "echo -n 414243 | akitools tobin",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

func run() error {
	r := bufio.NewReader(os.Stdin)
	src, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	src = bytes.Trim(src, "\t\n\r \"'")

	if bytes.Contains(src, []byte("\\")) {
		dst, err := DecodeEscapeSequence(src)
		if err != nil {
			return err
		}
		fmt.Printf("%s", dst)
	} else {
		if len(src)%2 != 0 {
			src = append([]byte{'0'}, src...)
		}
		dst := make([]byte, hex.DecodedLen(len(src)))
		n, err := hex.Decode(dst, src)
		if err != nil {
			return err
		}
		fmt.Printf("%s", dst[:n])
	}

	return nil
}
