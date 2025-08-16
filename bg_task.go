package main

import "time"

func runBackgroundTasks() {
	go func() {
		for {
			time.Sleep(time.Second)
			if debugWin != nil && debugWin.IsOpen() {
				updateDebugStats()
			}
		}
	}()

	go func() {
		for {
			//TODO CLEANUP -- Replace with tickers
			time.Sleep(time.Millisecond * 200)

			if inventoryDirty {
				updateInventoryWindow()
				updateHandsWindow()
				inventoryDirty = false
			}

			if playersDirty {
				updatePlayersWindow()
				playersDirty = false
			}
			if syncWindowSettings() {
				settingsDirty = true
			}
			if settingsDirty && qualityPresetDD != nil {
				qualityPresetDD.Selected = detectQualityPreset()
			}
			if time.Since(lastSettingsSave) >= 5*time.Second {
				if settingsDirty {
					saveSettings()
					settingsDirty = false
				}
				lastSettingsSave = time.Now()
			}

			// Periodically persist players if there were changes.
			if time.Since(lastPlayersSave) >= 5*time.Second {
				if playersDirty || playersPersistDirty {
					savePlayersPersist()
					playersPersistDirty = false
				}
				lastPlayersSave = time.Now()
			}

			// Ensure the movie controller window repaints at least once per second
			// while open, even without other UI events.
			if movieWin != nil && movieWin.IsOpen() {
				if time.Since(lastMovieWinTick) >= time.Second {
					lastMovieWinTick = time.Now()
					movieWin.Refresh()
				}
			}

		}
	}()
}
