// Copyright (C) 2017 Michael Picht
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

// cfg.go implements the logic that is needed for the configuration
// of gool and provides some constants:
// - For directory names (actually sub sirectorties of the gool working
//   directory)
// - For file name suffices (e.g. for error files)
// - For names of command line programs that are called by gool (e.g.
//   OTR decoder or FFmpeg)
//
// The configuration is stored in the global variable cfg.
// getFromFile is the main function. It reads the configuration from the
// file gool.conf (which is stored in the user condig directory of the OS).
// If the file does not contain all config values, the user is requested
// to enter them and gool.conf is updated accordingly.
// In addition, gool.conf could also be edited manually with any text editor.
// It is in INI format.

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/go-ini/ini"
	"github.com/romana/rlog"
	"github.com/zchee/go-xdgbasedir"
)

// Constants for gool configuration
const (
	cfgFileName       = "gool.conf"
	cfgSectionGeneral = "general"
	cfgSectionDecode  = "decode"
	cfgSectionCut     = "cut"
	cfgKeyWrkDir      = "working_dir"
	cfgKeyNumCPUs     = "num_cpus_for_gool"
	cfgKeyOTRDecDir   = "otr_decoder_dir"
	cfgKeyOTRUsername = "otr_username"
	cfgKeyOTRPassword = "otr_password"
	cfgKeyCLSUrl      = "cutlist_server_url"
)

// Constants for directory names
const (
	subDirNameEnc = "Encoded"
	subDirNameDec = "Decoded"
	subDirNameCut = "Cut"
	subDirNameLog = "log"
	subDirNameTmp = "tmp"
)

// Constants for error file suffices
const (
	errFileSuffixDec = ".decode.error"
	errFileSuffixCut = ".cut.eror"
)

// Constants related to cli commands or programs
const (
	otrDecoderName = "otrdecoder"
	ffmpegName     = "ffmpeg"
)

// config contains the content read from the gool config file
type config struct {
	wrkDirPath    string // working dir for gool
	encDirPath    string // dir for encoded videos
	decDirPath    string // dir for decoded videos
	cutDirPath    string // dir for cut videos
	logDirPath    string // dir for log files
	tmpDirPath    string // dir for temporary files
	numCpus       int    // number of CPUs that gool is allowed to use
	otrDecDirPath string // directory where otrdecoder is stored
	otrUsername   string // username for OTR
	otrPassword   string // password for OTR
	clsURL        string // URL of custlist server
	doCleanUp     bool   // delete files that are no longer needed
}

// global config structure
var cfg config

// Function type to abstract functions that retrieve config values from user input
type getFromKeyboard func() (string, error)

func init() {
	cfg.doCleanUp = true
}

// Checks if the directory dirName exists. Depending on the parameter doCreate, the directory
// is either created or an error is returned.
func checkDirPath(dir string, doCreate bool) error {
	var err error

	if _, err = os.Stat(dir); err != nil {
		// if dir doesn't exist ...
		if os.IsNotExist(err) {
			// ... create it, if doCreate is true ...
			if doCreate {
				rlog.Trace(1, dir+" doesn't exist: Create it")
				if err = os.MkdirAll(dir, 0755); err != nil {
					fmt.Printf("Das Verzeichnis %s kann nicht erzeugt werden: %v\n", dir, err)
				}
			} else {
				// ... otherwise: exit
				rlog.Trace(1, dir+" doesn't exist: Exit")
				fmt.Printf("Das Verzeichnis %s existiert nicht: %v\n", dir, err)
			}
		} else {
			fmt.Printf("Fehler beim Zugriff auf das Verzeichnis %s: %v\n", dir, err)
		}
	}

	return err
}

// Asks the user to enter the url of the cutlist server
func getCLSUrlFromKeyboard() (string, error) {
	var (
		err   error
		input string
	)

	fmt.Print("\nGeben Sie Ihr URL des Cutlist-Servers ein [http://cutlist.at/]: ")
	if _, err = fmt.Scanln(&input); err != nil {
		return "", fmt.Errorf("Fehler bei der Eingabe der URL des Cutlist-Server: %v", err)
	}
	if input == "" {
		// set default
		input = "http://cutlist.at/"
	} else if !strings.HasSuffix(input, "/") {
		// make sure that URL ends with slash
		input += "/"
	}

	return input, err
}

// getFromFile reads the gool configuration from the file $XDG_CONFIG_HOME/gool.conf
// and stores the configuration values in the attributes of instance of type config.
// - If $XDG_CONFIG_HOME is not set, ~/.config will be used as default instead.
// - If gool.conf is not yet existing, it will be created (incl. the directories
//   along the path (if necessary).
// - If the file gets created, it is filled with default values.
// - Only if the config file neither is existing nor can be created, the function
//   exits with error.
func (cfg *config) getFromFile() error {
	var (
		err     error
		cfgFile *ini.File

		// Default for config home. Is used if $XDG_CONFIG_HOME is not set
		cfgHomeDirPathDefault = os.Getenv("HOME") + "/.config"

		// Indicates whether the config file has changed and thus needs to be saved
		hasChanged = false

		// Variables to store content of config file: Sections and keysy
		sec *ini.Section
		key *ini.Key
	)

	// Get configuration directory via the environment variable $XDG_CONFIG_HOME.
	// If $XDG_CONFIG_HOME is empty, the path "~/.config" is used as default
	cfgHomeDirPath := xdgbasedir.ConfigHome()
	if cfgHomeDirPath == "" {
		rlog.Trace(1, "$XDG_CONFIG_HOME is not set, use "+cfgHomeDirPathDefault)
		cfgHomeDirPath = cfgHomeDirPathDefault
	}
	rlog.Trace(3, "Config home directory: "+cfgHomeDirPath)

	// Check if the config home directory is existing. Create it (and its parents) if necessary
	if err = checkDirPath(cfgHomeDirPath, true); err != nil {
		return err
	}

	// Assemble the name of the gool configuration file.
	cfgFilepath := cfgHomeDirPath + "/" + cfgFileName
	rlog.Trace(3, "Config file name: "+cfgFilepath)

	// Config file is tried to be loaded by go-ini package.
	// If that's not possible, it's created and filled with default values.
	if cfgFile, err = ini.InsensitiveLoad(cfgFilepath); err != nil {
		rlog.Trace(1, "Config file is not existing. Go forward with empty config")
		cfgFile = ini.Empty()
		hasChanged = true
	}

	// Get GENERAL section. If it doesn't exist: Create it.
	if sec, err = getSection(cfgFile, cfgSectionGeneral, &hasChanged); err != nil {
		return err
	}

	// Read WORKING_DIR key. If it doesn't exist: Create it.
	if key, err = getKey(cfgFile, sec, cfgKeyWrkDir, getWrkDirPathFromKeyboard, &hasChanged); err != nil {
		return err
	}
	cfg.wrkDirPath = key.Value()
	// determine sub directory paths
	if cfg.encDirPath, err = getSubDirPath(subDirNameEnc); err != nil {
		return err
	}
	if cfg.decDirPath, err = getSubDirPath(subDirNameDec); err != nil {
		return err
	}
	if cfg.cutDirPath, err = getSubDirPath(subDirNameCut); err != nil {
		return err
	}
	if cfg.logDirPath, err = getSubDirPath(subDirNameLog); err != nil {
		return err
	}
	if cfg.tmpDirPath, err = getSubDirPath(subDirNameTmp); err != nil {
		return err
	}

	// Read NUM_CPUS_FOR_GOOL key. If it doesn't exist: Create it.
	if key, err = getKey(cfgFile, sec, cfgKeyNumCPUs, getNumCPUsFromKeyboard, &hasChanged); err != nil {
		return err
	}
	cfg.numCpus, _ = strconv.Atoi(key.Value())

	// Get DECODE section. If it doesn't exist: Create it.
	if sec, err = getSection(cfgFile, cfgSectionDecode, &hasChanged); err != nil {
		return err
	}

	// Read OTR_DECODER_DIR key. If it doesn't exist: Create it.
	if key, err = getKey(cfgFile, sec, cfgKeyOTRDecDir, getOTRDecDirPathFromKeyboard, &hasChanged); err != nil {
		return err
	}

	cfg.otrDecDirPath = key.Value()

	// Read OTR_USERNAME key. If it doesn't exist: Create it.
	if key, err = getKey(cfgFile, sec, cfgKeyOTRUsername, getOTRUsernameFromKeyboard, &hasChanged); err != nil {
		return err
	}
	cfg.otrUsername = key.Value()

	// Read OTR_PASSWORD key. If it doesn't exist: Create it.
	if key, err = getKey(cfgFile, sec, cfgKeyOTRPassword, getOTRPasswordFromKeyboard, &hasChanged); err != nil {
		return err
	}
	cfg.otrPassword = key.Value()

	// Get CUT section. If it doesn't exist: Create it..
	if sec, err = getSection(cfgFile, cfgSectionCut, &hasChanged); err != nil {
		return err
	}

	// Read CLS_URL key. If it doesn't exist: Create it.
	if key, err = getKey(cfgFile, sec, cfgKeyCLSUrl, getCLSUrlFromKeyboard, &hasChanged); err != nil {
		return err
	}
	cfg.clsURL = key.Value()

	// if entries of the configuration file have been changed is needs to be saved
	if hasChanged {
		rlog.Trace(3, "Config has been changed and needs to be saved")
		if err = cfgFile.SaveTo(cfgFilepath); err != nil {
			return fmt.Errorf("Konfigurationsdatei %s kann nicht gesichert werden: %v", cfgFilepath, err)
		}
		rlog.Trace(3, "Config has been saved")
		// Change file mode. As a password is stored in there, only the owner should be able to read it
		if err = os.Chmod(cfgFilepath, 0600); err != nil {
			return fmt.Errorf("chmod 0600 konnte für %s nicht ausgeführt werden: %v", cfgFilepath, err)
		}
		rlog.Trace(3, "Mode of config file changed to 0600")
	}

	return err
}

// Checks if a key exists in ini file. It it doesn't, it's be created. Therefore,
// function f is called to ask the user for the key value. In case of success,
// the key is returned. In addition, a flag is returned that indicates whether file
// has been changed or not
func getKey(iniFile *ini.File, sec *ini.Section, keyName string, f getFromKeyboard, hasChanged *bool) (*ini.Key, error) {
	var (
		val string
		err error
	)
	keyExists := false
	valExists := false

	// Try to read key from ini file.
	if sec.HasKey(keyName) {
		keyExists = true
		if sec.Key(keyName).Value() != "" {
			valExists = true
		}
	}

	// If key exists and has a value: Get key and return
	if keyExists && valExists {
		rlog.Trace(3, "["+sec.Name()+"]."+keyName+"="+sec.Key(keyName).Value())

		return sec.Key(keyName), nil
	}

	// Configuration needs to be saved
	*hasChanged = true

	// Get key value from user input by calling function f
	rlog.Trace(2, "Key "+keyName+" is not filled: Ask user for value")
	if val, err = f(); err != nil {
		rlog.Trace(1, "Error during user input for key "+keyName+": "+err.Error())

		return nil, fmt.Errorf("Fehler bei Benutzereingabe für %s: %v", keyName, err)
	}

	// If key exists ...
	if keyExists {
		// ... set key value
		sec.Key(keyName).SetValue(val)
	} else {
		// ... otherwise create key and set value
		if _, err = sec.NewKey(keyName, val); err != nil {
			err = fmt.Errorf("Key %s kann nicht erzeugt werden: %v", keyName, err)
		}
	}

	rlog.Trace(3, "["+sec.Name()+"]."+keyName+"="+sec.Key(keyName).Value())

	return sec.Key(keyName), err
}

// Asks the user to enter the number of cpus to be used for gool
func getNumCPUsFromKeyboard() (string, error) {
	var (
		maxCPUs = runtime.NumCPU()
		numCPUs = 1
		input   string
		inputOK bool
		err     error
	)

	// if the system only has 1 CPU, no user input is necessary
	if maxCPUs == 1 {
		return "1", nil
	}

	// Ask the user as long as the input is OK
	for !inputOK {
		fmt.Printf("\nWieviele CPUs dürfen von gool genutzt werden (1..%d)? ", maxCPUs)
		if _, err = fmt.Scanln(&input); err != nil {
			fmt.Println("Fehler bei der Eingabe: " + err.Error())
			continue
		}
		// check if input is a number
		if re, _ := regexp.Compile(`\d+`); !re.MatchString(input) {
			fmt.Printf("Geben Sie eine Zahl zwischen 1 und %d ein.\n", maxCPUs)
			continue
		}
		numCPUs, _ = strconv.Atoi(input)
		// check if input doesn't exceed max number of CPU's
		if (numCPUs < 1) || (numCPUs > maxCPUs) {
			fmt.Printf("Geben Sie eine Zahl zwischen 1 und %d ein.\n", maxCPUs)
			continue
		}
		inputOK = true
	}

	return strconv.Itoa(numCPUs), err
}

// Asks the user to enter the path to the OTR decoder
func getOTRDecDirPathFromKeyboard() (string, error) {
	var (
		err           error
		otrDecDirPath string
		inputOK       bool
	)

	// Ask the user as long as the input is OK
	for !inputOK {
		fmt.Print("\nGeben Sie den Pfad zum otrdecoder ein: ")
		if _, err = fmt.Scanln(&otrDecDirPath); err != nil {
			fmt.Println("Fehler bei der Eingabe: " + err.Error())
			continue
		}
		// Check if the OTR decoder directory is existing
		if err = checkDirPath(otrDecDirPath, false); err != nil {
			continue
		}
		// Check if the directory contains the otrdecoder file. If not write an error and
		// ask again for the path
		otrDecFilepath := otrDecDirPath + "/" + otrDecoderName
		if _, err = os.Stat(otrDecFilepath); err != nil {
			if os.IsNotExist(err) {
				rlog.Trace(1, otrDecDirPath+" doesn't contain otrdecoder, ask again")
				fmt.Printf("%s enthält nicht otrdecoder: %v\n", otrDecDirPath, err)
			} else {
				rlog.Trace(1, "Could not access "+otrDecFilepath)
				fmt.Printf("Auf %s konnte nicht zugegriffen werden: %v\n", otrDecFilepath, err)
			}
			continue
		}
		inputOK = true
	}

	return otrDecDirPath, err
}

// Asks the user to enter the password for OTR
func getOTRPasswordFromKeyboard() (string, error) {
	var (
		err   error
		input string
	)

	fmt.Print("\nGeben Sie Ihr OTR-Password ein: ")
	if _, err = fmt.Scanln(&input); err != nil {
		return "", fmt.Errorf("Fehler bei der Eingabe des OTR-Passwords: %v", err)
	}

	return input, err
}

// Asks the user to enter the username for OTR
func getOTRUsernameFromKeyboard() (string, error) {
	var (
		err   error
		input string
	)

	fmt.Print("\nGeben Sie Ihren OTR-Benutzernamen ein: ")
	if _, err = fmt.Scanln(&input); err != nil {
		return "", fmt.Errorf("Fehler bei der Eingabe des OTR-Benutzernamens: %v", err)
	}

	return input, err
}

// Checks if a section exists in ini file. It it doesn't, it's be created.
// In case of success, section is returned. In addition, a flag is returned that
// indicates whether file has been changed or not
func getSection(iniFile *ini.File, secName string, hasChanged *bool) (*ini.Section, error) {
	var (
		sec *ini.Section
		err error
	)

	//Try to read section from ini file
	if sec, err = iniFile.GetSection(secName); err != nil {
		rlog.Trace(1, "Section "+secName+" does not exist: Create it")

		// if it doesn't exist: create it
		if sec, err = iniFile.NewSection(secName); err == nil {
			*hasChanged = true
		} else {
			err = fmt.Errorf("Section %s kann nicht erzeugt werden: %v", secName, err)
		}
	}

	return sec, err
}

// Checks if sub directories of the working directory ("Encoded", "Decoded" or "Cut")
// exists. If not, it's created.
// The function returns the full path of the directory.
func getSubDirPath(subDirName string) (string, error) {
	subDirPath := cfg.wrkDirPath + "/" + subDirName
	err := checkDirPath(subDirPath, true)

	return subDirPath, err
}

// Asks the user to enter working directory. If this directory doesn't exist, it is
// being created. Also the sub directories are created
func getWrkDirPathFromKeyboard() (string, error) {
	var (
		err        error
		inputOK    bool
		wrkDirPath string
	)

	for !inputOK {
		fmt.Print("\nGeben Sie das Arbeitsverzeichnis für gool ein (falls es nicht existiert, wird es angelegt): ")
		if _, err = fmt.Scanln(&wrkDirPath); err != nil {
			fmt.Println("Fehler bei der Eingabe: " + err.Error())
			continue
		}
		// Check if the working directory is existing. Create it (and its parents) if necessary
		if err = checkDirPath(wrkDirPath, true); err != nil {
			continue
		}
		inputOK = true
	}

	return wrkDirPath, err
}
