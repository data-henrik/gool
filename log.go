// Copyright (C) 2018 Michael Picht
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

// log.go implements some wrapper functionality for logging

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

// text formatting structure for gool
type goolTextFormatter struct{}

// helper function to check existence of file
func exists(fp string) bool {
	if _, err := os.Stat(fp); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// Format print one log line in gool specific format
func (f *goolTextFormatter) Format(entry *log.Entry) ([]byte, error) {
	var b *bytes.Buffer

	// initialize buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	// write log level
	b.WriteString(fmt.Sprintf("[%-7s]:", entry.Level.String()))

	// write custom data fields
	for _, value := range entry.Data {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		stringVal, ok := value.(string)
		if !ok {
			stringVal = fmt.Sprint(value)
		}
		b.WriteString("[" + stringVal + "]")
	}

	// write log message
	b.WriteByte(' ')
	b.WriteString(entry.Message)

	// new line
	b.WriteByte('\n')

	return b.Bytes(), nil
}

// createLogger creates and initializes the logger for gool
func createLogger(logFile string) {
	var (
		f   *os.File
		err error
	)

	// if no log file was specified at command line: Set logger output to Nirwana and do nothing else
	if logFile == "" {
		log.SetOutput(ioutil.Discard)
		return
	}

	// get absolute filepath for log file
	fp, _ := filepath.Abs(logFile)

	// delete log file if it already exists
	if exists(fp) {
		_ = os.Remove(fp)
	}

	// create log file
	if f, err = os.Create(fp); err != nil {
		fmt.Printf("Log file could not be created/opened: %v", err)
		return
	}

	// set log file as output for logging
	log.SetOutput(f)

	// log all messages
	log.SetLevel(log.DebugLevel)

	// set custom formatter
	log.SetFormatter(new(goolTextFormatter))
}
