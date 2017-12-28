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

package videos

// Package videos contains all the logic to decode videos, retrieve
// cutlists and cut videos.

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/romana/rlog"

	"github.com/mipimipi/gool/internal/cfg"
	"github.com/mipimipi/gool/internal/videos/progress"
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
	key      string // key [= file name without (a) suffix ".otrkey" and (b) sub string "cut."]
	status   string // Whether a video is encoded, decoded or cut
	res      string
	filePath string
	cl       *cutlist // cutlists
}

// Global video list
var vl = make(map[string]*video)

// format str for listing videos
var vidFormatStr = "%-" + strconv.Itoa(vidPrtKeyLen) + "s %-" + strconv.Itoa(vidPrtStatusLen) + "s %-" + strconv.Itoa(vidPrtCLLen) + "s %-" + strconv.Itoa(vidPrtResLen) + "s"

// Takes a file path and - based on the filename - checks if it's an OTR video
// or not. If it's no OTR video, an error is returned. If it's a video, the
// function returns (a) the key and (b) the status - i.e. whether it's encoded,
// decoded or cut
func analyzeFile(fileName string) (string, string, error) {
	var (
		key    string
		status string
		err    error
	)

	// check if fileName is an OTR file
	if re, _ := regexp.Compile(`\w+_\d{2}\.\d{2}\.\d{2}_\d{2}-\d{2}_\w+`); !re.MatchString(fileName) {
		rlog.Trace(2, "File "+fileName+" is no OTR File")
		return key, status, fmt.Errorf("File %s is no OTR File", fileName)
	}
	// check if video is encoded ...
	if filepath.Ext(fileName) == ".otrkey" {
		status = vidStatusEnc
		key = strings.TrimSuffix(fileName, ".otrkey")
	} else {
		// ... if not: check if video is already cut ...
		if strings.Contains(fileName, ".cut.") {
			status = vidStatusCut
			key = strings.Replace(fileName, "cut.", "", -1)
		} else {
			// ... otherwise video must be decoded but uncut
			status = vidStatusDec
			key = fileName
		}
	}
	if status == "" {
		return key, status, fmt.Errorf("Could not determine the status of %s", fileName)
	}

	return key, status, err
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
		rlog.Trace(1, "Error during decoding or cutlist fetching")
		return
	}

	// clean up stuff from former processing runs
	if err := v.preProcessing(); err != nil {
		return
	}

	// Call FFmpeg to cut the video
	errCut := callFFmpeg(v)

	// Process videos based on error info from decoding go routine
	if err := v.postProcessing(errCut); err != nil {
		fmt.Println(err.Error())
		rlog.Error(err.Error())
	}
}

// decode decodes an encoded video. Once decoding has been done,
// a corresponding item is send to the channel r.
func (v *video) decode(wg *sync.WaitGroup, r chan<- res) {
	// Decrease wait group counter when function is finished
	defer wg.Done()

	// clean up stuff from former processing runs
	if err := v.preProcessing(); err != nil {
		r <- res{key: v.key, err: err}
		return
	}

	// Call otrdecoder
	errOTR := callOTRDecoder(v.filePath, v.key)

	// Process videos based on error info from decoding go routine
	if err := v.postProcessing(errOTR); err != nil {
		fmt.Println(err.Error())
		rlog.Error(err.Error())
	}

	// write error message to channel
	if errOTR != nil {
		r <- res{key: v.key, err: errOTR}
		return
	}

	// Decoding successfully done: Write nil error into results channel
	r <- res{key: v.key, err: nil}
}

// fetchCutlist retrieves a cutlist from cutlist.at based on the key of the
// video. Once the retrieval  is done, a corresponding item is send to the
// channel r.
func (v *video) fetchCutlist(wg *sync.WaitGroup, r chan<- res) {
	// Decrease wait group counter when function is finished
	defer wg.Done()

	var ids []string

	// create stop channel for progress bar
	stop := make(chan struct{})

	// start automatic progress bar which increments every 0.5s
	go progress.AutoIncr(v.key, progress.PrgActCL, 500, stop)

	// stop progress bar once fetchCutlists finalizes
	defer func() { stop <- struct{}{} }()

	// fetch cutlist headers from cutlist.at. If no lists could be retrieved: Print error
	// message and return
	if ids = fetchCutlistHeaders(v.key); len(ids) == 0 {
		rlog.Trace(1, "No cutlist header could be fetched for "+v.key)
		r <- res{key: v.key, err: fmt.Errorf("Keine Cutlists vorhanden")}
		return
	}

	// retrieve cutlist from cutlist.at using the cutlist header list. If no cutlist could
	// be retrieved: Print error message and return
	if v.cl = fetchCutlistDetails(ids); v.cl == nil {
		rlog.Trace(1, "No cutlist could be fetched for "+v.key)
		r <- res{key: v.key, err: fmt.Errorf("Keine Cutlist konnte gelesen werden")}
		return
	}

	// Cutlist fetched: Write nil error into results channel
	r <- res{key: v.key, err: nil}
}

// hasCutlists checks if the cutlist server has cutlists for that video
func (v *video) hasCutlists() bool {
	// fetch cutlist headers from cutlist.at. If no lists could be retrieved: Log message and return
	if len(fetchCutlistHeaders(v.key)) == 0 {
		rlog.Trace(1, "No cutlist header could be fetched for "+v.key)
		return false
	}
	return true
}

// Print prints the video list to stdout
func Print() {
	// Check if there are videos at all ...
	if len(vl) == 0 {
		fmt.Printf("\nKeine Videos gefunden\n\n")
		return
	}

	// ... if yes: Print list
	fmt.Printf("\n\033[1m"+vidFormatStr+"\033[22m\n", "Video", "Status", "CL", "Resultat")
	fmt.Println("--------------------------------------------------------------------------------")
	for _, v := range vl {
		fmt.Println(v.string())
	}
	fmt.Printf("\n")
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
	if cfg.DoCleanUp {
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
		v.filePath = cfg.DecDirPath + "/" + v.key
		return nil
	}
	if v.status == vidStatusDec {
		v.status = vidStatusCut
		v.filePath = cfg.CutDirPath + "/" + v.key
	}

	return err
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
	errFilePath = cfg.LogDirPath + "/" + v.key
	switch v.status {
	case vidStatusEnc:
		errFilePath += cfg.ErrFileSuffixDec
	case vidStatusDec:
		errFilePath += cfg.ErrFileSuffixCut
	}
	_ = os.Remove(errFilePath)

	// get filename (without path)
	srcDir, fileName := path.Split(v.filePath)

	// depending on the status of the video, it's in the corresponding sub dir
	// (if it isn't already there)
	switch v.status {
	case vidStatusEnc:
		dstPath = cfg.EncDirPath + "/" + fileName
	case vidStatusDec:
		dstPath = cfg.DecDirPath + "/" + fileName
	case vidStatusCut:
		dstPath = cfg.CutDirPath + "/" + fileName
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

// Process triggers the complete processing of the videos in the video list:
// Decoding, fetching of cutlist, cutting.
// As far as possible, this is done in parallel. For one video, decoding and
// fetching of cutlist is done in parallel. If both is done, the video is cut.
// This behaviour is implemented using go routines and channels.
// The processing steps of different videos are done in parallel.
func Process() {
	var i int

	// determine if there are videos that are relevant for executiont (as otherwise start
	// message doesn't need to be dsplayed)
	for _, v := range vl {
		if v.status != vidStatusCut {
			i++
		}
	}

	// nothing to do: return
	if i == 0 {
		return
	}

	// print status message
	fmt.Println("\n\033[1m:: Prozessiere Videos ...\033[22m")

	var (
		wg sync.WaitGroup
		r  chan res
		rs []chan res
	)

	// remove tmp directory
	defer func() { _ = os.RemoveAll(cfg.TmpDirPath) }()

	// start progress tracking
	progress.Start()

	// trigger processing for all videos in the list
	for _, v := range vl {
		// if video is already cut: nothing to do
		if v.status == vidStatusCut {
			continue
		}

		// create channel for the communication:
		// (1) decode method        -> cut method
		// (2) fetch cutlist method -> cut method
		r = make(chan res, 2)
		rs = append(rs, r)

		// Increase waitgroup counter
		wg.Add(2)

		// Cut video in go routine
		go v.cut(&wg, r)

		// Fetch cutlist for video in go routine
		go v.fetchCutlist(&wg, r)

		// if videos needs to be decoded ...
		if v.status == vidStatusEnc {
			// Increase waitgroup counter
			wg.Add(1)
			// Decode video in go routine
			go v.decode(&wg, r)
		} else {
			// otherwise put success indication into channel
			r <- res{key: v.key, err: nil}
		}
	}

	// wait until parallel sub processes are finished
	wg.Wait()

	//close channels
	for _, r = range rs {
		close(r)
	}

	// stop progress tracking
	progress.Stop()
}

// Read builds up a video list by reading videos ...
// - from the places passed via command line parameters
// - stored in the gool working dir and its sub directories "Encoded", "Decoded", Cut"
func Read(patterns []string) error {
	var (
		err       error
		filePaths []string
		fileName  string
		filePath  string
		status    string
		key       string
		v         *video
	)

	// print status message
	fmt.Printf("\n\033[1m:: Lese Videodateien ein ...\033[22m\n")

	// add working dir and the sub dirs for enc, dec and cut to the pattern list
	patterns = append(patterns, cfg.WrkDirPath+"/*", cfg.EncDirPath+"/*", cfg.DecDirPath+"/*", cfg.CutDirPath+"/*")

	for _, p := range patterns {
		// Get all files that fits to pattern p by calling globbing function
		if filePaths, err = filepath.Glob(p); err != nil {
			rlog.Trace(1, "'"+p+"' couldn't be interpreted: "+err.Error())
			continue
		}

		for _, filePath = range filePaths {
			// check if filePath is a directory. In that case: do nothing and continue
			info, _ := os.Stat(filePath)
			if info.IsDir() {
				continue
			}

			// determine filename
			_, fileName = filepath.Split(filePath)

			// Update video list from filePath:
			// Determine key and status of video
			if key, status, err = analyzeFile(fileName); err != nil {
				continue
			}
			// print progress message
			if len(fileName) > 77 {
				fmt.Println(fileName[:77] + "...")
			} else {
				fmt.Println(fileName)
			}
			// update video list
			if vl[key] != nil {
				// if a video for that key is already existing: Update it
				vl[key].updateFromFile(status, filePath)
			} else {
				// ... else: Create a new one and add it to the global video list
				v = new(video)
				v.key = key
				v.status = status
				v.res = vidResultNone
				v.filePath = filePath
				vl[key] = v
			}
		}
	}

	return err
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

	if ((v.status == vidStatusEnc) && (status != vidStatusEnc)) || ((v.status == vidStatusDec) && (status == vidStatusCut)) {
		v.status = status
		v.filePath = filePath
	} else {
		// if clean up is required: Delete file
		if cfg.DoCleanUp {
			if err = os.Remove(filePath); err != nil {
				err = fmt.Errorf("%s konnte nicht gelöscht werden: %v", filePath, err)
				rlog.Warn(filePath + " couldn't be deleted: " + err.Error())
			} else {
				rlog.Trace(3, v.filePath+" has been deleted")
			}
		}
	}
}
