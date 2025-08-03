/*
**	Music_dts.h		dtslib2
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

#ifndef Music_dts_h
#define Music_dts_h

#ifndef _dtslib2_
#include "Prefix_dts.h"
#endif

#include "Local_dts.h"


// music from Ben Watson
DTSError	InitMusic();
void		ExitMusic();
DTSError	PlayMusicFile( DTSFileSpec * spec );
void		StopMusic();
void		PauseMusic();
void		ResumeMusic();
void		SetMusicVolume( int newVolume );
bool		IsMusicDone();


#endif  // Music_dts_h
