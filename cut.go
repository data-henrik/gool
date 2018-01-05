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

	"github.com/sirupsen/logrus"
)

// callMKVmerge calls mkvmerge and handles the command line output. It return
// the container format in case the container format has changed (otherwise "")
func (v *video) callMKVmerge() (string, error) {
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
		splitStr = "parts-frames:"
		for i := 0; i < len(v.cl.segs); i++ {
			if i > 0 {
				splitStr += ",+"
			}
			splitStr += strconv.Itoa(v.cl.segs[i].frameStart) + "-" + strconv.Itoa(v.cl.segs[i].frameStart+v.cl.segs[i].frameDur)
		}
	} else {
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
	// print cmd string to log
	{
		s := ""
		for _, t := range cmd.Args {
			s += t + " "
		}
		log.WithFields(logrus.Fields{"key": v.key}).Debugf("Cut command: %s", s)
	}
	// Set up error pipe
	stderr, err = cmd.StderrPipe()
	if err != nil {
		log.WithFields(logrus.Fields{"key": v.key}).Errorf("Cannot establish pipe for stderr: %v", err.Error())
		return "", err
	}
	// Start the command after having set up the pipes
	if err = cmd.Start(); err != nil {
		log.WithFields(logrus.Fields{"key": v.key}).Errorf("Cannot start MKVmerge: %v", err.Error())
		return "", err
	}
	log.WithFields(logrus.Fields{"key": v.key}).Infof("Video has been cut with MKVmerge: %s", outFilePath)

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
			log.WithFields(logrus.Fields{"key": v.key}).Errorf("Cannot create \"%s\": %v", errFilePath, e)
		} else {
			if _, e = errFile.WriteString(errStr); e != nil {
				log.WithFields(logrus.Fields{"key": v.key}).Errorf("Cannot write into \"%s\": %v", errFilePath, e)
			}
			_ = errFile.Close()
		}
	}

	// set progress to 100%
	v.setPrgBar(prgActCut, 100)

	return "mkv", err
}

// cut cuts a video according to it's cutlist. The method is called as go
// routine. It can only be called if the video has (a) been decoded and
// (b) a cutlist has been fetched. The fulfillment of both prerequisites
// is indicated by two items in the channel r.
func (v *video) cut(wg *sync.WaitGroup, r <-chan res) {
	// Decrease wait group counter when function is finished
	defer wg.Done()

	// receive to items from channel r ...
	r0 := <-r
	r1 := <-r
	// ... and check if none of them carries an error (this is the case
	// if decoding and fetching of cutlist have been successful)
	if r0.err != nil || r1.err != nil {
		if r0.err != nil {
			log.WithFields(logrus.Fields{"key": v.key}).Errorf("Error during decoding or cutlist loading: %v", r0.err)
		}
		if r1.err != nil {
			log.WithFields(logrus.Fields{"key": v.key}).Errorf("Error during decoding or cutlist loading: %v", r1.err)
		}
		return
	}

	// clean up stuff from former processing runs
	if err := v.preProcessing(); err != nil {
		return
	}

	// call MKVmerge to cut the video
	cf, errCut := v.callMKVmerge()

	// Process videos based on error info from decoding go routine
	if err := v.postProcessing(cf, errCut); err != nil {
		log.WithFields(logrus.Fields{"key": v.key}).Error(err.Error())
	}
}

// timeStr takes a time duration or point in time as floating point and
// returns a string representation in the format "HH:MM:SS.ssssss"
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
