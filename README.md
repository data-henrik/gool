# gool

gool (Go: Online TV Recorder on Linux) is a command line application for Linux. It allows decoding and cutting of videos that have been recorded by [Online TV Recorder](https://www.onlinetvrecorder.com/).

## Features

* **Decoding**

gool decodes otrkey files by using [OTR Decoder for Linux](http://www.onlinetvrecorder.com/downloads/otrdecoder-bin-linux-Ubuntu_8.04.2-x86_64-0.4.614.tar.bz2)

* **Cutlists**

gool automatically loads cutlists from [cutlist.at](http://cutlist.at)

* **Cutting**

Based on the cutlists, gool cuts videos by using [MKVmerge](https://mkvtoolnix.download/doc/mkvmerge.html)

* **Automated handling of otrkey files**

It's possible to create a dedicated mime type for otrkey files. gool can be defined as default application for it.

### Simple usage

Though being a command line application, the usage of gool is quite simple. If, for instance, you have downloaded some otrkey files from Online TV Recorder, the command `gool process ~/Downloads/*` processes all files (i.e., the y are decoded, cutlists are loaded and they are cut). With the dedicated mime type, it's even simpler: A double click on an otrkey file starts gool.

### Mass and parallel processing

With one gool can process many video files. The different steps are executed in parallel as far as possible and by taking care of the dependencies. The progress is displayed as status messages and progress bars.

## Installation

### Manual installation

gool is written in Golang and thus requires the installation of Go with it's corresponding tools (in Debian these are the packages **golang-go** and **golang-go.tools**). Make sure that you've set the environment variable `GOPATH` accordingly. Make sure that **git** is installed.

To download gool and all dependencies, enter

    go get github.com/mipimipi/gool

After that, build gool by executing

    cd $GOPATH/src/github.com/mipimipi/gool
    make

Finally, execute

    make install

as `root` to

* copy the gool binary to `/usr/bin`

* create a dedicated mime type for otrkey files

* create a desktop file for gool.

Since gool is the only application that can process files of the new mime type, it should now be called automatically if you double click on an otrkey file.

### Installation with package managers

For Arch Linux (and other Linux ditros, that can install packages from the Arch User Repository) there's a [gool package in AUR](https://aur.archlinux.org/packages/gool-git/).

## Usage

Prerequistes:

* [OTR Decoder for Linux](http://www.onlinetvrecorder.com/downloads/otrdecoder-bin-linux-Ubuntu_8.04.2-x86_64-0.4.614.tar.bz2) is required to decode videos.

* [MKVToolNix](https://mkvtoolnix.download/) is required to cut videos.

gool is controlled via sub commands:

        help     # help
        list     # Lists the retrieved videos files and its status
        process  # Processed the retrieved (e.g. decodes and cuts them)

### Configuration

During the first call gool requires some inputs for configuraion. This data is stored `gool.conf`. This configuration file is located in the user specific configuration directory of your operation system (e.g.`~/.config`). It can be changed with a text editor.

### Directories

gool requires a working directory (e.g. `~/Videos/OTR`). In this directory, the sub directories `Encoded`, `Decoded` and `Cut` are created. They'll store the video files depending on its processing status. `Cut`, for instance, contains the video files that have been cut, `Decoded` the decoded and uncut files (it can happen that a video can be decoded but cannot be cut because cutlists donÄt exist yet). If videos have been cut, the uncut version is stored in the sub directory `Decoded/Archive`to allow users to repeat the cutting if they are not hapoy with the result. Moreover, a sub directory `log` is being created. It contains log files if errors occurred.

### Call

The command `gool list` lists all video files, that are stored in the working directory or its sub directories, incl. its processing status. `gool process` starts processing of videos. In both cases, additional file paths can be passed to the command. These files are considered by gool as well. The command `gool process ~/Downloads/*` would process videos located in the downloads folder (in addition to the videos stored in the working directoy and its sub directories). The flag `--log [file]` after on of the sub commands switches on logging.

If the mime type for otrkey files has been created, a double click on such a file is sufficient to decode an cut it with gool.

### Processing

gool is capable to process many videos in one call. Processing happens in a concurrent way. For one video, decoding and fetching of cutlists is done parallel. Dependencies are being taken care of. I.e. the cutting step will only be started after the decoding and the loading of cutlists has been done. Processing steps of different videos are independend of each other and thus are executed in parallel as well. During processing, progress is displayed. After processing has ended, the result will be shown as summary.

Since gool uses [MKVmerge](https://mkvtoolnix.download/doc/mkvmerge.html) to cut videos, the resulting files has the [Matroska container format](https://de.wikipedia.org/wiki/Matroska) (.mkv).