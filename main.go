package main

import (
	"bytes"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/codem8s/2fy/version"
	"github.com/ghodss/yaml"
	"github.com/urfave/cli"
	"io"
	"io/ioutil"
	"k8s.io/client-go/util/jsonpath"
	"os"
)

var (
	inputPath  string
	outputPath string
)

// preload initializes any global options and configuration
// before the main or sub commands are run.
func preload(c *cli.Context) (err error) {
	if c.GlobalBool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "2fy"
	app.Version = fmt.Sprintf("version %s, build %s", version.VERSION, version.GITCOMMIT)
	app.Author = "codem8s"
	app.Email = "no-reply@codemat.es"
	app.Usage = "convert all the things!"
	app.Before = preload
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug, d",
			Usage: "run in debug mode",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:    "yaml2txt",
			Aliases: []string{"y2t"},
			Usage:   "conver YAML to a text representation",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "input, in",
					Usage:       "the input file (or stdin otherwise)",
					Destination: &inputPath,
				},
				cli.StringFlag{
					Name:        "output, out",
					Usage:       "the output file (or stdout otherwise)",
					Destination: &outputPath,
				},
			},
			Action: func(c *cli.Context) error {
				fileContent, err := readInput()
				if err != nil {
					return err
				}

				logrus.Debug("unmarshalling")
				var contentStructure map[string]interface{}
				err = yaml.Unmarshal(fileContent, &contentStructure)
				if err != nil {
					return err
				}

				outputContent := []byte(fmt.Sprintf("%v\n", contentStructure))
				return writeOutput(outputContent)
			},
		},
		{
			Name:    "yaml2json",
			Aliases: []string{"y2j"},
			Usage:   "conver YAML to JSON",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "input, in",
					Usage:       "the input file (or stdin otherwise)",
					Destination: &inputPath,
				},
				cli.StringFlag{
					Name:        "output, out",
					Usage:       "the output file (or stdout otherwise)",
					Destination: &outputPath,
				},
			},
			Action: func(c *cli.Context) error {
				fileContent, err := readInput()
				if err != nil {
					return err
				}

				logrus.Debug("Unmarshal to an object")
				var object interface{}
				if err := yaml.Unmarshal(fileContent, &object); err != nil {
					return err
				}
				if object == nil {
					return writeOutput([]byte{})
				}

				tmpl := "{.foo}"
				j := jsonpath.New("out")
				if err := j.Parse(tmpl); err != nil {
					return err
				}
				logrus.Debugf("JSON Path template: %v", tmpl)

				buffer := bytes.NewBuffer(nil)
				if err := j.Execute(buffer, object); err != nil {
					logrus.Debugf(
						"Error executing template: %v. Printing more information for debugging the template:\n" +
					    "\ttemplate was:\n\t\t%v\n" +
					    "\tobject given to jsonpath engine was:\n\t\t%#v\n\n", err, tmpl, object)
					return fmt.Errorf("error executing jsonpath %q: %v", tmpl, err)
				}

				var jsonContent []byte
				jsonContent = buffer.Bytes()
				logrus.Debugf("JSON: %v", string(jsonContent))

				return writeOutput(jsonContent)
			},
		},
	}

	app.CommandNotFound = func(c *cli.Context, command string) {
		fmt.Fprintf(cli.ErrWriter, "There is no %q command.\n", command)
		cli.OsExiter(1)
	}
	app.OnUsageError = func(c *cli.Context, err error, isSubcommand bool) error {
		if isSubcommand {
			return err
		}

		fmt.Fprintf(cli.ErrWriter, "WRONG: %v\n", err)
		return nil
	}
	cli.OsExiter = func(c int) {
		if c != 0 {
			logrus.Debugf("exiting with %d", c)
		}
		os.Exit(c)
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(cli.ErrWriter, "ERROR: %v\n", err)
	}
}

func readInput() ([]byte, error) {
	var inputFile *os.File
	if inputPath == "" {
		stdinFileInfo, _ := os.Stdin.Stat()
		if (stdinFileInfo.Mode() & os.ModeNamedPipe) != 0 {
			logrus.Debug("no input path, using piped stdin")
			inputFile = os.Stdin
		} else {
			return nil, cli.NewExitError("Expected a pipe stdin", 1)
		}
	} else {
		logrus.Debugf("input path: %v", inputPath)
		f, err := os.Open(inputPath)
		if err != nil {
			logrus.Debug("cannot open file")
			return nil, err
		}
		defer f.Close()
		inputFile = f
	}
	fileContent, err := ioutil.ReadAll(inputFile)
	if err != nil {
		logrus.Debug("cannot read file")
		return nil, err
	}
	return fileContent, nil
}

func writeOutput(outputContent []byte) error {
	if outputPath == "" {
		logrus.Debug("no output path, writing to stdout")
		count, err := os.Stdout.Write(outputContent)
		if err == nil && count < len(outputContent) {
			logrus.Debugf("wrote only %v/%v bytes", count, len(outputContent))
			return io.ErrShortWrite
		}
		if err != nil {
			logrus.Debug("error writing to file")
			return err
		}
	} else {
		logrus.Debugf("writing to file: %v", outputPath)
		err := ioutil.WriteFile(outputPath, outputContent, 0644)
		if err != nil {
			logrus.Debug("error writing to file")
			return err
		}
	}
	return nil
}
