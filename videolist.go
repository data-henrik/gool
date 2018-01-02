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

// videolist.go contains all the logic to decode videos, retrieve
// cutlists and cut videos.

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/romana/rlog"
)

// type video list
type videoList map[string]*video

// Takes a file path and - based on the filename - checks if it's an OTR video
// or not. If it's no OTR video, an error is returned. If it's a video, the
// function returns (a) the key and (b) the status - i.e. whether it's encoded,
// decoded or cut
func analyzeFile(fileName string) (string, string, error) {
	var (
		key    string
		status string
		fn     string
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
		fn = strings.TrimSuffix(fileName, ".otrkey")
	} else {
		// ... if not: check if video is already cut ...
		if strings.Contains(fileName, ".cut.") {
			status = vidStatusCut
			fn = strings.Replace(fileName, "cut.", "", -1)
		} else {
			// ... otherwise video must be decoded but uncut
			status = vidStatusDec
			fn = fileName
		}
	}
	if status == "" {
		return "", "", fmt.Errorf("Could not determine the status of %s", fileName)
	}

	// determine key (=filename without extension)
	key = fn[:len(fn)-len(path.Ext(fn))]

	return key, status, err
}

// print prints the video list to stdout
func (vl videoList) print() {
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

// process triggers the complete processing of the videos in the video list:
// Decoding, fetching of cutlist, cutting.
// As far as possible, this is done in parallel. For one video, decoding and
// fetching of cutlist is done in parallel. If both is done, the video is cut.
// This behaviour is implemented using go routines and channels.
// The processing steps of different videos are done in parallel.
func (vl videoList) process() {
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
	defer func() { _ = os.RemoveAll(cfg.tmpDirPath) }()

	// start progress tracking
	start()

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
	stop()
}

// read builds up a video list by reading videos ...
// - from the places passed via command line parameters
// - stored in the gool working dir and its sub directories "Encoded", "Decoded", Cut"
func (vl videoList) read(patterns []string) error {
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
	patterns = append(patterns, cfg.wrkDirPath+"/*", cfg.encDirPath+"/*", cfg.decDirPath+"/*", cfg.cutDirPath+"/*")

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

			// normalize filePath to absolute path
			filePath, _ = filepath.Abs(filePath)

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
				v = newVideo()
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
