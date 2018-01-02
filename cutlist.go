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

// cutlist.go contains the implmenetation of cutlist retrievals. Currently,
// only cutlist.at is supported.

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/html/charset"

	"github.com/go-ini/ini"
	"github.com/romana/rlog"
)

// Cutlist stores cutlists loaded from a cutlist server
// segment structure for cutlist
type seg struct {
	timeStart  float64 // start time (in seconds)
	timeDur    float64 // duration (time in seconds)
	frameStart int     // start frame (frame number)
	frameDur   int     // duration (number of frames)
}
type cutlist struct {
	id         string
	app        string
	ratio      string
	fps        float64
	timeBased  bool
	frameBased bool
	segs       []*seg // the list of cuts
}

// An array of clHeader is used to store the header information of the cutlists
// retrieved from the cutlist server. The score will be calculated based on the
// ratings. It will also be used to sort the array.
type clHeader struct {
	score float64
	id    string
}
type clHeaders []clHeader

// implement sort interface
func (clhs clHeaders) Len() int           { return len(clhs) }
func (clhs clHeaders) Less(i, j int) bool { return clhs[i].score > clhs[j].score } // sort descending by score
func (clhs clHeaders) Swap(i, j int)      { clhs[i], clhs[j] = clhs[j], clhs[i] }

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
	go v.autoIncr(prgActCL, 500, stop)

	// stop progress bar once fetchCutlists finalizes
	defer func() { stop <- struct{}{} }()

	// fetch cutlist headers from cutlist.at. If no lists could be retrieved: Print error
	// message and return
	if ids = v.fetchCutlistHeaders(); len(ids) == 0 {
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

// fetchCutlist loops at a (sorted) cutlist header list and fetches the corresponding
// cutlist. In case of success, it returns. In case of failure, it continues with
// the next entry of the list
func fetchCutlistDetails(ids []string) *cutlist {

	// constants for cl INI file sections and keys
	const (
		clSectionGeneral = "general"
		clKeyNumCuts     = "noofcuts"
		clKeyRatio       = "displayaspectratio"
		clKeyApp         = "intendedcutapplicationname"
		clKeyFPS         = "framespersecond"
		clSectionCut     = "cut"
		clKeyTimeStart   = "start"
		clKeyTimeDur     = "duration"
		clKeyFrameStart  = "startframe"
		clKeyFrameDur    = "durationframes"
	)

	var (
		err         error
		cl          *cutlist
		clRetrieved bool
	)

	// Loop over the cutlist headers and fetch the correspond cutlist.
	// In case of success: return the cutlist
	for _, id := range ids {
		var (
			resp    *http.Response
			clINI   []byte
			clFile  *ini.File
			sec     *ini.Section
			key     *ini.Key
			numCuts int
			sg      *seg
		)

		clRetrieved = false

		// create new cutlist
		cl = new(cutlist)
		cl.id = id

		// fetch cutlist from cutlist.at by calling URL
		if resp, err = http.Get(cfg.clsURL + "getfile.php?id=" + id); err != nil {
			// if no culist could be fetched: Nothing left to do, try next
			continue
		}
		// read data
		clINI, err = ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		// if data couldn't be read: Nothing left to do, try next
		if err != nil {
			continue
		}

		// open cutlist INI data source with go-ini
		if clFile, err = ini.InsensitiveLoad(clINI); err != nil {
			rlog.Error("Cutlist file could not be opened for ID '" + id + "':" + err.Error())
			continue
		}

		// get GENERAL section
		if sec, err = clFile.GetSection(clSectionGeneral); err != nil {
			rlog.Error("Cutlist ID=" + id + " does not have section '" + clSectionGeneral + "':" + err.Error())
			continue
		}

		// get display aspect ration
		if key, err = sec.GetKey(clKeyRatio); err != nil {
			rlog.Warn("Cutlist ID=" + id + " does not have key '" + clKeyRatio + "'")
		} else {
			cl.ratio = key.Value()
		}

		// get frames per second
		if key, err = sec.GetKey(clKeyFPS); err != nil {
			rlog.Warn("Cutlist ID=" + id + " does not have key '" + clKeyFPS + "'")
		} else {
			cl.fps, _ = strconv.ParseFloat(key.Value(), 64)
		}

		// get intended cut application
		if key, err = sec.GetKey(clKeyApp); err != nil {
			rlog.Warn("Cutlist ID=" + id + " does not have key '" + clKeyApp + "'")
		} else {
			cl.app = key.Value()
		}

		// get number of cuts
		if key, err = sec.GetKey(clKeyNumCuts); err != nil {
			rlog.Error("Cutlist ID=" + id + " does not have key '" + clKeyNumCuts + "'")
			continue
		}
		numCuts, _ = strconv.Atoi(key.Value())

		// read cuts
		for i := 0; i < numCuts; i++ {
			// get [Cut{i}] section
			if sec, err = clFile.GetSection(clSectionCut + strconv.Itoa(i)); err != nil {
				rlog.Error("Cutlist ID=" + id + " does not have section '" + clSectionCut + strconv.Itoa(i) + "'")
				break
			}
			sg = new(seg)
			// get start time
			if sec.HasKey(clKeyTimeStart) {
				key, _ = sec.GetKey(clKeyTimeStart)
				cl.timeBased = true
				sg.timeStart, _ = strconv.ParseFloat(key.Value(), 64)
			}
			// get time duration
			if sec.HasKey(clKeyTimeDur) {
				key, _ = sec.GetKey(clKeyTimeDur)
				sg.timeDur, _ = strconv.ParseFloat(key.Value(), 64)
			}
			// get start frame
			if sec.HasKey(clKeyFrameStart) {
				key, _ = sec.GetKey(clKeyFrameStart)
				cl.frameBased = true
				sg.frameStart, _ = strconv.Atoi(key.Value())
			}
			// get frames duration
			if sec.HasKey(clKeyFrameDur) {
				key, _ = sec.GetKey(clKeyFrameDur)
				sg.frameDur, _ = strconv.Atoi(key.Value())
			}

			if (sg.timeStart == 0.0 && sg.timeDur == 0.0) || (sg.frameStart == 0 && sg.frameDur == 0) {
				rlog.Warn("Cutlist ID=" + id + ": Cut " + clSectionCut + strconv.Itoa(i) + " does not have sufficient information")
				cl.segs = cl.segs[:0]
				break
			}

			cl.segs = append(cl.segs, sg)
		}
		// if no cuts
		if len(cl.segs) == 0 {
			continue
		}

		// cutlist has been parsed successfully: set clRetrieved accordingly
		//and leave loop
		clRetrieved = true
		break
	}

	// return either cutlist or nil
	if clRetrieved {
		return cl
	}
	return nil
}

// fetchCutlistHeaders requests cutlist header information for the cutlist server
// for the video. It returns the information as list of clHeader, sorted descending
// by score
func (v *video) fetchCutlistHeaders() []string {
	var (
		ids   []string
		clhs  clHeaders
		clh   clHeader
		resp  *http.Response
		err   error
		clXML []byte
		el    string
	)

	// constants for relevant element names of cutlist headers
	const (
		clTagID      = "ID"
		clTagRating  = "RATING"
		clTagCutlist = "CUTLIST"
	)

	// array of relevant element names
	clRelNames := [...]string{clTagID, clTagRating}
	// map to store values of relevant element values for one cutlist
	var clRelVals map[string]string

	fmt.Println(cfg.clsURL + "getxml.php?name=" + v.key + path.Ext(v.filePath))

	// fetch cutlist header from cutlist.at by calling URL
	if resp, err = http.Get(cfg.clsURL + "getxml.php?name=" + v.key + path.Ext(v.filePath)); err != nil {
		// if no culist could be fetched: Nothing left to do, return
		return ids
	}

	// read data
	clXML, err = ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	// if data couldn't be read: Nothing to do, return
	if err != nil {
		return ids
	}
	dec := xml.NewDecoder(bytes.NewReader(clXML))
	dec.CharsetReader = charset.NewReaderLabel
	// FROM: https://stackoverflow.com/questions/6002619/unmarshal-an-iso-8859-1-xml-input-in-go#32224438
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		} else if err != nil {
			rlog.Error("Error while reading cutlist headers: " + err.Error())
			return ids
		}

		switch tok := tok.(type) {
		case xml.StartElement:
			// if element is in list of relevant elements ...
			for _, s := range clRelNames {
				if strings.ToUpper(tok.Name.Local) == s {
					// ... store element name in el
					el = strings.ToUpper(tok.Name.Local)
					break
				}
			}
			// if new cutlists start ...
			if strings.ToUpper(tok.Name.Local) == clTagCutlist {
				// create new map to store the relevant values
				clRelVals = make(map[string]string)
			}
		case xml.EndElement:
			// if a relevant element ends ...
			if strings.ToUpper(tok.Name.Local) == el {
				// clear el
				el = ""
			}
			// if the end of a cutlist has been reached ...
			if strings.ToUpper(tok.Name.Local) == clTagCutlist {
				// fill custlist header struct ...
				clh.id = clRelVals[clTagID]
				clh.score, _ = strconv.ParseFloat(clRelVals[clTagRating], 64)
				// and append it to the header list
				if clh.id != "" {
					clhs = append(clhs, clh)
				}
			}
		case xml.CharData:
			// if element is relecvant ...
			if el != "" {
				// store value for later processing
				clRelVals[el] = string(tok)
			}
		}
	}

	// sort clHeaders descending by score
	sort.Sort(clhs)

	// build up cutlist array for cutlist header array
	for _, clh := range clhs {
		id := clh.id
		ids = append(ids, id)
	}

	return ids
}

// hasCutlists checks if the cutlist server has cutlists for that video
func (v *video) hasCutlists() bool {
	// fetch cutlist headers from cutlist.at. If no lists could be retrieved: Log message and return
	if len(v.fetchCutlistHeaders()) == 0 {
		rlog.Trace(1, "No cutlist header could be fetched for "+v.key)
		return false
	}
	return true
}
