package main

import (
	"fmt"
	"github.com/modfin/go18exp/slicez"
	"github.com/urfave/cli/v2"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const ext = ".gonzo.txt"
const timeformat = "2006-01-02_15:04_Monday" + ext

func main() {

	commands, content, hasContent := slicez.Cut(os.Args, "--")

	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:  "new",
				Usage: "create a new note ",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "editor",
						Aliases: []string{"e"},
						Usage:   "use your default editor to manage notes",
					},
				},
				Action: func(c *cli.Context) error {
					note, err := loadNewNote(c.Bool("editor"), content, hasContent)
					if err != nil {
						return err
					}
					at := time.Now()
					filename := at.In(time.Local).Format(timeformat)
					return saveNote(filename, note)
				},
			},
			{
				Name:  "delete",
				Usage: "create a new note ",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "editor",
						Aliases: []string{"e"},
						Usage:   "use your default editor to manage notes",
					},
				},
				Action: func(c *cli.Context) error {
					toDelete := c.Args().Slice()
					var err error
					var dir = getStorageDir()
					slicez.Each(toDelete, func(file string) {
						err1 := os.Remove(filepath.Join(dir, file+ext))
						if err1 != nil {
							err = err1
						}
					})

					return err
				},
			},

			{
				Name:  "read",
				Usage: "read a note ",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "reverse",
						Aliases: []string{"r"},
						Usage:   "Read in reverse chronological order",
					},
					&cli.BoolFlag{
						Name:    "cat",
						Aliases: []string{"c"},
						Usage:   "cat to std out",
					},
				},
				Action: func(c *cli.Context) error {

					var err error
					var toRead []string

					toRead, err = listNotes()
					if err != nil {
						return err
					}

					reverse := c.Bool("reverse")

					from := c.Args().First()
					filter := func(name string) bool {
						return true
					}

					if from != "" {
						filter = func(name string) bool {
							return name >= from
						}
						if reverse {
							filter = func(name string) bool {
								return name <= from
							}
						}
					}

					if reverse {
						toRead = slicez.Reverse(toRead)
					}
					toRead = slicez.Filter(toRead, filter)

					var cmd *exec.Cmd
					pager := getPager()
					var out io.WriteCloser = os.Stdout
					if !c.Bool("cat") && pager != "" {

						cmd = exec.Command(pager)
						cmd.Stdout = os.Stdout
						cmd.Stderr = os.Stderr
						out, err = cmd.StdinPipe()
						if err != nil {
							return err
						}
						err = cmd.Start()
						if err != nil {
							return err
						}
					}

					for _, note := range toRead {
						content, err := readNote(note)
						if err != nil {
							return err
						}
						_, err = out.Write([]byte(fmt.Sprintf("╭──────────────────────%s╮\n", strings.Repeat("─", len(note)+2))))
						_, err = out.Write([]byte(fmt.Sprintf("├──────────  %s  ──────────┤\n\n", note)))
						if err != nil {
							return err
						}
						_, err = out.Write([]byte(content))
						_, err = out.Write([]byte("\n\n"))
					}
					err = out.Close()
					if err != nil {
						return err
					}

					if cmd != nil {
						return cmd.Wait()
					}
					return nil
				},
			},

			{
				Name:  "edit",
				Usage: "edit a note ",
				Action: func(c *cli.Context) error {
					toEdit := c.Args().First()

					note, err := readNote(toEdit)
					if err != nil {
						return err
					}
					replacement, err := loadNewNote(true, []string{note}, true)
					if err != nil {
						return err
					}
					filename := toEdit + ext
					return saveNote(filename, replacement)
				},
			},
			{
				Name:  "list",
				Usage: "create a new note ",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "tail",
						Value: 20,
						Usage: "tail x elements",
					},
					&cli.IntFlag{
						Name:  "head",
						Value: 20,
						Usage: "head x elements",
					},
				},
				Action: func(c *cli.Context) error {

					notes, err := listNotes()

					if c.IsSet("head") {
						head := c.Int("head")
						if head > len(notes) {
							head = len(notes)
						}
						notes = notes[:head]
					}

					if c.IsSet("tail") {
						tail := c.Int("tail")
						if tail > len(notes) {
							tail = len(notes)
						}
						notes = notes[len(notes)-tail:]
					}

					slicez.Each(notes, func(n string) {
						fmt.Println(n)
					})
					return err
				},
			},
		},
	}

	err := app.Run(commands)
	if err != nil {
		log.Fatal(err)
	}

}

func loadNewNote(editor bool, content []string, hasContent bool) (string, error) {

	var note string
	if hasContent {
		note = strings.Join(content, " ")
	}

	if note == "-" {
		buff, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		note = string(buff)
		return note, nil
	}

	if editor {
		file, err := os.CreateTemp("", "gono")
		defer func() {
			err2 := os.Remove(file.Name())
			if err2 != nil {
				fmt.Println("error removing tmp file,", err2)
			}
		}()
		if err != nil {
			return "", err
		}
		_, err = file.WriteString(note)
		if err != nil {
			return "", err
		}

		cmd := exec.Command(getEditor(), file.Name())
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return "", err
		}

		b, err := ioutil.ReadFile(file.Name())
		if err != nil {
			return "", err
		}

		note = string(b)
	}
	return note, nil
}

func readNote(name string) (string, error) {
	dir := getStorageDir()
	filename := name + ext
	filename = filepath.Join(dir, filename)
	note, err := ioutil.ReadFile(filename)
	return string(note), err

}

func saveNote(name, note string) error {
	dir := getStorageDir()
	err := os.MkdirAll(dir, 0775)
	if err != nil {
		return err
	}
	filename := filepath.Join(dir, name)
	fmt.Println("saving to", filename)
	return ioutil.WriteFile(filename, []byte(note), 0644)
}

func listNotes() ([]string, error) {
	dir := getStorageDir()

	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if strings.HasSuffix(path, ext) {
			_, file := filepath.Split(path)
			file = strings.TrimSuffix(file, ext)
			files = append(files, file)
		}
		return nil
	})
	slicez.Sort(files)

	return files, err
}

func getStorageDir() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(dir, ".gonzo")
}

func getEditor() string {
	// Todo respect  ~/.selected_editor
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		return "vi"
	}
	return editor
}
func getPager() string {
	return os.Getenv("PAGER")
}
