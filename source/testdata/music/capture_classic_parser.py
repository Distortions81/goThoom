#!/usr/bin/env python3
"""Recapture note events from the original classic parser, without QuickTime."""

import argparse
import json
from pathlib import Path
import subprocess
import tempfile

# These stubs record the original parser's QuickTime calls. They do not parse
# notation. Instrument metadata covers the fixture instruments and concert
# instruments checked alongside them; unsupported indexes fail explicitly.
PRELUDE = r'''
#include <vector>
#include <string>
#include <cstring>
#include <cstdio>
#include <cstdlib>
#include <cctype>
#include <new>
using OSStatus=int;
using Ptr=char*;
using ushort=unsigned short;
constexpr int noErr=0,kMiddleC=60,kNumOctaves=3,kOctave=12,kDefaultVelocity=100,kMinimumTempo=60,kDefaultTempo=120,kMaximumTempo=180,kDefaultVoice=1;
struct MusicOpWord {
 int kind=0,part=0,pitch=0,velocity=0,duration=0;
 MusicOpWord& operator=(int) {kind=4;return *this;}
};
constexpr int kEndMarkerValue=4;
struct NoteRequest {};
struct SafeString {
 template<typename... Args> void Format(const char*,Args...){}
 const char* Get(){return "";}
};
void ShowInfoText(const char*){}
struct CDataHandle {
 std::vector<MusicOpWord> words;
 char* GetPtr(){return reinterpret_cast<char*>(words.data());}
 void Reset(){words.clear();}
 char* Add(const void*,long bytes){
  auto start=words.size();words.resize(start+bytes/sizeof(MusicOpWord));
  return reinterpret_cast<char*>(&words[start]);
 }
};
void qtma_StuffNoteEvent(MusicOpWord& w,int part,int pitch,int velocity,int duration){w={1,part,pitch,velocity,duration};}
void qtma_StuffXNoteEvent(MusicOpWord& w,MusicOpWord& x,int part,int pitch,int velocity,int duration){w={1,part,pitch,velocity,duration};x.kind=3;}
void qtma_StuffRestEvent(MusicOpWord& w,int duration){w={2,0,0,0,duration};}
struct CCLInstrument {
 enum {flags_NoChords=1,flags_NoMelody=2,flags_LongChord=4};
 int mFlags=0,mPolyphony=10,mOctaveOffset=0,mChordVelocity=100,mMelodyVelocity=100;
 bool HasChords() const{return !(mFlags&flags_NoChords);}
 bool HasMelody() const{return !(mFlags&flags_NoMelody);}
 int GetPolyphony() const{return HasChords()?mPolyphony:0;}
 bool IsNoteRestricted(int) const{return false;}
};
'''

RECORDING_MAIN = r'''
const char* const CTuneBuilder::sErrorMessages[]={""};
const char* errorName(int e){
 const char* names[]={"none","invalid_note","invalid_modifier","tone_overflow","polyphony_overflow","invalid_octave","invalid_chord","unsupported_instrument","out_of_memory","too_many_marks","unmatched_mark","unmatched_comment","invalid_tempo","invalid_tempo_change","modifier_need_value","duplicate_ending","duplicate_default_ending","ending_in_chord","ending_outside_loop","default_ending_error","invalid_ending_index","unterminated_loop","unterminated_chord","already_playing"};
 return e>=0&&e<24?names[e]:"unknown";
}
int main(int argc,char** argv){
 int inst=std::atoi(argv[1]);
 for(int i=2;i<argc;++i){
  CCLInstrument spec;
  if(inst==0){spec.mOctaveOffset=1;spec.mPolyphony=6;}
  else if(inst==7){spec.mOctaveOffset=-1;spec.mFlags=4;}
  else if(inst==8){spec.mOctaveOffset=-1;spec.mFlags=4;spec.mPolyphony=1;}
  else if(inst==1){spec.mOctaveOffset=1;spec.mPolyphony=0;spec.mFlags=1;}
  else if(inst==5){spec.mPolyphony=6;}
  else if(inst==16){spec.mOctaveOffset=1;spec.mPolyphony=2;}
  else if(inst!=2){return 2;}
  alignas(CTuneBuilder) unsigned char storage[sizeof(CTuneBuilder)]{};
  auto* builder=new(storage) CTuneBuilder(spec,120,100);
  CDataHandle song;
  int error=builder->BuildTune(&song,argv[i],std::strlen(argv[i]),false);
  std::printf("{\"index\":%d,\"error\":\"%s\",\"position\":%ld,\"total_ticks\":%lu,\"notes\":[",i-2,errorName(error),builder->GetErrorPosition(),builder->GetDuration());
  int current=0,count=0;
  for(auto w:song.words){
   if(w.kind==2){current+=w.duration;}
   if(w.kind==1){
    std::printf("%s{\"part\":%d,\"pitch\":%d,\"velocity\":%d,\"start_ticks\":%d,\"duration_ticks\":%d}",count++?",":"",w.part,w.pitch,w.velocity,current-300,w.duration);
   }
  }
  puts("]}");
  builder->~CTuneBuilder();
 }
}
'''


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("classic_checkout", type=Path,
                        help="ClanLordClient checkout at commit 6ba334c")
    args = parser.parse_args()
    source = args.classic_checkout / "mac_client/client/source"
    header = (source / "TuneHelper_cl.h").read_text()
    implementation = (source / "TuneHelper_cl.cp").read_text()
    start = header.index("class CTuneBuilder\n")
    declaration = header[start:header.index("\n};", start) + 3]
    start = implementation.index("CTuneBuilder::CTuneBuilder() :")
    end = implementation.index(
        "\n#ifdef CL_TUNE_HELPER_APP\n/*\n**\tCTuneBuilder::StuffNoteOffet", start)
    methods = implementation[start:end]
    fixture = Path(__file__).with_name("classic_parser.json")
    cases = json.loads(fixture.read_text())
    captured = []
    with tempfile.TemporaryDirectory(prefix="classic-parser-") as directory:
        directory = Path(directory)
        cpp = directory / "reference.cpp"
        executable = directory / "reference"
        cpp.write_text(PRELUDE + declaration + "\n" + methods + "\n" + RECORDING_MAIN)
        subprocess.run(["c++", "-std=c++17", "-O2", str(cpp), "-o", str(executable)],
                       check=True)
        for case in cases:
            result = subprocess.run(
                [str(executable), str(case["instrument"]), case["tune"]],
                check=True, capture_output=True, text=True, timeout=5)
            result = json.loads(result.stdout)
            row = {"instrument": case["instrument"], "tune": case["tune"],
                   "error": result["error"]}
            if result["error"] == "none":
                row["notes"] = [
                    {"key": note["pitch"], "velocity": note["velocity"],
                     "start": note["start_ticks"], "duration": note["duration_ticks"]}
                    for note in result["notes"] if note["duration_ticks"] > 0
                ]
            captured.append(row)
    fixture.write_text("[\n" + ",\n".join(
        "  " + json.dumps(row, separators=(",", ":")) for row in captured) + "\n]\n")
    print(f"Captured {len(captured)} classic reference cases in {fixture}")


if __name__ == "__main__":
    main()
