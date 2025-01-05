package tohex

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "tohex",
	Short:   "Convert binary to hex",
	Example: "echo -n ABC | akitools tohex",
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

	dst := make([]byte, hex.EncodedLen(len(src)))
	n := hex.Encode(dst, src)
	fmt.Printf("%s", dst[:n])

	return nil
}
