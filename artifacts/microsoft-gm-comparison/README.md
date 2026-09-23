# QuickTime vs Microsoft GM

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

Mean spectral cosine: Microsoft 0.9200; checked-in SoundFont 0.8334. Microsoft scores higher for 19 of 23 instruments.

| Start | Instrument | Microsoft spectral cosine | Checked-in SoundFont | Microsoft level minus QuickTime |
|---|---|---:|---:|---:|
| 0:00 | Lucky Lyra | 0.6554 | 0.6812 | -17.41 dB |
| 0:04 | Bone Flute | 0.9872 | 0.9367 | -6.78 dB |
| 0:08 | Starbuck Harp | 0.9836 | 0.9435 | -5.03 dB |
| 0:12 | Torjo | 0.8680 | 0.7706 | -5.17 dB |
| 0:16 | Xylo | 0.8418 | 0.8628 | -2.20 dB |
| 0:20 | Gitor | 0.9888 | 0.7767 | +0.94 dB |
| 0:24 | Reed Flute | 0.9767 | 0.9401 | -8.65 dB |
| 0:28 | Temple Organ | 0.9820 | 0.7038 | -6.11 dB |
| 0:32 | Conch | 0.8955 | 0.8964 | -7.57 dB |
| 0:36 | Ocarina | 0.9345 | 0.9485 | -11.88 dB |
| 0:40 | Centaur Organ | 0.8659 | 0.8442 | -8.42 dB |
| 0:44 | Vibra | 0.9785 | 0.9389 | -1.72 dB |
| 0:48 | Tuborn | 0.9406 | 0.7990 | -4.08 dB |
| 0:52 | Bagpipe | 0.9901 | 0.8806 | -4.94 dB |
| 0:56 | Orga Drum | 0.9715 | 0.8860 | -3.46 dB |
| 1:00 | Casserole | 0.6662 | 0.6475 | -3.65 dB |
| 1:04 | Violene | 0.8866 | 0.7231 | +0.08 dB |
| 1:08 | Pine Flute | 0.9239 | 0.7880 | -3.05 dB |
| 1:12 | Groanbox | 0.9813 | 0.7859 | -2.90 dB |
| 1:16 | Gho-To | 0.9365 | 0.8469 | -2.96 dB |
| 1:20 | Mammoth Violene | 0.9438 | 0.7989 | -4.20 dB |
| 1:24 | Gutbucket Bass | 0.9826 | 0.9128 | -5.14 dB |
| 1:28 | Glass Jug | 0.9794 | 0.8558 | -5.59 dB |

Reproduce the measurements and listening files with Python and NumPy: `python compare.py`.
