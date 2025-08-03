/*
**	Music_cl.cp		ClanLord
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

#include "ClanLord.h"

/*
**	Entry Routines
*/
/*
void	RunCLMusic();
void	PlayCLMusic();
bool	CheckMusicCommand( const char * text, size_t len );
*/

/*
**	Definitions
*/


/*
**	Internal Routines
*/
static DTSError		PlayRandomFile();
static DTSError		PlayIndexedFile( int index );
static void			PlayMusicByName( const char * name );


/*
**	Internal Variables
*/
static int			gLastFile = -1;



/*
**	RunCLMusic()
**
**	initialize
*/
void
RunCLMusic()
{
	if ( not gPrefsData.pdMusicPlay )
		return;
	if ( not IsMusicDone() )
		return;
	PlayRandomFile();
}


/*
**	PlayCLMusic()
**
**	handle the music menu item
*/
void
PlayCLMusic()
{
	if ( gPrefsData.pdMusicPlay )
		{
		gPrefsData.pdMusicPlay = false;
		StopMusic();
		}
	else
		{
		DTSError result = PlayRandomFile();
		if ( noErr == result )
			gPrefsData.pdMusicPlay = true;
		}
}


/*
**	PlayRandomFile()
**
**	pick a file in the music folder and play it
*/
DTSError
PlayRandomFile()
{
	long numfiles;
	short * table = nullptr;
	
	DTSFileSpec save;
	save.GetCurDir();
	
	DTSFileSpec musicfolder;
	musicfolder.SetFileName( "Music" );
	
	DTSError result = musicfolder.SetDir();
	
	if ( noErr == result )
		{
		numfiles = CountFiles();
		if ( numfiles <= 0 )
			result = -1;
		else
		if ( numfiles >= SHRT_MAX )		// paranoia
			numfiles = SHRT_MAX - 1;
		}
	if ( noErr == result )
		{
		table = NEW_TAG("PlayRandomMusicFile") short[ numfiles ];
		if ( not table )
			result = memFullErr;
		}
	if ( noErr == result )
		{
		// initialize the table to contain all files
		short * walk = table;
		int index;
		for ( index = 0;  index < numfiles;  ++index )
			*walk++ = index;
		
		// remove the last played file
		// correct the global because we use it
		// with assumptions later
		index = gLastFile;
		if ( index >= numfiles )
			{
			index     = -1;
			gLastFile = -1;
			}
		if ( index >= 0 )
			{
			--numfiles;
			table[index] = numfiles;
			}
		
		// pick files at random 
		for(;;)
			{
			// out of files
			if ( numfiles <= 0 )
				{
				// try to play the last song again
				index = gLastFile;
				if ( index >= 0 )
					{
					result = PlayIndexedFile( index );
					break;
					}
				
				// there was no last song
				result = -1;
				break;
				}
			
			// pick a song at random
			// play it
			index = GetRandom( numfiles );
			int trial = table[ index ];
			result = PlayIndexedFile( trial );
			if ( noErr == result )
				{
				// success!
				// set the menu option
				// remember this song
				gLastFile = trial;
				break;
				}
			
			// couldn't play it for some reason
			// too big, not a music file, whatever
			--numfiles;
			table[ index ] = table[ numfiles ];
			}
		}
	
	save.SetDirNoPath();
	
	delete[] table;
	
	return result;
}


/*
**	PlayIndexedFile()
**
**	play the indexed file
*/
DTSError
PlayIndexedFile( int index )
{
	DTSFileSpec indexedFile;
	DTSFileIterator iter;
	while ( index-- >= 0 && iter.Iterate( indexedFile ) )
		;
	
	DTSError result = noErr;
//	if ( noErr == result )
		{
		if ( false )	// better yet: gPrefsData.pdShowMusicMovieNames
			{
			char msg[ 256 ];
			snprintf( msg, sizeof msg, _(TXTCL_STARTING_MUSIC_FILE), indexedFile.GetFileName() );
			ShowInfoText( msg );
			}
		result = PlayMusicFile( &indexedFile );
		}
	
	return result;
}


/*
**	PlayMusicByName()
*/
void
PlayMusicByName( const char * name )
{
	if ( not gPrefsData.pdMusicPlay )
		return;
	
	DTSFileSpec save;
	save.GetCurDir();
	
	DTSFileSpec folder;
	folder.SetFileName( "Music" );
	DTSError result = folder.SetDir();
	
	// check the file exists
	DTSFileSpec file;
	if ( noErr == result )
		{
		file.SetFileName( name );
		DTSDate mod;
		result = file.GetModifiedDate( &mod );
		}
	
	if ( noErr == result )
		{
		StopMusic();
		(void) PlayMusicFile( &file );
		}
	
	save.SetDirNoPath();
}


/*
**	CheckMusicCommand
**
**	see if the text contained a music command
**	returns true if it did
*/
bool
CheckMusicCommand( const char * inCommand, size_t inLen )
{
	const size_t wantLen = 3;	// == strlen( KBEPPMusic )
	
	if ( inLen <= wantLen )
		return false;
	
	// if it's not our command, we're done
	if ( 0 == std::strncmp( inCommand, kBEPPMusic, wantLen ) )
		{
//		ShowMessage( inCommand );
		PlayMusicByName( inCommand + wantLen );
		return true;
		}
	
	return false;
}


