// Copyright (C) 2017 Michael Picht
//
// This file is part of gool (Online TV Recorder on Linux in Go).
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
// along with gool.  If not, see <http://www.gnu.org/licenses/>.

package main

// decode.go implements the call of command line tools (currently,
// that's only otrdecoder) to decode .otrkey files.

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/romana/rlog"
)

// callOTRDecoder calls otrdecoder and handles the command line output.
func callOTRDecoder(filePath string, key string) error {
	var (
		err         error
		errStr      string
		otrFilePath string
		prg         int
		prgSet      int
	)

	// Create filepath to call otr decoder: If no directory path has been configured ...
	if cfg.otrDecDirPath == "" {
		// set the filepath to the program file name ...
		otrFilePath = otrDecoderName
	} else {
		// else: build the filepath from the directory path and the program file name
		otrFilePath = cfg.otrDecDirPath + "/" + otrDecoderName
	}
	// Create shell command for decoding
	cmd := exec.Command(otrFilePath,
		"-e", cfg.otrUsername,
		"-p", cfg.otrPassword,
		"-i", filePath,
		"-o", cfg.decDirPath)
	// Set up output pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		rlog.Error("Cannot establish pipe for stdout: %v" + err.Error())
		return err
	}
	// Set up error pipe
	stderr, err := cmd.StderrPipe()
	if err != nil {
		rlog.Error("Cannot establish pipe for stderr: %v" + err.Error())
		return err
	}
	// Start the command after having set up the pipes
	if err = cmd.Start(); err != nil {
		rlog.Error("Cannot start decoding program: %v" + err.Error())
		return err
	}

	// read command's stdout line by line
	cmdOut := bufio.NewScanner(stdout)
	cmdOut.Split(bufio.ScanWords)
	for cmdOut.Scan() {
		if strings.Contains(cmdOut.Text(), "Dekodiere") || strings.Contains(cmdOut.Text(), "Ausgabe") {
			prg += 100
			continue
		}
		if re, _ := regexp.Compile(`\d+%`); re.MatchString(cmdOut.Text()) {
			n, _ := strconv.Atoi(strings.TrimSuffix(cmdOut.Text(), "%"))
			if (prg+n/3)-prgSet > 5 || prgSet == 0 {
				prgSet = (prg + n) / 3
				setPrgBar(key, prgActDec, prgSet)
			}
		}
	}
	setPrgBar(key, prgActDec, 100)

	// read command's stderr line by line and store it in a errStr for further processing
	cmdErr := bufio.NewScanner(stderr)
	for cmdErr.Scan() {
		errStr += fmt.Sprintf("%s\n", cmdErr.Text())
	}

	if err = cmd.Wait(); err != nil {
		// In case command line execution returns error, content of stderr (now contained in
		// errStr) is written into error file
		errFilePath := cfg.logDirPath + "/" + key + errFileSuffixDec
		if errFile, e := os.Create(errFilePath); e != nil {
			rlog.Error("Cannot create \"" + errFilePath + "\": " + e.Error())
		} else {
			if _, e = errFile.WriteString(errStr); e != nil {
				rlog.Error("Cannot write into \"" + errFilePath + "\": " + e.Error())
			}
			_ = errFile.Close()
		}
	}

	return err
}
