package main

import (
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// assetLoadTraceThreshold enables per-frame asset/atlas diagnostics. Frames
// that perform asset work are logged even when they are faster than the
// threshold, since Ebitengine can submit the corresponding GPU upload after
// Game.Draw returns. The threshold marks frames whose synchronous Draw work was
// itself slow.
var assetLoadTraceThreshold time.Duration

var (
	activeAssetLoadFrameTrace atomic.Pointer[assetLoadFrameTrace]
	assetLoadFrameSequence    atomic.Uint64
)

type assetLoadFrameTrace struct {
	sequence uint64
	started  time.Time

	worldNanos  atomic.Int64
	uiNanos     atomic.Int64
	viewWidth   atomic.Int64
	viewHeight  atomic.Int64
	renderScale atomic.Uint64

	prepareCalls    atomic.Uint64
	requestedSheets atomic.Uint64
	preparedSheets  atomic.Uint64
	prepareNanos    atomic.Int64
	decodeNanos     atomic.Int64
	processNanos    atomic.Int64
	uploadNanos     atomic.Int64
	atlasFactor     atomic.Uint32

	imageCreates       atomic.Uint64
	imageCreateBytes   atomic.Uint64
	imageCreateNanos   atomic.Int64
	managedAdds        atomic.Uint64
	managedAddBytes    atomic.Uint64
	managedRemoves     atomic.Uint64
	managedRemoveBytes atomic.Uint64
	unmanagedCreates   atomic.Uint64
	unmanagedBytes     atomic.Uint64
	unmanagedNanos     atomic.Int64

	unmanagedMu    sync.Mutex
	unmanagedSites map[string]uint64

	cacheClears        atomic.Uint64
	cacheClearedImages atomic.Uint64
	cacheClearedBytes  atomic.Uint64
	factorChanges      atomic.Uint64
	lastFactorChange   atomic.Uint32

	idsMu sync.Mutex
	ids   []uint16
}

func beginAssetLoadFrameTrace(started time.Time) *assetLoadFrameTrace {
	if assetLoadTraceThreshold <= 0 {
		return nil
	}
	trace := &assetLoadFrameTrace{
		sequence: assetLoadFrameSequence.Add(1),
		started:  started,
	}
	activeAssetLoadFrameTrace.Store(trace)
	return trace
}

func currentAssetLoadFrameTrace() *assetLoadFrameTrace {
	return activeAssetLoadFrameTrace.Load()
}

func (trace *assetLoadFrameTrace) finish() {
	if trace == nil {
		return
	}
	activeAssetLoadFrameTrace.CompareAndSwap(trace, nil)
	if !trace.hasAssetWork() {
		return
	}

	total := time.Since(trace.started)
	lastChange := trace.lastFactorChange.Load()
	oldFactor := uint8(lastChange >> 8)
	newFactor := uint8(lastChange)
	trace.idsMu.Lock()
	ids := append([]uint16(nil), trace.ids...)
	trace.idsMu.Unlock()
	trace.unmanagedMu.Lock()
	unmanagedSites := make([]string, 0, len(trace.unmanagedSites))
	for site, count := range trace.unmanagedSites {
		unmanagedSites = append(unmanagedSites, site+"="+strconv.FormatUint(count, 10))
	}
	trace.unmanagedMu.Unlock()
	sort.Strings(unmanagedSites)

	residentCount, residentBytes := tracedManagedImageTotals()
	log.Printf("asset frame: frame=%d total=%s slow=%t threshold=%s world=%s ui=%s view=%dx%d render_scale=%.3f atlas_factor=%d prepare_calls=%d requested_sheets=%d prepared_sheets=%d ids=%v prepare=%s decode=%s process=%s upload_submit=%s image_creates=%d image_bytes=%d image_create=%s managed_adds=%d managed_add_bytes=%d managed_removes=%d managed_remove_bytes=%d managed_residents=%d managed_resident_bytes=%d unmanaged_creates=%d unmanaged_bytes=%d unmanaged_create=%s unmanaged_sites=%v cache_clears=%d cleared_images=%d cleared_bytes=%d factor_changes=%d last_factor_change=%d->%d",
		trace.sequence,
		total.Round(time.Microsecond), total >= assetLoadTraceThreshold, assetLoadTraceThreshold,
		time.Duration(trace.worldNanos.Load()).Round(time.Microsecond),
		time.Duration(trace.uiNanos.Load()).Round(time.Microsecond),
		trace.viewWidth.Load(), trace.viewHeight.Load(), math.Float64frombits(trace.renderScale.Load()),
		trace.atlasFactor.Load(), trace.prepareCalls.Load(), trace.requestedSheets.Load(), trace.preparedSheets.Load(), ids,
		time.Duration(trace.prepareNanos.Load()).Round(time.Microsecond),
		time.Duration(trace.decodeNanos.Load()).Round(time.Microsecond),
		time.Duration(trace.processNanos.Load()).Round(time.Microsecond),
		time.Duration(trace.uploadNanos.Load()).Round(time.Microsecond),
		trace.imageCreates.Load(), trace.imageCreateBytes.Load(),
		time.Duration(trace.imageCreateNanos.Load()).Round(time.Microsecond),
		trace.managedAdds.Load(), trace.managedAddBytes.Load(), trace.managedRemoves.Load(), trace.managedRemoveBytes.Load(),
		residentCount, residentBytes,
		trace.unmanagedCreates.Load(), trace.unmanagedBytes.Load(), time.Duration(trace.unmanagedNanos.Load()).Round(time.Microsecond), unmanagedSites,
		trace.cacheClears.Load(), trace.cacheClearedImages.Load(), trace.cacheClearedBytes.Load(),
		trace.factorChanges.Load(), oldFactor, newFactor,
	)
}

func (trace *assetLoadFrameTrace) hasAssetWork() bool {
	return trace != nil && (trace.prepareCalls.Load() != 0 || trace.imageCreates.Load() != 0 ||
		trace.managedAdds.Load() != 0 || trace.managedRemoves.Load() != 0 ||
		trace.unmanagedCreates.Load() != 0 || trace.cacheClears.Load() != 0 || trace.factorChanges.Load() != 0)
}

func noteFrameUnmanagedImageCreation(width, height int, elapsed time.Duration, site string) {
	trace := currentAssetLoadFrameTrace()
	if trace == nil {
		return
	}
	trace.unmanagedCreates.Add(1)
	if width > 0 && height > 0 {
		trace.unmanagedBytes.Add(uint64(width) * uint64(height) * 4)
	}
	trace.unmanagedNanos.Add(int64(elapsed))
	site = strings.TrimPrefix(site, "gothoom/")
	trace.unmanagedMu.Lock()
	if trace.unmanagedSites == nil {
		trace.unmanagedSites = make(map[string]uint64)
	}
	trace.unmanagedSites[site]++
	trace.unmanagedMu.Unlock()
}

func (trace *assetLoadFrameTrace) setWorldContext(width, height int, renderScale float64) {
	if trace == nil {
		return
	}
	trace.viewWidth.Store(int64(width))
	trace.viewHeight.Store(int64(height))
	trace.renderScale.Store(math.Float64bits(renderScale))
}

func (trace *assetLoadFrameTrace) addWorldDuration(elapsed time.Duration) {
	if trace != nil {
		trace.worldNanos.Add(int64(elapsed))
	}
}

func (trace *assetLoadFrameTrace) addUIDuration(elapsed time.Duration) {
	if trace != nil {
		trace.uiNanos.Add(int64(elapsed))
	}
}

func noteFrameArtworkPrepare(requested, prepared, factor int, ids []uint16, total, decode, process, upload time.Duration) {
	trace := currentAssetLoadFrameTrace()
	if trace == nil {
		return
	}
	trace.prepareCalls.Add(1)
	trace.requestedSheets.Add(uint64(requested))
	trace.preparedSheets.Add(uint64(prepared))
	trace.prepareNanos.Add(int64(total))
	trace.decodeNanos.Add(int64(decode))
	trace.processNanos.Add(int64(process))
	trace.uploadNanos.Add(int64(upload))
	trace.atlasFactor.Store(uint32(factor))
	trace.idsMu.Lock()
	for _, id := range ids {
		if len(trace.ids) >= 16 {
			break
		}
		seen := false
		for _, existing := range trace.ids {
			if existing == id {
				seen = true
				break
			}
		}
		if !seen {
			trace.ids = append(trace.ids, id)
		}
	}
	trace.idsMu.Unlock()
}

func noteFrameImageCreation(width, height int, elapsed time.Duration) {
	trace := currentAssetLoadFrameTrace()
	if trace == nil {
		return
	}
	trace.imageCreates.Add(1)
	if width > 0 && height > 0 {
		trace.imageCreateBytes.Add(uint64(width) * uint64(height) * 4)
	}
	trace.imageCreateNanos.Add(int64(elapsed))
}

func noteFrameManagedImageAdd(bytes uint64) {
	trace := currentAssetLoadFrameTrace()
	if trace == nil {
		return
	}
	trace.managedAdds.Add(1)
	trace.managedAddBytes.Add(bytes)
}

func noteFrameManagedImageRemove(bytes uint64) {
	trace := currentAssetLoadFrameTrace()
	if trace == nil {
		return
	}
	trace.managedRemoves.Add(1)
	trace.managedRemoveBytes.Add(bytes)
}

func noteFrameAtlasCacheClear(images, bytes int) {
	trace := currentAssetLoadFrameTrace()
	if trace == nil {
		return
	}
	trace.cacheClears.Add(1)
	trace.cacheClearedImages.Add(uint64(images))
	trace.cacheClearedBytes.Add(uint64(bytes))
}

func noteFrameAtlasFactorChange(oldFactor, newFactor uint8) {
	trace := currentAssetLoadFrameTrace()
	if trace == nil || oldFactor == newFactor {
		return
	}
	trace.factorChanges.Add(1)
	trace.lastFactorChange.Store(uint32(oldFactor)<<8 | uint32(newFactor))
}
