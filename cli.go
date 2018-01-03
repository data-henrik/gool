// Copyright (C) 2017 Michael Picht
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

	"github.com/mipimipi/cobra"
)

// root command 'gool'
var rootCmd = &cobra.Command{
	Use:     "gool",
	Version: Version,
}

// sub command 'list'
var cmdLst = &cobra.Command{
	Use:   `list [Dateien]`,
	Short: `Liste Videos`,
	Long:  `Listet Videos, inkl. ihres Status ("ENC": verschlüsselt, "DEC": entschlüsselt, "CUT: geschnittet). Außerdem wird angezeigt, ob Schneidelisten (Cutlists: Spalte "CL") existieren. Die Videos werden nicht bearbeitet.`,
	DisableFlagsInUseLine: true,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
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
	Use:   `process [Dateien]`,
	Short: `Bearbeite Videos`,
	Long:  `Bearbeitet Videos (d.h. Videos werden - je nach Status - entschlüsselt und geschnitten. Ferner werden - als Voraussetzung damit Videos geschnitten werden können - Schneidelisten beschafft). Am Ende wird eine Zusammenfassung der Bearbeitung angezeigt.`,
	DisableFlagsInUseLine: true,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
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

func init() {
	// build up command structure: 'list' and 'process' are sub commands of 'gool')
	rootCmd.AddCommand(cmdLst, cmdPrc)
}

// Execute executes the root command
func execute() error {
	return rootCmd.Execute()
}
