package clmusicref

/*
#cgo CFLAGS: -std=c11 -O2
#include <stdlib.h>
#include <string.h>

typedef struct { int start_ms; int dur_ms; int keys[8]; int key_count; } ref_event_t;

static int note_offset(int ch) {
    switch (ch) {
        case 'c': return 0; case 'd': return 2; case 'e': return 4;
        case 'f': return 5; case 'g': return 7; case 'a': return 9; case 'b': return 11;
    }
    return -1;
}

// Minimal reference parser focusing on tempo handling, notes/rests/ties.
// Supported: a..g/A..G with optional '#' '.' '_' and 1..9 duration,
// rests 'p' with optional 1..9, inline tempo '@', '@+N', '@-N', '@N'.
// Octaves and chords are ignored for now; keys are computed in a fixed octave 4.
int parse_ref(const char* s, int base_tempo, ref_event_t** out, int* out_len) {
    if (!s) return -1;
    int tempo = base_tempo > 0 ? base_tempo : 120;
    int i = 0; int n = (int)strlen(s);
    int start_ms = 0;
    int cap = 64; int len = 0;
    ref_event_t* evs = (ref_event_t*)calloc(cap, sizeof(ref_event_t));
    if (!evs) return -1;

    while (i < n) {
        char c = s[i];
        if (c==' '||c=='\t'||c=='\n' || c=='\r') { i++; continue; }
        if (c=='@') {
            i++;
            int sign = 0; if (i<n && (s[i]=='+'||s[i]=='-')) { sign = s[i]; i++; }
            int val = 0; while (i<n && s[i]>='0' && s[i]<='9') { val = val*10 + (s[i]-'0'); i++; }
            if (val == 0) { tempo = 120; }
            else if (sign=='+') tempo += val; else if (sign=='-') tempo -= val; else tempo = val;
            if (tempo < 1) tempo = 1;
            continue;
        }
        if (c=='p') {
            i++;
            double beats = 2.0; // lowercase default
            if (i<n && s[i]>='1' && s[i]<='9') { beats = (double)(s[i]-'0'); i++; }
            int dur_ms = (int)((beats/4.0) * (60000.0/(double)tempo) + 0.5);
            start_ms += dur_ms;
            continue;
        }
        int is_note = 0; int base = -1; int upper = 0;
        if ((c>='a' && c<='g') || (c>='A' && c<='G')) {
            is_note = 1; upper = (c>='A' && c<='G');
            base = note_offset(upper ? (c-'A'+'a') : c);
            i++;
        }
        if (!is_note) { i++; continue; }
        // modifiers
        int pitch = base + (4+1)*12; // octave 4
        double beats = upper ? 4.0 : 2.0;
        int tied = 0;
        while (i<n) {
            char m = s[i];
            if (m=='#') { pitch++; i++; }
            else if (m=='.') { pitch--; i++; }
            else if (m=='_') { tied = 1; i++; }
            else if (m>='1' && m<='9') { beats = (double)(m-'0'); i++; }
            else break;
        }
        int dur_ms_full = (int)((beats/4.0) * (60000.0/(double)tempo) + 0.5);
        int gap = (int)((1500.0/(double)tempo) + 0.5);
        int note_ms = dur_ms_full - (tied ? 0 : gap); if (note_ms < 0) note_ms = 0;

        // merge into previous if tied and same key and contiguous
        if (tied && len>0) {
            ref_event_t* prev = &evs[len-1];
            if (prev->key_count==1 && prev->keys[0]==pitch && prev->start_ms + dur_ms_full == start_ms + dur_ms_full) {
                // extend previous by full duration
                prev->dur_ms += dur_ms_full;
            } else {
                if (len==cap) { cap*=2; evs = (ref_event_t*)realloc(evs, cap*sizeof(ref_event_t)); }
                evs[len].start_ms = start_ms;
                evs[len].dur_ms = note_ms;
                evs[len].keys[0] = pitch; evs[len].key_count = 1;
                len++;
            }
        } else {
            if (len==cap) { cap*=2; evs = (ref_event_t*)realloc(evs, cap*sizeof(ref_event_t)); }
            evs[len].start_ms = start_ms;
            evs[len].dur_ms = note_ms;
            evs[len].keys[0] = pitch; evs[len].key_count = 1;
            len++;
        }
        start_ms += dur_ms_full;
    }
    *out = evs; *out_len = len; return 0;
}
*/
import "C"

import "unsafe"

// ParseRef calls the C reference parser for a tune string at baseTempo.
func ParseRef(s string, baseTempo int) ([]RefEvent, error) {
    cs := C.CString(s)
    defer C.free(unsafe.Pointer(cs))
    var out *C.ref_event_t
    var outLen C.int
    if rc := C.parse_ref(cs, C.int(baseTempo), &out, &outLen); rc != 0 {
        return nil, nil
    }
    defer C.free(unsafe.Pointer(out))
    n := int(outLen)
    res := make([]RefEvent, n)
    evs := (*[1 << 28]C.ref_event_t)(unsafe.Pointer(out))[:n:n]
    for i := 0; i < n; i++ {
        e := evs[i]
        keys := make([]int, int(e.key_count))
        for k := 0; k < int(e.key_count); k++ {
            keys[k] = int(e.keys[k])
        }
        res[i] = RefEvent{StartMS: int(e.start_ms), DurMS: int(e.dur_ms), Keys: keys}
    }
    return res, nil
}

