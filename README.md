# gool

gool (Go: Online TV Recorder on Linux) ist eine Kommandozeilen-Anwendung für Linux, mit der Filme, die von [Online TV Recorder](https://www.onlinetvrecorder.com/) aufgenommen und heruntergeladen wurden, dekodiert und geschnitten werden können.

## Features

* **Dekodieren**

gool dekodiert .otrkey-Dateien unter Verwendung des [OTR-Dekoders für Linux](http://www.onlinetvrecorder.com/downloads/otrdecoder-bin-linux-Ubuntu_8.04.2-x86_64-0.4.614.tar.bz2)

* **Cutlists**

gool lädt automatisch Cutlists von [cutlist.at](http://cutlist.at) herunter

* **Schneiden**

Basierend auf den Cutlists schneidet gool Videos unter Verwendung des Tools [MKVmerge](https://mkvtoolnix.download/doc/mkvmerge.html)

* **Automatisches Handling von otrkey-Dateien**

Es ist möglich, einen dedizierter Mimetype für otrkey-Dateien anzulegen und gool als Default-Anwendung dafür zu konfigurieren.

### Einfacher Aufruf

Obwohl eine Kommandozeilen-Anwendung, lässt sich gool sehr einfach bedienen. Haben Sie z.B. gerade einige .otrkey-Dateien von Online TV Recorder heruntergeladen, so genügt der Aufruf `gool process ~/Downloads/*`, um alle Datein zu verarbeiten (d.h. dekodieren, Cutlists laden, schneiden). Mit dem dedizerten Mimetype genügt sogar ein Doppelklick auf eine otrkey-Datei und diese mit gool zu dekodieren und das Video zu schneiden.

### Massen- und Parallelverarbeitung

gool kann mit einem einzigen Aufruf viele Videodateien bearbeiten. Die Arbeitsschritte werden dabei soweit möglich und unter Berücksichtigung der Abhängigkeiten parallel durchgeführt. Der Fortgang der Verarbeitung wird über Statusmeldungen und Forschrittsbalken angezeigt.

## Installation

gool ist in der Sprache Go geschrieben und setzt die Installation von Go und den Go-Tools (unter Debian etwa sind dies die Pakete golang-go und golang-go.tools) voraus. Stellen Sie sicher, dass Sie die Umgebungsvariable `GOPATH` gesetzt haben. Es ist außerdem ratsam, den Pfad `$GOPATH/bin` in `PATH` aufzunehmen, damit Sie gool aus jedem Verzeichnis einfach aufrufen können. Stellen Sie außerdem sicher, dass die [xdg-utils](https://freedesktop.org/wiki/Software/xdg-utils/) installiert sind. Diese werden für die Erzeugung des Mimetaypes für otrkey-Dateien benötigt.

Um gool mit allen Abhängigkeiten zu laden, geben Sie

    go get github.com/mipimipi/gool

ein. Falls Sie eine Fehlermeldung wegen des Fehlens von git bekommen, installieren Sie git und führen Sie den o.g. Befehl noch einmal aus.

Führen Sie dann

    cd $GOPATH/src/github.com/mipimipi/gool
    make

aus, um das gool Binary zu erzeugen.

Ein anschließend als `root` ausgeführtes

    make install

* kopiert das gool Binary nach `/usr/bin`

* erzeugt einen dedizerten Mimetype für otrkey-Dateien

* erzeugt eine Desktop-Datei für gool

* und setzt gool als Default-Anwendung für den Mimetype.

## Verwendung

Voraussetzung für die Verwendung sind:

* [OTR-Dekoder für Linux](http://www.onlinetvrecorder.com/downloads/otrdecoder-bin-linux-Ubuntu_8.04.2-x86_64-0.4.614.tar.bz2), welcher zum Dekodieren von Videos verwendet wird.

* [MKVToolNix](https://mkvtoolnix.download/), da MKVmerge zum Schneiden verwendet wird.

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

Ist der Mimetype für otrkey-Datein eingerichtet, genügt ein Doppelklick auf solche eine Datei, um sie mit gool zu dekodieren und das Video zu schneiden.

### Verarbeitung

gool ist massenfähig, d.h. es kann bei einem Aufruf mehrere Videodateien verarbeiten. Die Verarbeitung findet nebenläufig statt. Z.B. werden für ein Video das Dekodieren sowie das Holen von Cutlisten parallel durchgeführt. Hierbei werden die Abhängigkeiten berücksichtigt, d.h. der Schneidevorgang wird erst gestartet, wenn das Video dekodiert ist und eine Cutlist geladen wurde.
Verarbeitungsschritte verschiedener Video sind unabhängig voneinander und werden ebenfalls parallel ausgeführt. Während der Verarbeitung wird der Fortschritt angezeigt. Am Ende der Verarbeitung wird eine Zusammenfassung des Ergebnissen angezeigt. 
Da gool [MKVmerge](https://mkvtoolnix.download/doc/mkvmerge.html) verwendet, um Videos zu schneiden, ist die resultierende Datei im [Matroska-Containerformat](https://de.wikipedia.org/wiki/Matroska).