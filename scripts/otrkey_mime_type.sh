#!/bin/bash
#
# Copyright (C) 2017 Michael Picht
#
# This file is part of gool.
#
# gool is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# gool is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with gool. If not, see <http://www.gnu.org/licenses/>.
#

# otrkey_mime_type.sh Installs mime type for .otrkey files and sets gool
# as default application

# install mime type
xdg-mime install --novendor otrkey_mime.xml 

# set default application
xdg-mime default gool.desktop application/x-onlinetvrecorder