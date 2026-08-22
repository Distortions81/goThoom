package main

import (
	"testing"

	"gothoom/eui"
)

func TestHandleDownloadAssetError(t *testing.T) {
	started := true
	downloadStatus = func(string) {}
	downloadProgress = func(string, int64, int64) {}
	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW}
	statusText := &eui.ItemData{}
	pb := &eui.ItemData{}
	downloadWin = eui.NewWindow()

	handleDownloadAssetError(flow, statusText, pb, func() {}, &started, "fail")

	if started {
		t.Fatalf("startedDownload not reset")
	}
	if downloadStatus != nil || downloadProgress != nil {
		t.Fatalf("download callbacks not cleared")
	}
	if len(flow.Contents) != 3 {
		t.Fatalf("expected 3 flow contents, got %d", len(flow.Contents))
	}
	retryRow := flow.Contents[2]
	if len(retryRow.Contents) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(retryRow.Contents))
	}
	if retryRow.Contents[0].Text != "Retry" || retryRow.Contents[1].Text != "Quit" {
		t.Fatalf("unexpected button labels %q %q", retryRow.Contents[0].Text, retryRow.Contents[1].Text)
	}
}
