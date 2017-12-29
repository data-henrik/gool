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

package videos

// In this file, the call of command line tools to cut a video based on
// a cutlist is implemented. Currently, only FFmpeg is used.

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/romana/rlog"

	"github.com/mipimipi/gool/internal/cfg"
	"github.com/mipimipi/gool/internal/videos/progress"
)

// cleanTmpDir deletes all files in the tmp directory that belong to the video indicated by key
func cleanTmpDir(key string) {
	filePaths, _ := filepath.Glob(cfg.TmpDirPath + "/" + key + "*")
	for _, filePath := range filePaths {
		if err := os.Remove(filePath); err != nil {
			err = fmt.Errorf("%s konnte nicht gelöscht werden: %v", filePath, err)
			rlog.Warn(filePath + " couldn't be deleted: " + err.Error())
		} else {
			rlog.Trace(3, filePath+" has been deleted")
		}
	}
}

const (
	directUp   = iota // search upwards for IDR frame
	directDown        // search downwards for IDR frame
)

// getIDRFrameTime receives a point in time for a given video and return the point in time
// of the closest IDR frame. The frames of the video are retrieved by calling FFprobe.
func getIDRFrameTime(filePath string, key string, timeOrig float64, direct int) (float64, error) {
	// this function searches in [timeOrig-diff, timeOrig+diff] for IDR frame
	const diff = 10.0

	var (
		pictType bool // set to true if pict_type of frame is "I"
		keyFrame bool // set to true if key_frame of frame is "1"
		t        float64
		ts       float64
		te       float64
		errStr   string
	)
	// stores pkt_dts_time of the closest IDR frame. Here, it's set an initial value that
	// definitely is outside of [timeOrig-diff, timeOrig+diff]
	timeIDR := timeOrig + 2*diff

	switch direct {
	case directUp:
		ts = timeOrig
		te = timeOrig + diff
	case directDown:
		ts = timeOrig - diff
		te = timeOrig
	default:
		ts = timeOrig - diff
		te = timeOrig + diff
	}

	// Create shell command for decoding
	cmd := exec.Command("ffprobe",
		"-show_frames",
		"-pretty",
		"-read_intervals", strconv.FormatFloat(ts, 'f', 6, 64)+"%"+strconv.FormatFloat(te, 'f', 6, 64),
		filePath,
	)
	// Set up output pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		rlog.Error("Cannot establish pipe for stdout: %v" + err.Error())
		return 0, err
	}
	// Set up error pipe
	stderr, err := cmd.StderrPipe()
	if err != nil {
		rlog.Error("Cannot establish pipe for stderr: %v" + err.Error())
		return 0, err
	}
	// Start the command after having set up the pipes
	if err = cmd.Start(); err != nil {
		rlog.Error("Cannot start FFprobe: %v" + err.Error())
		return 0, err
	}

	// read command's stdout line by line
	cmdOut := bufio.NewScanner(stdout)
	cmdOut.Split(bufio.ScanWords)
	for cmdOut.Scan() {
		// set pictType
		if strings.Contains(cmdOut.Text(), "pict_type=I") {
			pictType = true
		}
		// set keyFrame
		if strings.Contains(cmdOut.Text(), "key_frame=1") {
			keyFrame = true
		}

		// store pkt_dts_time of frame in t in floating point format
		if strings.Contains(cmdOut.Text(), "pkt_dts_time") {
			s := strings.Split(cmdOut.Text()[13:], ":")
			// set t only if pkt_dts_time was filled
			if len(s) == 3 {
				t0, _ := strconv.ParseFloat(s[0], 64)
				t1, _ := strconv.ParseFloat(s[1], 64)
				t2, _ := strconv.ParseFloat(s[2], 64)

				t = 3600*t0 + 60*t1 + t2
			}
		}

		// in case the end of a frame is reached ...
		if strings.Contains(cmdOut.Text(), "[/FRAME]") {
			// ... and it's an IDR frame ...
			if keyFrame && pictType {
				// ... check if this frame is closer to timeOrig
				if math.Abs(timeOrig-t) < math.Abs(timeOrig-timeIDR) {
					timeIDR = t // store pkt_dts_time in timeIDR
				}
			}
			// initialize values for the next frame
			pictType = false
			keyFrame = false
			t = 0
		}
	}

	// read command's stderr line by line and store it in a errStr for further processing
	cmdErr := bufio.NewScanner(stderr)
	for cmdErr.Scan() {
		errStr += fmt.Sprintf("%s\n", cmdErr.Text())
	}

	if err = cmd.Wait(); err != nil {
		// In case command line execution returns error, content of stderr (now contained in
		// errStr) is written into error file
		errFilePath := cfg.LogDirPath + "/" + key + cfg.ErrFileSuffixCut
		if errFile, e := os.Create(errFilePath); e != nil {
			rlog.Error("Cannot create \"" + errFilePath + "\": " + e.Error())
		} else {
			if _, e = errFile.WriteString(errStr); e != nil {
				rlog.Error("Cannot write into \"" + errFilePath + "\": " + e.Error())
			}
			_ = errFile.Close()
		}
	}

	return timeIDR, nil
}

// callFFmpeg calls ffmpeg and handles the command line output. In case the
// cutlist only has one step, one single call of FFmpeg is sufficient.
// Otherwise FFmpeg needs to be called (a) once for every cut to extract a
// corresponding piece of the video and (b) again to concatenate the different
// partial videos. Therefore, these two cases need to be distinguished
// throughout this function
func callFFmpeg(v *video) error {
	var (
		err            error
		errStr         string
		prg            int
		prgInterval    int
		outFilePath    string
		concatFilePath string
		f              *os.File
		stderr         io.ReadCloser
	)
	// entire procedure consists of one step in case the cutlist only contains
	// one step. If it contains more than one step, another step is necessary
	// to concatenate the partial videos. prgInterval is set to 100/#steps
	if len(v.cl.segs) == 1 {
		// ... one step in case
		prgInterval = 100
	} else {
		prgInterval = 100 / (len(v.cl.segs) + 1)
	}

	// create file to store the file list for FFmpeg concat
	if len(v.cl.segs) > 1 {
		concatFilePath = cfg.TmpDirPath + "/" + v.key + ".list"
		if f, err = os.Create(concatFilePath); err != nil {
			rlog.Error("Cannot create list file for FFmpeg concat for " + v.key + ": " + err.Error())
			return err
		}
		// make sure that the file is closed
		defer func() { _ = f.Close() }()
		rlog.Trace(3, "File to store the names of the temporary video has been created: ", concatFilePath)

		// make sure that tmp directory is cleaned up
		defer cleanTmpDir(v.key)
	}

	// loop over cutlist
	for i := 0; i < len(v.cl.segs); i++ {
		// assemble filepath for output file
		if len(v.cl.segs) == 1 {
			outFilePath = cfg.CutDirPath + "/" + v.key
			outFilePath = outFilePath[0:len(outFilePath)-len(filepath.Ext(outFilePath))] + ".cut" + filepath.Ext(outFilePath)
		} else {
			// in case different partial videos needs to be created temporarily,
			// their name is set to [VIDEO.KEY].[COUNTER].extension
			outFilePath = cfg.TmpDirPath + "/" + v.key + fmt.Sprintf(".%d", i) + filepath.Ext(v.key)
		}

		// write to file list for FFmpeg concatenate
		if len(v.cl.segs) > 1 {
			if _, err = f.WriteString("file '" + outFilePath + "'\n"); err != nil {
				rlog.Error("Could not write to " + concatFilePath + ": " + err.Error())
				return err
			}
		}

		// adjust start and end of cut intervall to IDR frames
		ts, _ := getIDRFrameTime(v.filePath, v.key, v.cl.segs[i].timeStart, directUp)
		te, _ := getIDRFrameTime(v.filePath, v.key, v.cl.segs[i].timeStart+v.cl.segs[i].timeDur, directDown)

		// Create shell command for decoding
		cmd := exec.Command("ffmpeg",
			"-ss", strconv.FormatFloat(ts, 'f', 3, 64),
			"-i", v.filePath,
			"-t", strconv.FormatFloat(te-ts, 'f', 3, 64),
			"-codec", "copy",
			"-reset_timestamps", "1",
			"-async", "1",
			"-map", "0",
			"-y",
			outFilePath,
		)
		// Set up error pipe
		stderr, err = cmd.StderrPipe()
		if err != nil {
			rlog.Error("Cannot establish pipe for stderr: %v" + err.Error())
			return err
		}
		// Start the command after having set up the pipes
		if err = cmd.Start(); err != nil {
			rlog.Error("Cannot start FFmpeg: %v" + err.Error())
			return err
		}
		rlog.Trace(3, "Video has been cut with FFmpeg: ", outFilePath)

		// read command's stderr line by line and store it in a errStr for further processing
		cmdErr := bufio.NewScanner(stderr)
		for cmdErr.Scan() {
			errStr += fmt.Sprintf("%s\n", cmdErr.Text())
		}
		if err = cmd.Wait(); err != nil {
			// In case command line execution returns error, content of stderr (now contained in
			// errStr) is written into error file
			errFilePath := cfg.LogDirPath + "/" + v.key + cfg.ErrFileSuffixCut
			if errFile, e := os.Create(errFilePath); e != nil {
				rlog.Error("Cannot create \"" + errFilePath + "\": " + e.Error())
			} else {
				if _, e = errFile.WriteString(errStr); e != nil {
					rlog.Error("Cannot write into \"" + errFilePath + "\": " + e.Error())
				}
				_ = errFile.Close()
			}
		}

		// update progress
		prg += prgInterval
		progress.Set(v.key, progress.PrgActCut, prg)
	}

	// assemble
	if len(v.cl.segs) > 1 {
		// assemble filepath of output file
		outFilePath = cfg.CutDirPath + "/" + v.key
		outFilePath = outFilePath[0:len(outFilePath)-len(filepath.Ext(outFilePath))] + ".cut" + filepath.Ext(outFilePath)

		// Create shell command for concatenating
		cmd := exec.Command("ffmpeg",
			"-f", "concat",
			"-safe", "0",
			"-i", concatFilePath,
			"-codec", "copy",
			"-reset_timestamps", "1",
			"-y",
			outFilePath,
		)
		// Set up error pipe
		stderr, err = cmd.StderrPipe()
		if err != nil {
			rlog.Error("Cannot establish pipe for stderr: %v" + err.Error())
			return err
		}
		// Start the command after having set up the pipes
		if err = cmd.Start(); err != nil {
			rlog.Error("Cannot start FFmpeg concat: %v" + err.Error())
			return err
		}
		rlog.Trace(3, "Videos have been concatenated: ", outFilePath)

		// read command's stderr line by line and store it in a errStr for further processing
		cmdErr := bufio.NewScanner(stderr)
		for cmdErr.Scan() {
			errStr += fmt.Sprintf("%s\n", cmdErr.Text())
		}
		if err = cmd.Wait(); err != nil {
			// In case command line execution returns error, content of stderr (now contained in
			// errStr) is written into error file
			errFilePath := cfg.LogDirPath + "/" + v.key + cfg.ErrFileSuffixCut
			if errFile, e := os.Create(errFilePath); e != nil {
				rlog.Error("Cannot create \"" + errFilePath + "\": " + e.Error())
			} else {
				if _, e = errFile.WriteString(errStr); e != nil {
					rlog.Error("Cannot write into \"" + errFilePath + "\": " + e.Error())
				}
				_ = errFile.Close()
			}
		}
	}

	// set progress to 100%
	progress.Set(v.key, progress.PrgActCut, 100)

	return err
}
