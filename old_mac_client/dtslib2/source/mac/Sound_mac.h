/*
**	Sound_mac.h		dtslib2
**
**	Copyright 2023 Delta Tao Software, Inc.
** 
**	Licensed under the Apache License, Version 2.0 (the "License");
**	you may not use this file except in compliance with the License.
**	You may obtain a copy of the License at
** 
**		https://www.apache.org/licenses/LICENSE-2.0
** 
**	Unless required by applicable law or agreed to in writing, software
**	distributed under the License is distributed on an "AS IS" BASIS,
**	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
**	See the License for the specific language governing permissions and
**	imitations under the License.
*/

#ifndef Sound_mac_h
#define Sound_mac_h

#ifndef _dtslib2_
#include "Prefix_dts.h"
#endif

// Utilities for dealing with old-fashioned "System 7" sounds
// DTSSound needs these, to help with sound playback;
// the CLEditor needs them for translating Sys7 to/from AIFFs.


bool		IsFormatLPCM( OSType format );

// these basically replicate the functionality that used to be provided by
// the similarly-named routines of the old & deprecated Sound Manager.

OSStatus	MyGetSoundHeaderOffset( const void * ptr, long& headerOffset );
OSStatus	MyParseSndHeader( const void * inPtr, SoundComponentData * cd,
				UInt32 * nFrames, UInt32 * sampleOffset );

#if 0
OSStatus	MyGetCompressionInfo( short compressionID, OSType fmt,
					short numChannels, short sampleSize, CompressionInfo * ci );
#endif  // 0


// a utility, also needed by the editor, to find out exactly how long it took
// to play a given sound.  Returns -1.0 if the sound hasn't been played yet,
// or 0 on various other errors.
CFTimeInterval	GetSoundDuration( const DTSSound * snd );

#endif	// Sound_mac_h

