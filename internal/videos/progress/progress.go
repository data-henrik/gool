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

package progress

// Package progress is a wrapper for the multi process bar package mpb.
// Process bars are created with a key consisting of a videky key and an
// action (decode, fetch cutlist, cut).
// Progress can be set for a given video / action combination in two
// different ways:
// (1) Explicite via function Set
// (2) In an automated way via function AutoIncr

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"
)

// constants to indicate actions
const (
	PrgActDec = iota // action "decode"
	PrgActCL         // action "fetch cutlist"
	PrgActCut        // action "cut"
)

// constants for string lengths
const (
	prgBarLen = 20 // length of progress bar
	prgKeyLen = 38 // length of video key in front of progress bar
)

// progress container
var p *mpb.Progress

// key structure for video / action combination
type prgKey struct {
	key string
	act int
}

// mapping video / action -> bar
var prgList = make(map[prgKey]*mpb.Bar)

// lock to enable concurrent writing into map
var prgLock sync.Mutex

// AutoIncr implements an automated counter to increase the progress for a given
// video and action combination. The counter is based on the Tick channel from
// the time package and can be stopped via the stop channel. It is incremented
// each interval microseconds
func AutoIncr(key string, act int, interval time.Duration, stop <-chan struct{}) {
	// tick channel receives an event every 0.5 seconds
	tick := time.Tick(interval * time.Millisecond)
	for {
		select {
		case <-tick:
			// increase progress bar
			Set(key, act, int(getBar(key, act).Current())+100/prgBarLen)
		case <-stop:
			// set progress to 100% (which also completes the bar) ...
			Set(key, act, 100)
			// and return
			return
		}
	}
}

// getBar returns a progress bar for a given video / action combination.
// If there's not yet a bar for that combination, it's created.
func getBar(key string, act int) *mpb.Bar {

	var (
		bar *mpb.Bar
		ok  bool
	)

	// Locking is done to enable concurrent writing
	prgLock.Lock()
	defer prgLock.Unlock()

	// read bar from map. If there's no bar for the given video / action
	// combination ...
	if bar, ok = prgList[prgKey{key: key, act: act}]; !ok {
		// create new bar
		bar = p.AddBar(100,
			mpb.PrependDecorators(
				decor.StaticName(prependStr(key, act), 0, 0),
			),
			mpb.AppendDecorators(
				decor.Percentage(3, decor.DSyncSpace),
			),
			mpb.BarTrim(),
		)

		// writing bar into video/action/bar map.
		prgList[prgKey{key: key, act: act}] = bar
	}

	return bar
}

// prependStr builds the string that is printed left of the progress bar
func prependStr(key string, act int) string {
	// define strings for the corresponsing actions
	actStr := [3]string{"Dekodiere", "Hole Cutlist", "Schneide"}

	// adjust key length for printing
	if len(key) > prgKeyLen {
		key = key[:prgKeyLen-3] + "..."
	}

	// build and return string
	return fmt.Sprintf("%-12s :: %"+strconv.Itoa(prgKeyLen)+"s ", actStr[act], key)
}

// Set updates the progress bar for a specific video (key) / action (act) combination based on the
// progress (prg)
func Set(key string, act int, prg int) {
	//get progress bar for a combination of a video and an action
	bar := getBar(key, act)

	// update the bar
	bar.Incr(prg - int(bar.Current()))
}

// Start creates a new progress container and needs to be called before any
//progress bar is created
func Start() {
	// create new progress container
	p = mpb.New(
		mpb.WithWidth(prgBarLen),
	)
}

// Stop calls the Stop function of progress container. This flushes the
// buffer. Stop needs to be called at the end of video processing.
func Stop() {
	p.Stop()
}
