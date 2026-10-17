package cfgsave

import (
	"fmt"
	"os"
)

// Recover scans unfinished journals. Prepared journals never reached a write
// and are discarded. Written or unknown journals are retained when every file
// has its expected new hash; partial writes are restored from all backups.
func Recover(stateDir string) ([]string, error) {
	journals, err := readJournals(stateDir)
	if err != nil {
		return nil, fmt.Errorf("scan save journals: %w", err)
	}
	var recovered []string
	for _, journal := range journals {
		if journal.Phase == "prepared" {
			if err := removeJournal(stateDir, journal.ID); err != nil && !os.IsNotExist(err) {
				return recovered, err
			}
			continue
		}
		allNew := true
		for _, file := range journal.Files {
			data, readErr := os.ReadFile(file.Path)
			if readErr != nil || hash(data) != file.AfterHash {
				allNew = false
				break
			}
		}
		if !allNew {
			if err := rollback(journal); err != nil {
				return recovered, err
			}
			recovered = append(recovered, "config.recovered")
		}
		if err := removeJournal(stateDir, journal.ID); err != nil && !os.IsNotExist(err) {
			return recovered, err
		}
	}
	return recovered, nil
}
