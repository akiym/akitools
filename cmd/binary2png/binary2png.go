package binary2png

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "binary2png",
	Short: "Convert binary to png",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
}

func run(args []string) error {
	var (
		outfile = flag.String("outfile", "out.png", "")
		width   = flag.Int("width", 128, "")
		bcolor  = flag.Bool("color", false, "")
	)
	err := flag.CommandLine.Parse(args)
	if err != nil {
		return err
	}

	if flag.NArg() != 1 {
		_, _ = fmt.Fprintf(os.Stderr,
			"Usage: %s [--width=%%d] [--outfile=%%s] filename\n", os.Args[0])
		os.Exit(1)
	}
	filename := flag.Args()[0]

	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	buf, err := io.ReadAll(file)
	if err != nil {
		log.Fatal(err)
	}

	m := image.NewRGBA(image.Rect(0, 0, *width, (len(buf)-1) / *width + 1))
	for i, c := range buf {
		var bitColor color.RGBA
		if *bcolor {
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
		m.Set(i%*width, int(i / *width), bitColor)
	}

	img, err := os.Create(*outfile)
	if err != nil {
		log.Fatal(err)
	}
	defer img.Close()
	_ = png.Encode(img, m)

	return nil
}
