# gool

gool (Go: Online TV Recorder on Linux) ist eine Kommandozeilen-Anwendung für Linux, mit der Filme, die von [Online TV Recorder](https://www.onlinetvrecorder.com/) aufgenommen und heruntergeladen wurden, dekodiert und geschnitten werden können.

## Features

* **Dekodieren**

gool dekodiert .otrkey-Dateien unter Verwendung des [OTR-Dekoders für Linux](http://www.onlinetvrecorder.com/downloads/otrdecoder-bin-linux-Ubuntu_8.04.2-x86_64-0.4.614.tar.bz2)

* **Cutlists**

gool lädt automatisch Cutlists von [cutlist.at](http://cutlist.at) herunter

* **Schneiden**

Basierend auf den Cutlists schneidet gool Videos unter Verwendung des Tools [FFmpeg](https://ffmpeg.org/)

### Einfacher Aufruf

Obwohl eine Kommandozeilen-Anwendung, lässt sich gool sehr einfach bedienen. Haben Sie z.B. gerade einige .otrkey-Dateien von Online TV Recorder heruntergeladen, so genügt der Aufruf `gool process ~/Downloads/*`, um alle Datein zu verarbeiten (d.h. dekodieren, Cutlists laden, schneiden).

### Massen- und Parallelverarbeitung

gool kann mit einem einzigen Aufruf viele Videodateien bearbeiten. Die Arbeitsschritte werden dabei soweit möglich und unter Berücksichtigung der Abhängigkeiten parallel durchgeführt. Der Fortgang der Verarbeitung wird über Statusmeldungen und Forschrittsbalken angezeigt.

## Installation

gool ist in der Sprache Go geschrieben und setzt die Installation von Go und den Go-Tools (unter Debian etwa sind dies die Pakete golang-go und golang-go.tools) voraus. Stellen Sie sicher,dass Sie die Umbegungsvariablen GOPATH und GOBINgesetzt haben. Es ist außerdem ratsam, den Pfad $GOBIN in PATH aufzunehmen, damit Sie gool aus jedem Verzeichnis einfach aufrufen können.

Um gool mit allen Abhängigkeiten zu laden, geben Sie

    `go get github.com/mipimipi/gool`

ein. Falls Sie eine Fehlermeldung wegen des Fehlens von git bekommen, installieren Sie git und führen Sie den o.g. Befehl noch einmal aus.

Mit

    `go install github.com/mipimipi/gool`

installieren Sie gool.

## Verwendung

Voraussetzung für die Verwendung sind:

* [OTR-Dekoder für Linux](http://www.onlinetvrecorder.com/downloads/otrdecoder-bin-linux-Ubuntu_8.04.2-x86_64-0.4.614.tar.bz2), welcher zum Dekodieren von Videos verwendet wird.

* [FFmpeg](https://ffmpeg.org/), welches zum Schneiden verwendet wird.

gool wir über Unterkommandos gesteuert:

        help     # Hilfe
        list     # Listet die gefundenen Videodateien samt ihres Status
        process  # Verarbeitet die gefundenen Videodateien (d.h. dekodiert und schneidet sie)

### Konfiguration

Beim ersten Aufruf fragt gool einige Konfigurationangaben ab, die in der Datei `gool.conf` abgespeichert werden. `gool.conf` befindet sich im benutzerspezifischen Konfigurationsverzeichnis Ihres Betriebssystems (also etwa in `~/.config`) und kann mit einem Texteditor geändert werden.

### Verzeichnis

gool setzt die Einrichtung eines Arbeitsverzeichnisses voraus (z.B. `~/Videos/OTR`). In diesem Verzeichnis werden die Unterverzeichnisse `Encoded`, `Decoded` und `Cut` erzeugt, die die Videodateien in den entsprechenden Bearbeitungsschritten aufnehmen. `Cut` enthält beispielsweise die fertig geschnittenen Videodateien, `Decoded` die dekodierten aber ungeschnittenen Dateien (z.B. kann es sein, dass eine Videodatei zwar dekodiert aber nicht geschnitten werden konnte, da noch keine Cutlist vorhanden ist). Darüber hinaus wird das Unterverzeichnis `log` angelegt, in welchem im Fehlerfall Log-Dateien abgelegt werden.

### Aufruf

Der Aufruf `gool list` listet alle Videodateien, die im Arbeitsverzeichnis und den o.g. Unterverzeichnissen enthalten sind, samt ihres Verarbeitungsstatus. `gool process` stößt die Verarbeitung der Dateien an. In beiden Fällen können zusätzlich Dateipfade als Parameter übergeben werden. Diese Dateien werden dann ebenfalls durch gool verarbeitet. Ein Aufruf von `gool process ~/Downloads/*` würde also auch die Videos berücksichtigen, die sich im Downloads-Ordner befinden (etwa weil Sie gerade .otrkey-Dateien von Online TV Recorder heruntergeladen haben).

### Verarbeitung

gool ist massenfähig, d.h. es kann bei einem Aufruf mehrere Videodateien verarbeiten. Die Verarbeitung findet nebenläufig statt. Z.B. werden für ein Video das Dekodieren sowie das Holen von Cutlisten parallel durchgeführt. Hierbei werden die Abhängigkeiten berücksichtigt, d.h. der Schneidevorgang wird erst gestartet, wenn das Video dekodiert ist und eine Cutlist geladen wurde.
Verarbeitungsschritte verschiedener Video sind unabhängig voneinander und werden ebenfalls parallel ausgeführt. Während der Verarbeitung wird der Fortschritt angezeigt. Am Ende der Verarbeitung wird eine Zusammenfassung des Ergebnissen angezeigt.