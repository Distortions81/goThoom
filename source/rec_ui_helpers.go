package main

// updateRecordButton updates the toolbar record button label and theme based on
// whether we're recording, armed to record, or playing back a movie.
func updateRecordButton() {
	if recordBtn == nil {
		return
	}
	if (playingMovie && !setupWizardPreviewActive) || sessionRecordingRequested(selectedAppSession()) {
		recordBtn.Text = "STOP"
		setMaterialButtonIcon(recordBtn, "stop")
	} else {
		recordBtn.Text = "Record"
		setMaterialButtonIcon(recordBtn, "fiber_manual_record")
	}
	// Force re-render of the button and toolbar window
	recordBtn.Dirty = true
	refreshToolbar()
}
