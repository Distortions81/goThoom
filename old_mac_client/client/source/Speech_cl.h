/*
**	Speech_cl.h		ClanLord
**
**	Copyright 2023 Delta Tao Software, Inc.
** 
**	Licensed under the Apache License, Version 2.0 (the "License");
**	you may not use this file except in compliance with the License.
**	You may obtain a copy of the License at
** 
**     https://www.apache.org/licenses/LICENSE-2.0
** 
**	Unless required by applicable law or agreed to in writing, software
**	distributed under the License is distributed on an "AS IS" BASIS,
**	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
**	See the License for the specific language governing permissions and
**	imitations under the License.
*/

#ifndef SPEECH_CL_H
#define SPEECH_CL_H

/*
**	Global Variables
*/
extern bool gSpeechAvailable;

/*
**	Exported Routines
*/

// call once at startup: returns an error code and sets gSpeechAvailable
void		InitSpeech();

// call once each frame
void		IdleSpeech();

// add a request for speech to the queue
DTSError	Speak( const char * text );

// call when the application is being suspended
void		SuspendSpeech();

// call when being resumed
void		ResumeSpeech();

// call at quitting time
void		ExitSpeech();

#endif	// SPEECH_CL_H
