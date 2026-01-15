package fileserver

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// addDirRecursive adds a directory and all its subdirectories to the watcher
func addDirRecursive(watcher *fsnotify.Watcher, dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			err = watcher.Add(path)
			if err != nil {
				log.Printf("Error watching directory %s: %v", path, err)
				return err
			}
			log.Printf("Watching directory: %s", path)
		}
		return nil
	})
}

// watchFiles watches for file system changes and broadcasts them
func (fs *FileServer) watchFiles() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Error creating file watcher: %v", err)
		return
	}
	defer watcher.Close()

	// Watch the configured directory
	dir := fs.config.GetFileServerDir()
	absDir, err := filepath.Abs(dir)
	if err != nil {
		log.Printf("Error getting absolute path: %v", err)
		return
	}

	// Add the directory and all subdirectories recursively
	err = addDirRecursive(watcher, absDir)
	if err != nil {
		log.Printf("Error setting up recursive watch: %v", err)
		return
	}

	// Debounce timer to avoid too many updates
	var debounceTimer *time.Timer
	debounceDuration := 500 * time.Millisecond

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// If a new directory is created, add it to the watcher
			if event.Op&fsnotify.Create == fsnotify.Create {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					err = addDirRecursive(watcher, event.Name)
					if err != nil {
						log.Printf("Error adding new directory to watch: %v", err)
					}
				}
			}

			// Reset debounce timer
			if debounceTimer != nil {
				debounceTimer.Stop()
			}

			debounceTimer = time.AfterFunc(debounceDuration, func() {
				eventType := "modified"
				if event.Op&fsnotify.Create == fsnotify.Create {
					eventType = "created"
				} else if event.Op&fsnotify.Remove == fsnotify.Remove {
					eventType = "removed"
				} else if event.Op&fsnotify.Rename == fsnotify.Rename {
					eventType = "renamed"
				}

				fileName := filepath.Base(event.Name)
				fs.BroadcastChange(fileName + " " + eventType)
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("File watcher error: %v", err)
		}
	}
}
