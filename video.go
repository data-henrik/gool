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
// along with gool. If not, see <http://www.gnu.org/licenses/>.

package main

// videos.go contains all the logic to decode videos, retrieve
// cutlists and cut videos.

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/romana/rlog"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"
)

// Constants for video status
const (
	vidStatusEnc = "ENC" // encoded
	vidStatusDec = "DEC" // decoded
	vidStatusCut = "CUT" // cut
)

// Constants for video processing status
const (
	vidResultOK   = "ERFOLG" // everything OK
	vidResultErr  = "FEHLER" // error
	vidResultNone = ""       // no result yet
)

// Constants for printing video information
const (
	vidPrtKeyLen    = 60 // key length
	vidPrtCLLen     = 2  // length of cutlist existence indicator
	vidPrtStatusLen = 6  // Status length
	vidPrtResLen    = 8  // result length
)

// Structure for the result of video processing (decoding or cutting)
type res struct {
	key string
	err error
}

// Represents one video
type video struct {
	key      string // key [= file name without (a) suffix ".otrkey", (b) sub string "cut." and (c) file type (.avi, .mkv etc.)]
	status   string // Whether a video is encoded, decoded or cut
	res      string
	filePath string
	cl       *cutlist         // cutlists
	pbs      map[int]*mpb.Bar // progress bars (key is action, like "decode", "cut", "fetch cutlist")
}

// format str for listing videos
var vidFormatStr = "%-" + strconv.Itoa(vidPrtKeyLen) + "s %-" + strconv.Itoa(vidPrtStatusLen) + "s %-" + strconv.Itoa(vidPrtCLLen) + "s %-" + strconv.Itoa(vidPrtResLen) + "s"

// constants to indicate actions
const (
	prgActDec = iota // action "decode"
	prgActCL         // action "fetch cutlist"
	prgActCut        // action "cut"
)

// constants for string lengths
const (
	prgBarLen = 20 // length of progress bar
	prgKeyLen = 38 // length of video key in front of progress bar
)

// progress container
var p *mpb.Progress

// lock to enable concurrent writing into map
var prgLock sync.Mutex

// autoIncr implements an automated counter to increase the progress for a given
// video and action combination. The counter is based on the Tick channel from
// the time package and can be stopped via the stop channel. It is incremented
// each interval microseconds
func (v *video) autoIncr(act int, interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			// increase progress bar
			v.setPrgBar(act, int(v.getBar(act).Current())+100/prgBarLen)
		case <-stop:
			// set progress to 100% (which also completes the bar) ...
			v.setPrgBar(act, 100)
			// stop ticker ...
			ticker.Stop()
			// and return
			return
		}
	}
}

// getBar returns a progress bar for a given video / action combination.
// If there's not yet a bar for that combination, it's created.
func (v *video) getBar(act int) *mpb.Bar {

	var (
		bar *mpb.Bar
		ok  bool
	)

	// Locking is done to enable concurrent writing
	prgLock.Lock()
	defer prgLock.Unlock()

	// read bar from map. If there's no bar for the given video / action
	// combination ...
	if bar, ok = v.pbs[act]; !ok {
		// create new bar
		bar = p.AddBar(100,
			mpb.PrependDecorators(
				decor.StaticName(v.prependStr(act), 0, 0),
			),
			mpb.AppendDecorators(
				decor.Percentage(3, decor.DSyncSpace),
			),
			mpb.BarTrim(),
		)

		// writing bar into video/action/bar map.
		v.pbs[act] = bar
	}

	return bar
}

// newVideo allocates memory for a new video and returns a reference to that. This dedicated
// function is necessary to make the progress bar map
func newVideo() *video {
	var v video
	v.pbs = make(map[int]*mpb.Bar)
	return &v
}

// Takes result of a video processing step (decoding or cutting) and adjusts the
// video status etc.
func (v *video) postProcessing(vErr error) error {
	var err error

	// In case of error: Set processing status to error
	if vErr != nil {
		v.res = vidResultErr
		return nil
	}

	// ... otherwise: Set processing status to OK
	v.res = vidResultOK

	// If cleanup is required: Delete old file
	// TODO: Store uncutted file in "CutOriginal"
	if cfg.doCleanUp {
		if err = os.Remove(v.filePath); err != nil {
			err = fmt.Errorf("%s konnte nicht gelöscht werden: %v", v.filePath, err)
			rlog.Warn(v.filePath + " couldn't be deleted: " + err.Error())
		} else {
			rlog.Trace(3, v.filePath+" has been deleted")
		}
	}

	// Set new status and adjust filePath
	if v.status == vidStatusEnc {
		v.status = vidStatusDec
		v.filePath = cfg.decDirPath + "/" + v.key + path.Ext(v.filePath)
		return nil
	}
	if v.status == vidStatusDec {
		v.status = vidStatusCut
		v.filePath = cfg.cutDirPath + "/" + v.key + path.Ext(v.filePath)
	}

	return err
}

// prependStr builds the string that is printed left of the progress bar
func (v *video) prependStr(act int) string {
	var key string

	// define strings for the corresponsing actions
	actStr := [3]string{"Dekodiere", "Hole Cutlist", "Schneide"}

	// adjust key length for printing
	if len(v.key) > prgKeyLen {
		key = v.key[:prgKeyLen-3] + "..."
	} else {
		key = v.key
	}

	// build and return string
	return fmt.Sprintf("%"+strconv.Itoa(prgKeyLen)+"s:: %-12s ", key, actStr[act])
}

// Does some cleanup before processing is started:
// - deletes log files from former runs
// - moves video file to the corresponding sub dir of
//   the working dir if necessary
func (v *video) preProcessing() error {
	var errFilePath string
	var dstPath string
	var err error

	// Delete old error file
	errFilePath = cfg.logDirPath + "/" + v.key + path.Ext(v.filePath)
	switch v.status {
	case vidStatusEnc:
		errFilePath += errFileSuffixDec
	case vidStatusDec:
		errFilePath += errFileSuffixCut
	}
	_ = os.Remove(errFilePath)

	// get filename (without path)
	srcDir, fileName := path.Split(v.filePath)

	// depending on the status of the video, it's in the corresponding sub dir
	// (if it isn't already there)
	switch v.status {
	case vidStatusEnc:
		dstPath = cfg.encDirPath + "/" + fileName
	case vidStatusDec:
		dstPath = cfg.decDirPath + "/" + fileName
	case vidStatusCut:
		dstPath = cfg.cutDirPath + "/" + fileName
	}
	// if video file is not in the correct sub dir ...
	if v.filePath != dstPath {
		// move video file into correspondig sub dir
		if err = os.Rename(srcDir+fileName, dstPath); err != nil {
			err = fmt.Errorf("File %s kann nicht verschoben werden: %v", fileName, err)
			rlog.Error(v.filePath + " cannot be moved to " + dstPath + ": " + err.Error())
		}
		// adjust file path in video object accordingly
		v.filePath = dstPath
	}

	return err
}

// setPrgBar updates the progress bar for a specific video (key) / action (act) combination based on the
// progress (prg)
func (v *video) setPrgBar(act int, prg int) {
	//get progress bar for a combination of a video and an action
	bar := v.getBar(act)

	// update the bar
	bar.Incr(prg - int(bar.Current()))
}

// start creates a new progress container and needs to be called before any
//progress bar is created
func start() {
	// create new progress container
	p = mpb.New(
		mpb.WithWidth(prgBarLen),
	)
}

// Stop calls the Stop function of progress container. This flushes the
// buffer. Stop needs to be called at the end of video processing.
func stop() {
	p.Stop()
}

// returns the video attributes as string, formatted according to the format string
func (v *video) string() string {
	var (
		keyStr string
		clStr  string
		resStr string
	)

	// print cutlist information
	if v.hasCutlists() {
		clStr = fmt.Sprintf("\033[32m\033[1m++\033[22m\033[39m")
	} else {
		clStr = fmt.Sprintf("\033[31m\033[1m--\033[22m\033[39m")
	}

	// print result
	switch v.res {
	case vidResultOK:
		resStr = fmt.Sprintf("\033[32m\033[1m%-8s\033[22m\033[39m", v.res)
	case vidResultErr:
		resStr = fmt.Sprintf("\033[31m\033[1m%-8s\033[22m\033[39m", v.res)
	case vidResultNone:
		resStr = v.res
	}

	// print key (potentially shortened)
	if len(v.key) > vidPrtKeyLen {
		keyStr = v.key[:vidPrtKeyLen-3] + "..."
	} else {
		keyStr = v.key
	}

	return fmt.Sprintf(vidFormatStr, keyStr, v.status, clStr, resStr)
}

// updateFromFile is called once another file for an already existing video
// (i.e. a video that is already existing in the video list) has been read.
// The existing video is updated accordingly. E.g., if the video in the list
// is of status "encoded" and the file contains the decoded version of the
// video, the status is set to decoded, and the filepath is set to the file,
// in addtion the encoded version of the video is deleted.
func (v *video) updateFromFile(status string, filePath string) {
	var err error

	// nothing to do if both video files are the same
	if filePath == v.filePath {
		return
	}

	if ((v.status == vidStatusEnc) && (status != vidStatusEnc)) || ((v.status == vidStatusDec) && (status == vidStatusCut)) {
		v.status = status
		v.filePath = filePath
	} else {
		// if clean up is required: Delete file
		if cfg.doCleanUp {
			if err = os.Remove(filePath); err != nil {
				err = fmt.Errorf("%s konnte nicht gelöscht werden: %v", filePath, err)
				rlog.Warn(filePath + " couldn't be deleted: " + err.Error())
			} else {
				rlog.Trace(3, v.filePath+" has been deleted")
			}
		}
	}
}
