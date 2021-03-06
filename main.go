package main

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/codem8s/2fy/version"
	"github.com/ghodss/yaml"
	"github.com/urfave/cli"
	"io"
	"io/ioutil"
	"k8s.io/client-go/util/jsonpath"
	"os"
	"encoding/json"
	"reflect"
)

var (
	inputPath  string
	outputPath string
	jsonpathTemplate string
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
				cli.StringFlag{
					Name:        "jsonpath, jp",
					Usage:       "the optional JSONPath template to parse the input with",
					Destination: &jsonpathTemplate,
				},
			},
			Action: func(c *cli.Context) error {
				return transform(
					func(input []byte) (interface{}, error) {
						var object interface{}
						if err := yaml.Unmarshal(input, &object); err != nil {
							return nil, err
						}
						return object, nil
					},
					func(object interface{}) ([]byte, error) {
						output := []byte(fmt.Sprintf("%v", object))
						return output, nil
					})
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
				cli.StringFlag{
					Name:        "jsonpath, jp",
					Usage:       "the optional JSONPath template to parse the input with",
					Destination: &jsonpathTemplate,
				},
			},
			Action: func(c *cli.Context) error {
				return transform(
					func(input []byte) (interface{}, error) {
						var object interface{}
						if err := yaml.Unmarshal(input, &object); err != nil {
							return nil, err
						}
						return object, nil
					},
					func(object interface{}) ([]byte, error) {
						output, err := json.Marshal(object)
						if err != nil {
							return nil, err
						}
						return output, nil
					})
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
		cli.OsExiter(1)
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

func collectResults(cr []interface{}, results []reflect.Value) []interface{} {
	for _, r := range results {
		cr = append(cr, r.Interface())
	}
	return cr
}

func filter(object interface{}, jsonpathTemplate string) (interface{}, error) {
	if jsonpathTemplate != "" {
		jp := jsonpath.New("out")
		if err := jp.Parse(jsonpathTemplate); err != nil {
			return nil, err
		}
		logrus.Debugf("JSON Path template: '%v'", jsonpathTemplate)

		fullResults, err1 := jp.FindResults(object)
		if err1 != nil {
			logrus.Debugf(
					"Error executing template: %v. Printing more information for debugging the template:\n" +
						"\ttemplate was:\n\t\t%v\n" +
						"\tobject given to jsonpath engine was:\n\t\t%#v\n\n", err1, jsonpathTemplate, object)
			return nil, fmt.Errorf("error executing jsonpath %q: %v", jsonpathTemplate, err1)
		}

		var rs []interface{}
		for ix := range fullResults {
			rs = collectResults(rs, fullResults[ix])
		}
		if len(rs) == 0 {
			return nil, nil
		} else if len(rs) == 1 {
			return rs[0], nil
		} else {
			return rs, nil
		}
	} else {
		logrus.Debug("No results found for the JSON Path")
		return object, nil
	}
}

type unmarshaller func([]byte) (interface{}, error)
type marshaller func(interface{}) ([]byte, error)

func transform(unmarshal unmarshaller, marshal marshaller) error {
	inputContent, err := readInput()
	if err != nil {
		return err
	}

	logrus.Debug("Unmarshal to an object")
	object, err1 := unmarshal(inputContent)
	if err1 != nil {
		return err1
	}
	if object == nil {
		return writeOutput([]byte{})
	}

	resultObject, err2 := filter(object, jsonpathTemplate)
	if err2 != nil {
		return err2
	}

	if resultObject == nil {
		logrus.Debug("No results found for the JSON Path")
		return writeOutput([]byte{})
	}

	logrus.Debug("Marshal to an object")
	outputContent, err3 := marshal(resultObject)
	if err3 != nil {
		return err3
	}
	logrus.Debugf("Output: %v", string(outputContent))
	return writeOutput(outputContent)
}