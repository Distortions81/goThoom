/*
**	MessageWin_cl.cp		ClanLord, CLServer
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

#ifdef DEBUG_VERSION

#include <unistd.h>
#include "Public_cl.h"
#include "View_mac.h"


/*
**	Entry Routines
*/
/*
void	CreateMsgWin( int numLines, int kind, const DTSRect * box );
void	CloseMsgWin();
void	ShowMessage( const char * format, ... );
void	VShowMessage( const char * format, va_list params );
void	ShowDump( const void * dataPtr, size_t count );
void	GetMsgWinPos( DTSPoint * pt, DTSPoint * size = nullptr );
void	SaveMsgLog( const char * fname );
*/


/*
**	Definitions
*/

/*
**	Internal Variables
*/
static FILE *		gMsgFile;


/*
**	CreateMsgWin()
**
**	Create and show the floating message window.
*/
void
CreateMsgWin( int /* numLines */, int /* kind */, const DTSRect * /* box */ )
{
}


/*
**	CloseMsgWin()
**
**	close the message window and the file
*/
void
CloseMsgWin()
{
	if ( FILE * stream = gMsgFile )
		{
		fclose( stream );
		gMsgFile = nullptr;
		}
}


/*
**	ShowMessage()
**
**	show the message
*/
void
ShowMessage( const char * format, ... )
{
	va_list params;
	va_start( params, format );
	
	VShowMessage( format, params );
	
	va_end( params );
}


/*
**	VShowMessage()
**
**	show the message, both to stderr (unconditionally)
**	and to the gMsgFile (if it's been opened)
*/
void
VShowMessage( const char * format, va_list params )
{
	// generate a time-stamp prefix ("hh:mm:ss ")
	char timestamp[ 32 ];
	time_t now = time( nullptr );
	struct tm now_tm;
	localtime_r( &now, &now_tm );
	strftime( timestamp, sizeof timestamp, "%T ", &now_tm );	// "%T" == "%H:%M:%S"
	
	// we rely on stderr being connected to somewhere useful, e.g. the console log.
	fputs( timestamp, stderr );
	
	// cache 'params', because this vfprintf() may destroy it
	va_list saved_params;
	va_copy( saved_params, params );
	vfprintf( stderr, format, params );
	
	fputc( '\n', stderr );
	
	// echo it all to the log file, if it's open
	if ( FILE * stream = gMsgFile )
		{
		fputs( timestamp, stderr );
		
		vfprintf( stream, format, saved_params );
		fputc( '\n', stream );
//		fflush( stream );
		}
	va_end( saved_params );
}


/*
**	ShowDump()
**
**	show a hex dump of the memory location
*/
void
ShowDump( const void * /* dataPtr */, size_t /* count */ )
{
#if 0  // not currently used
	char buff[ 128 ];
	char * dst = buff;
	const uchar * src = static_cast<const uchar *>( dataPtr );
	int flush = 32;
	int size = sizeof buff;
	for ( ;  count > 0;  --count )
		{
		size_t len = snprintfx( dst, size, "%2.2x ", *src++ );
		size -= len;
		dst  += len;
		
		if ( --flush <= 0 )
			{
			ShowMessage( "%s", buff );
			flush = 32;
			dst = buff;
			size = sizeof buff;
			}
		}
	
	if ( flush < 32 )
		ShowMessage( "%s", buff );
#endif  // 0
}


/*
**	GetMsgWinPos()
**
**	return the position of the message window
*/
void
GetMsgWinPos( DTSPoint * /* pos */, DTSPoint * /* size */ /* =nullptr */)
{
}


/*
**	SaveMsgLog()
**
**	set the flag that decides if we save to the message log or not
*/
void
SaveMsgLog( const char * fname )
{
	// close the file
	if ( FILE * stream = gMsgFile )
		{
		fclose( stream );
		gMsgFile = nullptr;
		}
	
	// if a file name was given then open it
	// kill anything that was in the file
	if ( fname )
		{
		// depending on how this app was launched, the current working dir might be
		// the app's folder, or "/", or "$HOME", or ... .
		// So use a fixed dir for this fopen().
		
		// first, save the current working dir. (This method is simpler than getcwd().)
		int prevWD = open( ".", O_RDONLY );
		
		// chdir() to application's folder
		CFURLRef parURL = nullptr;
		if ( CFURLRef appURL = CFBundleCopyBundleURL( CFBundleGetMainBundle() ) )
			{
			parURL = CFURLCreateCopyDeletingLastPathComponent( kCFAllocatorDefault, appURL );
			CFRelease( appURL );
			}
		if ( parURL )
			{
			char appPath[ PATH_MAX + 1 ];
			if ( CFURLGetFileSystemRepresentation( parURL,
					true, (uchar *) appPath, sizeof appPath ) )
				{
				int err = chdir( appPath );
				if ( err )
					err = - errno;
				}
			
			CFRelease( parURL );
			}
		
		// open/create the logfile, in the app's directory
		gMsgFile = fopen( fname, "w" );
		
		// set line-buffered mode
		if ( gMsgFile )
			setvbuf( gMsgFile, nullptr, _IOLBF, 0 );
		
		// restore previous working dir
		if ( -1 != prevWD )
			{
			fchdir( prevWD );
			close( prevWD );
			}
		}
}

#endif  // defined( DEBUG_VERSION )

