/*
**	Music_mac.cp		dtslib2
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

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif


#include "Local_dts.h"
#include "Music_dts.h"
#include "File_mac.h"
#include "Music_mac.h"
#include "Shell_mac.h"


/*
**	Entry Routines
*/
/*
// user
DTSError	InitMusic();
void		ExitMusic();
DTSError	PlayMusicFile( DTSFileSpec * spec );
void		StopMusic();
void		PauseMusic();
void		ResumeMusic();
void		SetMusicVolume( int newVolume );
bool		IsMusicDone();

// internal
void		IdleMusic();
*/


/*
**	Definitions
*/
const int kMaxVolume = 100;


/*
**	Internal Variables
*/
static Movie	gCurMovie;
static int		gVolume;
static bool		gHasQuickTime;		// true if QT with midi playback exists


/*
**	InitMusic()
**
**	initialize music
*/
DTSError
InitMusic()
{
	gVolume	= kMaxVolume;
	
	// check to see if QuickTime exists, before relying on it
	SInt32 data;
	OSStatus err = ::Gestalt( gestaltQuickTimeVersion, &data );
	
	// Can we run with less than QT 2.0? If so, adjust this as needed
	// I think midi files first became playable at this version
	if ( not gOSFeatures.hasQuickTime		// this only implies 1.0
	||	 noErr != err
	||	 data < 0x02000000 )
		{
		return -1;
		}
	gHasQuickTime = true;
	
	return ::EnterMovies();
}


/*
**	ExitMusic()
**
**	stop the music
*/
void
ExitMusic()
{
	// don't call QT if it doesn't exist
	if ( gHasQuickTime )
		{
		StopMusic();
		::ExitMovies();
		}
}


/*
**	PlayMusicFile()
**
**	play a music file
*/
DTSError
PlayMusicFile( DTSFileSpec * spec )
{
	// bail cleanly if no QT
	if ( not gHasQuickTime )
		return noErr;
	
	if ( not spec )
		return -1;
	
	Movie movie = NULL;
	
#if 1
	FSRef fsr;
	Handle dataRef = NULL;
	OSType dataRefType = 0;
	DTSError result = DTSFileSpec_CopyToFSRef( spec, &fsr );
	if ( noErr == result )
		{
		result = QTNewDataReferenceFromFSRef( &fsr, kNilOptions, &dataRef, &dataRefType );
		__Check_noErr( result );
		}
	if ( noErr == result )
		{
		SInt16 resid = movieInDataForkResID;
		result = NewMovieFromDataRef( &movie, newMovieActive, &resid, dataRef, dataRefType );
		__Check_noErr( result );
		}
	if ( dataRef )
		{
		DisposeHandle( dataRef );
		__Check_noErr( MemError() );
		}
#else
	FSSpec fss;
	FSRef fsr;
	SInt16 fRefNum = 0;
	DTSError result = DTSFileSpec_CopyToFSRef( spec, &fsr );
	if ( noErr == result )
		{
		// convert FSRef to FSSpec, until we can rewrite this whole mess to use
		// NewMovieFromDataRef() or whatever the API-du-jour is...
		result = FSGetCatalogInfo( &fsr, kFSCatInfoNone, NULL, NULL, &fss, NULL );
		__Check_noErr( result );
		}
	
	if ( noErr == result )
		result = ::OpenMovieFile( &fss, &fRefNum, fsRdPerm );
	if ( noErr == result )
		{
		SInt16 resid = movieInDataForkResID;
		Boolean refChanged;
		result = ::NewMovieFromFile( &movie, fRefNum, &resid, NULL, newMovieActive, &refChanged );
		}
	
	if ( fRefNum )
		::CloseMovieFile( fRefNum );
#endif  // 0
	
	if ( noErr == result )
		{
		// end the current movie
		if ( gCurMovie )
			{
			::DisposeMovie( gCurMovie );
			gCurMovie = NULL;
			}
		
		// SetMovieVolume() takes a ShortFixed parameter
		int volume = 0x100 * gVolume / kMaxVolume;
		::SetMovieVolume( movie, volume );
		::GoToBeginningOfMovie( movie );
		::MoviesTask( movie, 0 );
		::StartMovie( movie );
		gCurMovie = movie;
		}
	
	if ( noErr != result )
		{
		if ( movie )
			{
			::StopMovie( movie );
			::DisposeMovie( movie );
			}
		}
	
	return result;
}


/*
**	StopMusic()
**
**	stop playing the music
*/
void
StopMusic()
{
	// bail cleanly if no QT
	if ( not gHasQuickTime )
		return;
	
	if ( Movie movie = gCurMovie )
		{
		::StopMovie( movie );
		::DisposeMovie( movie );
		gCurMovie = NULL;
		}
}


/*
**	PauseMusic()
**
**	pause the music
*/
void
PauseMusic()
{
	// bail cleanly if no QT
	if ( not gHasQuickTime )
		return;
	
	if ( Movie movie = gCurMovie )
		::StopMovie( movie );
}


/*
**	ResumeMusic()
**
**	resume the music
*/
void
ResumeMusic()
{
	// bail cleanly if no QT
	if ( not gHasQuickTime )
		return;
	
	if ( Movie movie = gCurMovie )
		::StartMovie( movie );
}


/*
**	SetMusicVolume()
**
**	set the volume of the music
*/
void
SetMusicVolume( int volume )
{
	// clamp to 0..100
	if ( volume < 0 )
		volume = 0;
	else
	if ( volume > kMaxVolume )
		volume = kMaxVolume;
	gVolume = volume;
	
	// bail cleanly if no QT
	if ( not gHasQuickTime )
		return;
	
	if ( Movie movie = gCurMovie )
		::SetMovieVolume( movie, volume << 8 );
}


/*
**	IsMusicDone()
**
**	return true if no music is playing
*/
bool
IsMusicDone()
{
	bool result = true;
	
	// bail cleanly if no QT	
	if ( not gHasQuickTime )
		return result;
	
	if ( Movie movie = gCurMovie )
		result = ::IsMovieDone( movie );
	
	return result;
}


/*
**	IdleMusic()
**
**	call movie task
*/
void
IdleMusic()
{
	// bail cleanly if no QT
	if ( not gHasQuickTime )
		return;
	
	if ( Movie movie = gCurMovie )
		::MoviesTask( movie, 0 );
}


