/*
**	File_dts.h		dtslib2
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

#ifndef File_dts_h
#define File_dts_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"
#include "Platform_dts.h"


/*
**	generic file type references
*/
const int 	rTextFREF			= 32000;
const int	rFolderFREF			= 32001;


/*
**	File-related error codes
*/
enum
{
#ifndef __MACERRORS__
	fnOpnErr				= -38,
	fnfErr					= -43,
	dupFNErr				= -48,
	wrPermErr				= -61,
#endif	// __MACERRORS__

	kCouldNotOpenFile		= -31999,
	kReopenPermissionErr	= -31998,
	kBadHeaderVersion		= -31997,
	kUnableToAllocMem		= -31996,
	kEntryDoesNotExist		= -31995,
	kBadSizeParam			= -31994,
	kUnableToCreateFile		= -31993,
	kKeyFileNotOpen			= -31992,
	kCouldNotRead			= -31991,
	kCouldNotWrite			= -31990,
	kCouldNotSeek			= -31989,
	kCouldNotGetPos			= -31988,
	kCouldNotTruncate		= -31987
};


/*
**	Define	DTSFileSpec class
*/
class DTSFileSpecPriv;
class DTSFileSpec
{
public:
	DTSImplement<DTSFileSpecPriv> priv;
	
	// set the file name
	DTSError		SetFileName( const char * fname );
	
	// get the file name
	const char *	GetFileName() const;					
	
	// sets to the current directory including fileName
	DTSError		GetCurDir();
	
	// changes the current directory ignoring fileName
	DTSError		SetDirNoPath() const;
	
	// changes the current directory using path in fileName
	DTSError		SetDir();
	
	// set the preferences folder in the system folder
	DTSError		SetToPrefs();
	
	// creates a new folder
	DTSError		CreateDir();
	
	// sets the Mac file type and creator (indirectly, via FREF IDs)
	DTSError		SetTypeCreator( int typeFREFID, int creatorFREFID );
	
	// returns the FREF ID of the Mac file type
	DTSError		GetType( int * typeFREFID );
	
	// renames the file
	DTSError		Rename( const char * newFileName );
	
	// moves the file to the new directory
	DTSError		Move( DTSFileSpec * newDir );
	
	// deletes the file
	DTSError		Delete();
	
	// returns the modification date
	DTSError		GetModifiedDate( DTSDate * date, bool bJustWantSeconds = false );
	
	// calls fopen(3)
	std::FILE *		fopen( const char * perms );
	
	// user selects a file name and directory for writing
	bool			PutDialog( const char * prompt, ulong prefKey = 0 );
	
	// user selects a file name and directory for reading
	// NOTE: prompt is ignored unless using NavServices!
	bool			GetDialog( int fileTypeFREF,
						const char * prompt = nullptr, ulong prefKey = 0 );
};

int			CountFiles();		// returns the number of files in the current directory


/*
**	replacement for removed by-index accessor
*/
class DTSFileIteratorPriv;
class DTSFileIterator
{
public:
	DTSImplementNoCopy<DTSFileIteratorPriv> priv;
	
					DTSFileIterator();
					~DTSFileIterator();
					
	bool			Iterate( DTSFileSpec& outSpec ) const;
};


/*
**	file io wrappers
*/
DTSError	DTS_create( DTSFileSpec * spec, ulong fileType );
DTSError	DTS_open( DTSFileSpec * spec, bool bWritePerm, int * fileref );
void		DTS_close( int fileref );
DTSError	DTS_seek( int fileref, ulong position );
DTSError	DTS_seteof( int fileref, ulong eof );
DTSError	DTS_geteof( int fileref, ulong * eof );
DTSError	DTS_read( int fileref, void * buffer, size_t size );
DTSError	DTS_write( int fileref, const void * buffer, size_t size );

#endif	// File_dts_h
