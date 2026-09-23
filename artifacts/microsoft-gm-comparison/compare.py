from pathlib import Path
import csv, hashlib, re, wave
import numpy as np

out = Path(__file__).resolve().parent
root = out.parent.parent
rate = 44100
slot = 4 * rate

def read(path):
    with wave.open(str(path), 'rb') as f:
        assert (f.getframerate(), f.getnchannels(), f.getsampwidth(), f.getnframes()) == (rate, 2, 2, 92*rate)
        return np.frombuffer(f.readframes(f.getnframes()), '<i2').reshape(-1, 2).astype(float) / 32768

def write(path, data):
    assert np.max(np.abs(data)) <= 1
    with wave.open(str(path), 'wb') as f:
        f.setparams((2, 2, rate, 0, 'NONE', 'not compressed'))
        f.writeframes(np.rint(data * 32767).astype('<i2').tobytes())

def rms(x):
    return np.sqrt(np.mean(x*x))

def db(x):
    return 20*np.log10(max(x, 1e-12))

def spectrum(x):
    # Mean power spectrum of the first 2.2 seconds; independent of phase.
    frames = np.lib.stride_tricks.sliding_window_view(x[:int(2.2*rate)], 2048)[::512]
    power = np.mean(np.abs(np.fft.rfft(frames * np.hanning(2048), axis=1))**2, axis=0)
    # Square root compresses dominance of the strongest harmonics.
    return np.sqrt(power[1:])

def cosine(a, b):
    return float(np.dot(a,b) / (np.linalg.norm(a)*np.linalg.norm(b)))

qt = read(root / 'bard-instrument-audition-quicktime.wav')
ms = read(out / 'microsoft-gm.wav')
old = read(root / 'bard-instrument-audition.wav')
names = re.findall(r'"([^"]+)"', (root/'source/instrument_audition.go').read_text().split('var classicInstrumentNames = [...]string{')[1].split('}')[0])
raw, matched, alternating, rows = [], [], [], []
for i, name in enumerate(names):
    q, m, b = [x[i*slot:(i+1)*slot] for x in (qt, ms, old)]
    qm, mm, bm = [x.mean(axis=1) for x in (q,m,b)]
    raw.append(np.column_stack((qm,mm)))
    # Match whole-slot RMS, then apply one common gain to leave 1 dB headroom.
    ratio = rms(qm)/rms(mm)
    pair = np.column_stack((qm, mm*ratio))
    peak = np.max(np.abs(pair))
    pair *= min(1, 10**(-1/20)/peak)
    matched.append(pair)
    # Stereo playback, QuickTime then Microsoft, separately RMS matched.
    stereo_ratio = rms(q)/rms(m)
    ab = np.concatenate((q,m*stereo_ratio))
    ab *= min(1, 10**(-1/20)/np.max(np.abs(ab)))
    alternating.append(ab)
    sq, sm, sb = [spectrum(x) for x in (qm,mm,bm)]
    rows.append({'index': i, 'instrument': name, 'start_seconds': 4*i,
                 'quicktime_rms_dbfs': round(db(rms(q)),2), 'microsoft_rms_dbfs': round(db(rms(m)),2),
                 'microsoft_minus_quicktime_db': round(db(rms(m)/rms(q)),2),
                 'microsoft_spectral_cosine': round(cosine(sq,sm),4),
                 'checked_in_soundfont_spectral_cosine': round(cosine(sq,sb),4)})
write(out/'quicktime-left-microsoft-right.wav',np.concatenate(raw))
write(out/'quicktime-left-microsoft-right-level-matched.wav',np.concatenate(matched))
write(out/'quicktime-then-microsoft-level-matched.wav',np.concatenate(alternating))
with (out/'measurements.csv').open('w') as f:
    writer=csv.DictWriter(f,fieldnames=rows[0].keys()); writer.writeheader(); writer.writerows(rows)
ms_scores=np.array([r['microsoft_spectral_cosine'] for r in rows])
old_scores=np.array([r['checked_in_soundfont_spectral_cosine'] for r in rows])
print(f'Mean spectral cosine: Microsoft {ms_scores.mean():.4f}; checked-in SoundFont {old_scores.mean():.4f}')
print(f'Microsoft higher on {sum(ms_scores>old_scores)}/23 instruments')
for r in rows: print(r)
report = '''# QuickTime vs Microsoft GM

SoundFont: `/home/dist/.local/share/goThoom/microsoft_gm_4.sf2`.
SHA-256: `816ddcc25f9031ad399fe4ba295f1f5691fe1d9d8fdf3d1dc16e2c38edf37565`.
Reference: repository `bard-instrument-audition-quicktime.wav`.
The generated MIDI is byte-identical to `bard-instrument-audition.mid`.
All renders are 44.1 kHz, stereo, 16-bit PCM, 92 seconds.

- `microsoft-gm.wav`: unmodified goThoom Microsoft GM render.
- `quicktime-left-microsoft-right.wav`: original levels, each input downmixed to mono.
- `quicktime-left-microsoft-right-level-matched.wav`: same, RMS matched per instrument with 1 dB peak headroom.
- `quicktime-then-microsoft-level-matched.wav`: original stereo, four seconds QuickTime then four seconds Microsoft for each instrument (184 seconds total), RMS matched per instrument.
- `measurements.csv`: original stereo levels plus phase-independent spectral diagnostics.

The spectral diagnostic is cosine similarity between square-root mean power spectra from the first 2.2 seconds of each slot (2048-sample Hann windows, 512-sample hop, DC omitted). It measures broad spectral agreement, not perceptual equivalence or a percentage match. It does not score attack/release timing, stereo placement, or reverb separately. The existing checked-in SoundFont recording is included as a baseline; it was not freshly rendered, so this is not a controlled comparison of SoundFonts alone. The Microsoft recording uses the current goThoom renderer, including its instrument gain normalization and release behavior. The QuickTime reference contains 11,433 channel samples at the 16-bit PCM limits, indicating clipping in the reference. Level matching cannot undo that distortion. No listening judgment is inferred from the numbers.

'''
report += f'Mean spectral cosine: Microsoft {ms_scores.mean():.4f}; checked-in SoundFont {old_scores.mean():.4f}. Microsoft scores higher for {sum(ms_scores>old_scores)} of 23 instruments.\n\n'
report += '| Start | Instrument | Microsoft spectral cosine | Checked-in SoundFont | Microsoft level minus QuickTime |\n|---|---|---:|---:|---:|\n'
for r in rows:
    s=r['start_seconds']; report += f"| {s//60}:{s%60:02d} | {r['instrument']} | {r['microsoft_spectral_cosine']:.4f} | {r['checked_in_soundfont_spectral_cosine']:.4f} | {r['microsoft_minus_quicktime_db']:+.2f} dB |\n"
report += '\nReproduce the measurements and listening files with Python and NumPy: `python compare.py`.\n'
(out/'README.md').write_text(report)
