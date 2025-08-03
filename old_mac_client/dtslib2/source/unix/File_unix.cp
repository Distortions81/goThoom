/*
**	File-unix.cp		dtslib2
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

#include "File-dts.h"

#define		kOpenMagic		  (00400|00200|00040|00020)


/*
**	DTS_create()
**
**	create a file
**	platform dependent
*/
DTSError
DTS_create( DTSFileSpec * spec, long fileType )
{
	fileType = 0;	// not used
	
	DTSError result = noErr;
	
	int fd = open( spec->fileName, O_RDWR|O_CREAT, kOpenMagic );
	if ( fd < 0 )
		{
		result = kUnableToCreateFile;
		}
	else
		{
		close( fd );
		}
	
	return result;
}


/*
**	DTS_open()
**
**	open a file
**	platform dependent
*/
DTSError
DTS_open( DTSFileSpec * spec, DTSBoolean bWritePerm, long * fileref )
{
	DTSError result = noErr;
	
	int perm = O_RDONLY;
	if ( bWritePerm )
		{
		perm = O_RDWR;
		}
	
	int fd = open( spec->fileName, perm );
	if ( fd <= 0 )
		{
		extern int errno;
		if ( errno == 2 )
			{
			result = fnfErr;
			}
		else
			{
			result = kCouldNotOpenFile;
			}
		}
	
	*fileref = fd;
	
	return result;
}


/*
**	DTS_close()
**
**	close a file
**	platform dependent
*/
void
DTS_close( long fileref )
{
	if ( fileref >= 0 )
		{
		close( fileref );
		}
}


/*
**	DTS_seek()
**
**	seek to the position in the file
**	platform dependent
*/
DTSError
DTS_seek( long fileref, long position )
{
	DTSError result = noErr;
	long newpos = lseek( fileref, position, SEEK_SET );
	if ( newpos < 0 )
		{
		result = kCouldNotSeek;
		}
	return result;
}


/*
**	DTS_seteof()
**
**	set the end of the file
**	platform dependent
*/
DTSError
DTS_seteof( long fileref, long eof )
{
	// um...
	fileref = 0;
	eof = 0;
	
	return noErr;
}


/*
**	DTS_geteof()
**
**	get the end of the file
**	platform dependent
*/
DTSError
DTS_geteof( long fileref, long * peof )
{
	DTSError result = noErr;
	long eof = lseek( fileref, 0, SEEK_END );
	if ( eof < 0 )
		{
		result = kCouldNotSeek;
		eof = 0x7FFFFFFF;
		}
	*peof = eof;
	return result;
}


/*
**	DTS_read()
**
**	read data
**	platform dependent
*/
DTSError
DTS_read( long fileref, void * buffer, long size )
{
	// do the fread
	long numread = read( fileref, (char *) buffer, size );
	
	// convert to an error code
	// anything less than the largest negative number
	// and anything where we didn't read the whole record
	DTSError result = noErr;
	if ( numread < (long) 0xFFFF8000 )
		{
		result = kCouldNotRead;
		}
	else
	if ( numread != size )
		{
		result = kCouldNotRead;
		}
	
	return result;
}


/*
**	DTS_write()
**
**	write data
**	platform dependent
*/
DTSError
DTS_write( long fileref, const void * buffer, long size )
{
	// do the fwrite
	long numwritten = write( fileref, (const char *) buffer, size );
	
	// convert to an error code
	// anything less than the largest negative number
	// and anything where we didn't write the whole record
	DTSError result = noErr;
	if ( numwritten < (long) 0xFFFF8000
	|| ( numwritten >= 0
	&&   numwritten != size ) )
		{
		result = kCouldNotWrite;
		}
	
	return result;
}

