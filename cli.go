// Copyright (C) 2018 Michael Picht
//
// This file is part of gool.
//
// gool is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// gool is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with gool. If not, see <http://www.gnu.org/licenses/>.

package main

// cli.go implementes the command line interface (i.e. the root command
// and sub commands for gool. This is done with the help of a fork of package
// cobra (https://github.com/spf13/cobra). The only reason for forking cobra
// instead of using the original package was to have a German translation of
// the help texts.

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

var preamble = `gool (Go - Online TV Recorder on Linux) ` + Version + `
Copyright (C) 2018 Michael Picht <https://github.com/mipimipi/gool>
`

var helpTemplate = preamble + `
{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

// root command 'gool'
var rootCmd = &cobra.Command{
	Use:     "gool",
	Version: Version,
}

// sub command 'list'
var cmdLst = &cobra.Command{
	Use:   `list [files]`,
	Short: `List videos`,
	Long:  `List videos, incl. status ("ENC": encoded, "DEC": decoded but uncut, "CUT: cut). In addition, it's shown whether cutlists exist or not (column "CL"). Videos will not be processed.`,
	DisableFlagsInUseLine: true,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// retrieve flags
		cmd.ParseFlags(args)
		// print copyright etc. on command line
		fmt.Printf(preamble)
		// Read configuration and ...
		if err := cfg.getFromFile(); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		// ... set the number of processes to be used by gool
		_ = runtime.GOMAXPROCS(cfg.numCpus)
		// create video list
		vl := make(videoList)
		// read videos
		if err := vl.read(args); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		// print list of videos
		vl.print()
	},
}

// sub command 'process'
var cmdPrc = &cobra.Command{
	Use:   `process [files]`,
	Short: `process videos`,
	Long:  `process videos (i.e. videos are decoded and cut - depending on its status. As prerequisite for cutting videos, cutlists will be loaded). Finally, a summary is displayed.`,
	DisableFlagsInUseLine: true,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// retrieve flags
		cmd.ParseFlags(args)
		// print copyright etc. on command line
		fmt.Printf(preamble)
		// Read configuration and ...
		if err := cfg.getFromFile(); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		// ... set the number of processes to be used by gool
		_ = runtime.GOMAXPROCS(cfg.numCpus)
		// create video list
		vl := make(videoList)
		// read videos
		if err := vl.read(args); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		// process videos
		vl.process()
		// print list of videos
		vl.print()
	},
}

// logFile stores parameter of logging flag
var logFile string

func init() {
	// set custom help template
	rootCmd.SetHelpTemplate(helpTemplate)
	cmdLst.SetHelpTemplate(helpTemplate)
	cmdPrc.SetHelpTemplate(helpTemplate)

	// build up command structure: 'list' and 'process' are sub commands of 'gool')
	rootCmd.AddCommand(cmdLst, cmdPrc)

	// define flag for logging
	cmdLst.Flags().StringVarP(&logFile, "log", "l", "", "Switch on logging and set log file name")
	cmdPrc.Flags().StringVarP(&logFile, "log", "l", "", "Switch on logging and set log file name")
}

// Execute executes the root command
func execute() error {
	return rootCmd.Execute()
}
