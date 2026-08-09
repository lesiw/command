package command

import "lesiw.io/fs"

func toDirEntry(e fs.DirEntry) dirEntry {
	info, _ := e.Info()
	de := dirEntry{
		name: e.Name(),
		dir:  e.IsDir(),
		mode: e.Type(),
		info: toFileInfo(info),
	}
	if pather, ok := e.(fs.Pather); ok {
		de.path = pather.Path()
	}
	return de
}

func toFileInfo(fi fs.FileInfo) *fileInfo {
	return &fileInfo{
		name:  fi.Name(),
		size:  fi.Size(),
		mode:  fi.Mode(),
		mtime: fi.ModTime(),
		dir:   fi.IsDir(),
	}
}
