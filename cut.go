// Copyright (C) 2018 Michael Picht
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

// cut.go implements the call of command line tools to cut a video based on
// a cutlist is implemented. Currently, only FFmpeg is used.

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/romana/rlog"
)

// callMKVmerge calls mkvmerge and handles the command line output.
func (v *video) callMKVmerge() error {
	var (
		err         error
		errStr      string
		splitStr    string
		outFilePath string
		stderr      io.ReadCloser
	)

	// create stop channel for progress bar
	stop := make(chan struct{})

	// start automatic progress bar which increments every 0.5s
	go v.autoIncr(prgActCut, 500, stop)

	// stop progress bar once fetchCutlists finalizes
	defer func() { stop <- struct{}{} }()

	// create split string for MKVmerge
	if v.cl.frameBased {
		rlog.Trace(3, "Cutlist is frame based")
		splitStr = "parts-frames:"
		for i := 0; i < len(v.cl.segs); i++ {
			if i > 0 {
				splitStr += ",+"
			}
			splitStr += strconv.Itoa(v.cl.segs[i].frameStart) + "-" + strconv.Itoa(v.cl.segs[i].frameStart+v.cl.segs[i].frameDur)
		}
	} else {
		rlog.Trace(3, "Cutlist is time based")
		splitStr = "parts:"
		for i := 0; i < len(v.cl.segs); i++ {
			if i > 0 {
				splitStr += ",+"
			}
			splitStr += timeStr(v.cl.segs[i].timeStart) + "-" + timeStr(v.cl.segs[i].timeStart+v.cl.segs[i].timeDur)
		}
	}

	// set path of output file
	outFilePath = cfg.cutDirPath + "/" + v.key + ".cut.mkv"

	// Create shell command for decoding
	cmd := exec.Command("mkvmerge",
		"-o", outFilePath,
		"--split", splitStr,
		v.filePath,
	)
	rlog.Trace(3, cmd)
	// Set up error pipe
	stderr, err = cmd.StderrPipe()
	if err != nil {
		rlog.Error("Cannot establish pipe for stderr: %v" + err.Error())
		return err
	}
	// Start the command after having set up the pipes
	if err = cmd.Start(); err != nil {
		rlog.Error("Cannot start MKVmerge: %v" + err.Error())
		return err
	}
	rlog.Trace(3, "Video has been cut with MKVmerge: ", outFilePath)

	// read command's stderr line by line and store it in a errStr for further processing
	cmdErr := bufio.NewScanner(stderr)
	for cmdErr.Scan() {
		errStr += fmt.Sprintf("%s\n", cmdErr.Text())
	}
	if err = cmd.Wait(); err != nil {
		// In case command line execution returns error, content of stderr (now contained in
		// errStr) is written into error file
		errFilePath := cfg.logDirPath + "/" + v.key + path.Ext(v.filePath) + errFileSuffixCut
		if errFile, e := os.Create(errFilePath); e != nil {
			rlog.Error("Cannot create \"" + errFilePath + "\": " + e.Error())
		} else {
			if _, e = errFile.WriteString(errStr); e != nil {
				rlog.Error("Cannot write into \"" + errFilePath + "\": " + e.Error())
			}
			_ = errFile.Close()
		}
	}

	// set progress to 100%
	v.setPrgBar(prgActCut, 100)

	return err
}

/*
TODO: Delete?
// cleanTmpDir deletes all files in the tmp directory that belong to the video indicated by key
func cleanTmpDir(key string) {
	filePaths, _ := filepath.Glob(cfg.tmpDirPath + "/" + key + "*")
	for _, filePath := range filePaths {
		if err := os.Remove(filePath); err != nil {
			err = fmt.Errorf("%s konnte nicht gelöscht werden: %v", filePath, err)
			rlog.Warn(filePath + " couldn't be deleted: " + err.Error())
		} else {
			rlog.Trace(3, filePath+" has been deleted")
		}
	}
}
*/

// cut cuts a video according to it's cutlist. The method is called as go
// routine. It can only be called if the video has (a) been decoded and
// (b) a cutlist has been fetched. The fulfillment of both prerequisites
// is indicated by two items in the channel r.
func (v *video) cut(wg *sync.WaitGroup, r <-chan res) {
	var errCut error

	// Decrease wait group counter when function is finished
	defer wg.Done()

	// receive to items from channel r ...
	r0 := <-r
	r1 := <-r
	// ... and check if none of them carries an error (this is the case
	// if decoding and fetching of cutlist have been successful)
	if r0.err != nil || r1.err != nil {
		rlog.Trace(1, "Error during decoding or cutlist fetching")
		return
	}

	// clean up stuff from former processing runs
	if err := v.preProcessing(); err != nil {
		return
	}

	// call MKVmerge to cut the video
	errCut = v.callMKVmerge()

	// Process videos based on error info from decoding go routine
	if err := v.postProcessing(errCut); err != nil {
		fmt.Println(err.Error())
		rlog.Error(err.Error())
	}
}

// timeStr takes a time duration or point in time as floating point and
// return a string in the format "HH:MM:SS.ssssss"
func timeStr(f float64) string {
	// a is an aray of length 2: a[0] is time in full seconds, a[1] contains the sub second time
	a := strings.Split(strconv.FormatFloat(f, 'f', 6, 64), ".")

	// i is time in full seconds as integer
	i, _ := strconv.Atoi(a[0])

	// hours
	hh := int(i / 3600)

	// decrease i by full hours
	i -= hh * 3600

	// minutes
	mm := int(i / 60)

	// seconds
	ss := i - mm*60

	return fmt.Sprintf("%02d:%02d:%02d.%s", hh, mm, ss, a[1])
}
