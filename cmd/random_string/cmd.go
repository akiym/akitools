package random_string

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "random_string <length>",
	Short: "",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
}

var ascii bool
var asciiLc bool
var lessAscii bool
var hirakata bool

func init() {
	Cmd.Flags().BoolVarP(&ascii, "ascii", "a", false, "ASCII")
	Cmd.Flags().BoolVarP(&asciiLc, "asciilc", "A", false, "ASCII lowercase")
	Cmd.Flags().BoolVarP(&lessAscii, "lessascii", "l", false, "Less ASCII")
	Cmd.Flags().BoolVarP(&hirakata, "hirakata", "H", false, "Hiragana and katakana")
}

func run(args []string) error {
	length := 8
	if len(args) > 0 {
		var err error
		length, err = strconv.Atoi(args[0])
		if err != nil {
			return err
		}
		if length < 1 {
			return fmt.Errorf("length must be greater than 0")
		}
	}

	chars := make([]rune, 0)
	if ascii {
		chars = append(chars, runeRange('A', 'Z')...)
		chars = append(chars, runeRange('a', 'z')...)
		chars = append(chars, runeRange('0', '9')...)
	} else if asciiLc {
		chars = append(chars, runeRange('a', 'z')...)
		chars = append(chars, runeRange('0', '9')...)
	} else if lessAscii {
		chars = append(chars, runeRange('A', 'Z')...)
		chars = append(chars, runeRange('a', 'z')...)
		chars = append(chars, runeRange('0', '9')...)
		chars = append(chars, []rune("!@^&*-_()[]{}<>,./")...)
	} else if hirakata {
		chars = append(chars, runeRange('あ', 'ん')...)
		chars = append(chars, runeRange('ア', 'ン')...)
	} else {
		chars = runeRange(0x20, 0x7e)
	}

	randMax := big.NewInt(int64(len(chars)))
	ret := make([]rune, length)
	for i := 0; i < length; i++ {
		randInt, err := rand.Int(rand.Reader, randMax)
		if err != nil {
			return err
		}
		ret[i] = chars[randInt.Int64()]
	}
	fmt.Println(string(ret))

	return nil
}

func runeRange(start, end rune) []rune {
	runes := make([]rune, 0)
	for i := start; i <= end; i++ {
		runes = append(runes, i)
	}
	return runes
}
