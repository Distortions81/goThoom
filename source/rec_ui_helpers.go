package main

// recordingMovie arms recording to begin on the first draw-state after
// connecting. This is toggled via the Record/STOP button when disconnected.
var recordingMovie bool

// updateRecordButton updates the toolbar record button label and theme based on
// whether we're recording, armed to record, or playing back a movie.
func updateRecordButton() {
	if recordBtn == nil {
		return
	}
	if (playingMovie && !setupWizardPreviewActive) || recordingMovie {
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
