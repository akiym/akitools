package binary2png

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var (
	outfile string
	width   int
	bcolor  bool
)

var Cmd = &cobra.Command{
	Use:   "binary2png <filename>",
	Short: "Convert binary to png",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
}

func init() {
	Cmd.Flags().StringVar(&outfile, "outfile", "out.png", "output png path")
	Cmd.Flags().IntVar(&width, "width", 128, "image width in pixels")
	Cmd.Flags().BoolVar(&bcolor, "color", false, "use the extended color palette")
}

func run(args []string) error {
	if width < 1 {
		return errors.New("--width must be greater than 0")
	}
	filename := args[0]

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	buf, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	if len(buf) == 0 {
		return fmt.Errorf("%s is empty", filename)
	}

	m := image.NewRGBA(image.Rect(0, 0, width, (len(buf)-1)/width+1))
	for i, c := range buf {
		var bitColor color.RGBA
		if bcolor {
			if c == 0x00 {
				bitColor = color.RGBA{R: 255, G: 255, B: 255, A: 255}
			} else if 0x01 <= c && c <= 0x1f {
				bitColor = color.RGBA{G: 255, B: 255, A: 255}
			} else if 0x20 <= c && c <= 0x7f {
				bitColor = color.RGBA{R: 255, A: 255}
			} else if 0x80 <= c && c <= 0x9f {
				bitColor = color.RGBA{R: 255, G: 255, A: 255}
			} else if 0xa0 <= c && c <= 0xfe {
				bitColor = color.RGBA{R: 255, B: 255, A: 255}
			} else if c == 0xff {
				bitColor = color.RGBA{A: 255}
			}
		} else {
			if c == 0x00 {
				bitColor = color.RGBA{R: 255, G: 255, B: 255, A: 255}
			} else if 0x01 <= c && c <= 0x1f {
				bitColor = color.RGBA{G: 255, B: 255, A: 255}
			} else if 0x20 <= c && c <= 0x7f {
				bitColor = color.RGBA{R: 255, A: 255}
			} else if 0x80 <= c {
				bitColor = color.RGBA{A: 255}
			}
		}
		m.Set(i%width, i/width, bitColor)
	}

	img, err := os.Create(outfile)
	if err != nil {
		return err
	}
	defer img.Close()
	if err := png.Encode(img, m); err != nil {
		return err
	}
	return img.Close()
}
