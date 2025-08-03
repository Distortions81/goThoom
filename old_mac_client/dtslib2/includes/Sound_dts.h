/*
**	Sound_dts.h		dtslib2
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

#ifndef Sound_dts_h
#define Sound_dts_h

#ifndef _dtslib2_
#include "Prefix_dts.h"
#endif

#include "Local_dts.h"
#include "Platform_dts.h"


/*
**	Define DTSSound class
*/
class DTSSoundPriv;
class DTSSound
{
public:
	DTSImplementNoCopy<DTSSoundPriv> priv;
	
	// interface
	DTSError	Init( const void * ptr, size_t len );
	void		Play();
	void		Stop();
	bool		IsDone() const;
};


/*
**	Interface Routines
*/
void	DTSInitSound( int numChannels );		// initialize the sound player
void	DTSExitSound();							// de-initialize the sound player
void	DTSSetSoundVolume( ushort volume );		// set baseline volume for sounds
// Sound volumes range from 0 = silent to 0xFFFF = hardware maximum

#endif	// Sound_dts_h

