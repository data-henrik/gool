// Copyright 2017 The go-xdgbasedir Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package xdgbasedir

import (
	"os"
	"path/filepath"

	"github.com/zchee/go-xdgbasedir/home"
)

var (
	localAppData      = filepath.FromSlash(os.Getenv("LOCALAPPDATA"))
	defaultDataHome   = localAppData
	defaultConfigHome = localAppData
	defaultDataDirs   = localAppData
	defaultConfigDirs = localAppData
	defaultCacheHome  = filepath.Join(localAppData, "cache")
	defaultRuntimeDir = home.Dir()
)

func dataHome() string {
	return defaultDataHome
}

func configHome() string {
	return defaultConfigHome
}

func dataDirs() string {
	return defaultDataDirs
}

func configDirs() string {
	return defaultConfigDirs
}

func cacheHome() string {
	return defaultCacheHome
}

func runtimeDir() string {
	return defaultRuntimeDir
}
