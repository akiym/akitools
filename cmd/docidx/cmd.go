package docidx

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	flagDB       string
	flagLimit    int
	flagAliases  string
	flagExcludes []string
	flagByPath   bool
)

var Cmd = &cobra.Command{
	Use:   "docidx <command>",
	Short: "Build and search a local documentation index (SQLite FTS5/BM25)",
	Long: `docidx indexes library documentation (Markdown/HTML/reStructuredText)
into SQLite FTS5 by splitting files on their heading structure (H1-H3), then
serves fast BM25-ranked search over title/headings/breadcrumbs/body.

Typical flow:
  docidx build docs/          # generate index.db
  docidx search "spawn enemy" # list candidates (id, score, kind, path, title)
  docidx cat 42               # print the chunk body

Query terms are expanded via an optional aliases.json next to the index
(e.g. {"spawn": ["instantiate", "PackedScene"]}).`,
}

var buildCmd = &cobra.Command{
	Use:   "build <docs-dir>",
	Short: "Index Markdown/HTML/reST files under a directory into index.db",
	Long: `Index Markdown/HTML/reStructuredText files under a directory into index.db.

--exclude accepts gitignore syntax patterns, matched against paths relative
to <docs-dir> ("/foo.html" anchors to the root, "foo/" matches directories
at any depth, "*_source.html" matches files at any depth, "!" re-includes).
Doxygen output is best indexed with:
  docidx build html/ --exclude '*_source.html' --exclude '*-members.html'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		files, chunks, err := buildIndex(flagDB, args[0], flagExcludes)
		if err != nil {
			return err
		}
		fmt.Printf("indexed %d files (%d chunks) into %s\n", files, chunks, flagDB)
		return nil
	},
}

var searchCmd = &cobra.Command{
	Use:   "search <query>...",
	Short: "Search the index; prints id, score, kind, body bytes, path#anchor, title (TSV)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		aliasPath := flagAliases
		required := aliasPath != ""
		if aliasPath == "" {
			aliasPath = filepath.Join(filepath.Dir(flagDB), "aliases.json")
		}
		dict, err := loadAliases(aliasPath, required)
		if err != nil {
			return err
		}

		db, err := openIndex(flagDB)
		if err != nil {
			return err
		}
		defer db.Close()

		results, err := searchIndex(db, strings.Join(args, " "), dict, flagLimit)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			fmt.Println("no results")
			return nil
		}
		fallbackMarked := false
		for _, r := range results {
			if r.Fallback && !fallbackMarked {
				fmt.Println("# or-fallback: rows below match only some query terms")
				fallbackMarked = true
			}
			loc := r.Path
			if r.Anchor != "" {
				loc += "#" + r.Anchor
			}
			fmt.Printf("%d\t%.2f\t%s\t%d\t%s\t%s\n", r.ID, -r.Score, r.Kind, r.BodyLen, loc, r.Title)
		}
		return nil
	},
}

var catCmd = &cobra.Command{
	Use:   "cat <id>... | cat --path <path>...",
	Short: "Print chunk bodies by id, or whole pages by path with --path",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openIndex(flagDB)
		if err != nil {
			return err
		}
		defer db.Close()

		for i, arg := range args {
			if i > 0 {
				fmt.Println("---")
			}
			if flagByPath {
				if err := catPage(db, arg); err != nil {
					return err
				}
				continue
			}
			id, err := strconv.ParseInt(arg, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid id %q", arg)
			}
			c, err := getChunk(db, id)
			if err != nil {
				return err
			}
			loc := c.Path
			if c.Anchor != "" {
				loc += "#" + c.Anchor
			}
			fmt.Printf("id: %d\npath: %s\nbreadcrumbs: %s\nkind: %s\n\n%s\n", id, loc, c.Breadcrumbs, c.Kind, c.Body)
		}
		return nil
	},
}

func catPage(db *sql.DB, path string) error {
	chunks, err := getPageChunks(db, path)
	if err != nil {
		return err
	}
	fmt.Printf("path: %s\n", chunks[0].Path)
	for _, c := range chunks {
		fmt.Printf("\n%s\n", c.Body)
	}
	return nil
}

func init() {
	Cmd.PersistentFlags().StringVar(&flagDB, "db", "index.db", "path to the index database")
	buildCmd.Flags().StringArrayVar(&flagExcludes, "exclude", nil, "gitignore syntax pattern to skip (repeatable)")
	searchCmd.Flags().IntVar(&flagLimit, "limit", 30, "maximum number of results")
	searchCmd.Flags().StringVar(&flagAliases, "aliases", "", "path to aliases.json (default: next to --db)")
	catCmd.Flags().BoolVar(&flagByPath, "path", false, "treat arguments as document paths and print all chunks of each page in order")
	Cmd.AddCommand(buildCmd, searchCmd, catCmd)
}
