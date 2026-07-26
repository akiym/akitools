package htmlmd

import (
	"errors"
	"fmt"
	"io"
	nurl "net/url"
	"os"
	"path/filepath"
	"strings"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/spf13/cobra"
)

var (
	flagWrite bool
	flagURL   string
)

var Cmd = &cobra.Command{
	Use:   "htmlmd [file...]",
	Short: "Extract readable content from HTML and convert it to Markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
}

func init() {
	Cmd.Flags().BoolVarP(&flagWrite, "write", "w", false, "write result to <file>.md instead of stdout")
	Cmd.Flags().StringVarP(&flagURL, "url", "u", "", "base URL for resolving relative links")
}

func run(args []string) error {
	var pageURL *nurl.URL
	if flagURL != "" {
		u, err := nurl.Parse(flagURL)
		if err != nil {
			return fmt.Errorf("invalid URL %q: %w", flagURL, err)
		}
		pageURL = u
	}

	if len(args) == 0 {
		if flagWrite {
			return errors.New("-w requires input files")
		}
		md, err := convert(os.Stdin, pageURL)
		if err != nil {
			return err
		}
		fmt.Println(md)
		return nil
	}

	for i, name := range args {
		md, err := convertFile(name, pageURL)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if flagWrite {
			out := strings.TrimSuffix(name, filepath.Ext(name)) + ".md"
			if out == name {
				return fmt.Errorf("%s: output file would overwrite input", name)
			}
			if err := os.WriteFile(out, []byte(md+"\n"), 0o644); err != nil {
				return err
			}
		} else {
			if i > 0 {
				fmt.Println()
			}
			fmt.Println(md)
		}
	}
	return nil
}

func convertFile(name string, pageURL *nurl.URL) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return convert(f, pageURL)
}

func convert(r io.Reader, pageURL *nurl.URL) (string, error) {
	article, err := readability.FromReader(r, pageURL)
	if err != nil {
		return "", err
	}
	if article.Node == nil {
		return "", errors.New("no readable content found")
	}

	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			table.NewTablePlugin(),
			strikethrough.NewStrikethroughPlugin(),
		),
	)
	md, err := conv.ConvertNode(article.Node)
	if err != nil {
		return "", err
	}

	out := strings.TrimSpace(string(md))
	if title := article.Title(); title != "" {
		out = "# " + title + "\n\n" + out
	}
	return out, nil
}
