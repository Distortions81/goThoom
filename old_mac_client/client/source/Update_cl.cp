/*
**	Update_cl.cp		ClanLord
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
#include "VersionNumber_cl.h"
#include "ProgressWin_cl.h"


/*
**	Entry Routines
**
void		OpenKeyFiles();
void		CloseKeyFiles();
void		UpdateKeyFiles();
DTSError	UpdateImagesFile( int newversion );
DTSError	UpdateSoundsFile( int newversion );
*/


/*
**	Definitions
*/
const int kVersionCD				= 64 << 8;
//const int kVersionOldestUpdate	= 30;

	// progress window layout geometry
const int kPWinHorzInset			= 20;		// L/R margins of progress control
const int kPWinBarTop				= 14;		// top of ditto
const int kPWinBarHeight			= 10;		// height of ditto (small variant)
const int kPWinBarMargin			= 16;		// dist from control bottom to window bottom

	// overall progress window dimensions
const int kPWinWidth				= 320;		// seems reasonable...
const int kPWinHeight				= kPWinBarTop + kPWinBarHeight + kPWinBarMargin;

	//	for download variant
const int	kPWinDLStatsOff			= 12;		// dist from prog-bar to DL stats line 1
const int	kPWinDLStatsHt			= 14;		// height of stats line 1
const int	kPWinDLStats2Off		=  6;		// dist from stats line 1 to line 2
const int	kPWinDLStats2Ht			= kPWinDLStatsHt;	// ht line 2

	// total height
const int	kPWinDLHeight			= kPWinHeight
									+ kPWinDLStatsOff  + kPWinDLStatsHt
									+ kPWinDLStats2Off + kPWinDLStats2Ht;

const uint	kProgressWindowKind		= kWKindMovableDlgBox
//									| kWKindCompositing | kWKindHighResolution
//									| kWKindStandardHandler
									;

/*
**	Internal Routines
*/
static DTSError	Update1File( DTSKeyFile *, const char *, int, int );
static DTSError	MergeVersionFile( DTSKeyFile * file, DTSFileSpec * spec );
static DTSError	OpenFind( DTSKeyFile * file, const char * fname, int fref );
static void		UpdateVersionResource( const char * name, int newversion );
static DTSError	CreateDummyImages();
static DTSError	CreateDummySounds();


/*
**	Variables
*/
ProgressWin * ProgressWin::pwin = nullptr;


/*
**	OpenKeyFiles()
**
**	open the essential keyfiles: prefs, images, sounds
*/
void
OpenKeyFiles()
{
	gImagesVersion = 0;
	gSoundsVersion = 0;
	
	DTSError result = gClientPrefsFile.Open( kClientPrefsFName,
		kKeyReadWritePerm | kKeyCreateFile, rPrefsFREF );
	if ( noErr != result )
		GenericError( _(TXTCL_UNABLE_OPEN_PREFS) );
	
	// if image or sound files are missing, make dummy ones;
	// we'll auto-update them later on.
	DTSError err;
	if ( noErr == result )
		{
		err = OpenFind( &gClientImagesFile, kClientImagesFName, rClientImagesFREF );
		if ( err == noErr )
			result = ReadFileVersion( &gClientImagesFile, &gImagesVersion );
		else
			result = CreateDummyImages();
		}
	if ( noErr == result )
		{
		err = OpenFind( &gClientSoundsFile, kClientSoundsFName, rClientSoundsFREF );
		if ( err == noErr )
			result = ReadFileVersion( &gClientSoundsFile, &gSoundsVersion );
		else
			result = CreateDummySounds();
		}
	
	SetErrorCode( result );
}


/*
**	CloseKeyFiles()
**
**	close the keyfiles
*/
void
CloseKeyFiles()
{
	gClientImagesFile.Close();
	gClientSoundsFile.Close();
	gClientPrefsFile.Close();
}


/*
**	UpdateKeyFiles()
**
**	update the images and sounds files if necessary
*/
void
UpdateKeyFiles()
{
	// we used to care about errors prior to auto-update;
	// now used only for debug
	const bool careabouterrors = false;
	
	int imagesversion = gImagesVersion >> 8;	// ignoring sub-version #s
	int soundsversion = gSoundsVersion >> 8;
	
	// brain damage for testing
//	imagesversion -= 0x200;
//	soundsversion -= 0x200;
	
	DTSError result = noErr;
	int clientversion = gPrefsData.pdClientVersion >> 8;
	
	bool bImagesFileChanged = false;	// no need to flush caches, etc.,
	bool bSoundsFileChanged = false;	// ... if file data did not change
	
	// Image file
	if ( noErr == result )
		{
		if ( imagesversion < clientversion )
			{
			result = UpdateImagesFile( clientversion );
			if ( noErr == result )
				{
				imagesversion = gImagesVersion >> 8;
				bImagesFileChanged = true;
				}
			}
		if ( not careabouterrors )
			result = noErr;
		else
		if ( result == fnfErr )
			GenericError( _(TXTCL_LOCATE_IMAGE_UPDATE) );
		}
	
	// sounds file
	if ( noErr == result )
		{
		if ( soundsversion < clientversion )
			{
			result = UpdateSoundsFile( clientversion );
			if ( noErr == result )
				{
				soundsversion = gSoundsVersion >> 8;
				bSoundsFileChanged = true;
				}
			}
		if ( not careabouterrors )
			result = noErr;
		else
		if ( result == fnfErr )
			GenericError( _(TXTCL_LOCATE_SOUND_UPDATE) );
		}
	
	// cross check with client version number
	if ( noErr == result )
		{
		char client[ 32 ];
		int minor = gPrefsData.pdClientVersion & 0x0FF;
		if ( minor )
			snprintf( client, sizeof client, "%d.%d", clientversion, minor );
		else
			snprintf( client, sizeof client, "%d", clientversion );
		
		char file[ 32 ];
		if ( imagesversion > kVersionNumber )
			{
			minor = gImagesVersion & 0x0FF;
			if ( minor )
				snprintf( file, sizeof file, "%d.%d", imagesversion, minor );
			else
				snprintf( file, sizeof file, "%d", imagesversion );
			
			// "This client (version %s) should not be used with this newer data file
			// (file \"%s\" is version %s)."
			GenericError( _(TXTCL_CLIENT_OLDER_THAN_DATA),
				client,
				kClientImagesFName,
				file );
			}
		if ( soundsversion > kVersionNumber )
			{
			minor = gSoundsVersion & 0x0FF;
			if ( minor )
				snprintf( file, sizeof file, "%d.%d", soundsversion, minor );
			else
				snprintf( file, sizeof file, "%d", soundsversion );
			
			GenericError( _(TXTCL_CLIENT_OLDER_THAN_DATA),
				client,
				kClientSoundsFName,
				file );
			}
		}
	
	if ( bImagesFileChanged  ||  bSoundsFileChanged )
		FlushCaches( true );
	if ( bImagesFileChanged )
		ChangeWindowSize();
	
	SetErrorCode( result );
}


/*
**	UpdateImagesFile()
**
**	update the images file
**	assume newversion > gImagesVersion
**	assume there is no problem with using this version of the client
**	with this version of the data.
**	ie assume that RemoveCrapFromImagesFile isn't going to 
**	delete records it should not. 
*/
DTSError
UpdateImagesFile( int newversion )
{
	DTSShowWatchCursor();
	
	DTSError result = noErr;
	//if ( noErr == result )
		{
		// close the file
		// open it for writing
		// (it was previously open read-only)
		gClientImagesFile.Close();
		result = gClientImagesFile.Open( kClientImagesFName,
			kKeyReadWritePerm | kKeyDontCreateFile );
		}
	if ( noErr == result )
		{
		int curversion = gImagesVersion >> 8;
//		curversion -= 2;  // brain damage for testing
		result = Update1File( &gClientImagesFile, kClientImagesFName, curversion, newversion );
		}
	if ( noErr == result )
		{
		// update the version number by reading it from the file
		result = ReadFileVersion( &gClientImagesFile, &gImagesVersion );
		}
	if ( noErr == result )
		{
		// fix up the file
		RemoveCrapFromImagesFile( &gClientImagesFile );
		UpdateVersionResource( kClientImagesFName, gImagesVersion );
		}
	
	// close the file again
	// open it again read-only
	// it was open for writing
	gClientImagesFile.Close();
	gClientImagesFile.Open( kClientImagesFName, kKeyReadOnlyPerm | kKeyDontCreateFile );
	
	DTSShowArrowCursor();
	
	return result;
}


/*
**	UpdateSoundsFile()
**
**	update the sounds file
**	assume newversion > gSoundsVersion
**	assume there is no problem with using this version of the client
**	with this version of the data.
**	ie assume that RemoveCrapFromSoundsFile isn't going to 
**	delete records it should not. 
*/
DTSError
UpdateSoundsFile( int newversion )
{
	DTSShowWatchCursor();
	
	DTSError result = noErr;
	//if ( noErr == result )
		{
		// close the file
		// open it for writing
		// it was open read-only
		gClientSoundsFile.Close();
		
		result = gClientSoundsFile.Open( kClientSoundsFName,
			kKeyReadWritePerm | kKeyDontCreateFile );
		}
	if ( noErr == result )
		{
		int curversion = gSoundsVersion >> 8;
//		curversion -= 2;  // brain damage for testing
		result = Update1File( &gClientSoundsFile, kClientSoundsFName, curversion, newversion );
		}
	if ( noErr == result )
		{
		// update the version number by reading it from the file
		result = ReadFileVersion( &gClientSoundsFile, &gSoundsVersion );
		}
	if ( noErr == result )
		{
		// fix up the file
		RemoveCrapFromSoundsFile( &gClientSoundsFile );
		UpdateVersionResource( kClientSoundsFName, gSoundsVersion );
		}
	
	// close the file again
	// open it again read-only
	// it was open for writing
	gClientSoundsFile.Close();
	gClientSoundsFile.Open( kClientSoundsFName, kKeyReadOnlyPerm | kKeyDontCreateFile );
	
	DTSShowArrowCursor();
	
	return result;
}


/*
**	ApplyDeleteOneFile()
**
**	helper for Update1File()
**	merges a single updater, deletes the updater if successful, and
**	updates our notion of the current version
*/
static DTSError
ApplyDeleteOneFile( DTSKeyFile *	file,
					DTSFileSpec *	dir,
					int				inVersion,
					int&			outCurVersion )
{
	DTSError result = MergeVersionFile( file, dir );
	if ( noErr == result )
		{
		outCurVersion = inVersion;
		dir->Delete();
		}
	return result;
}


/*
**	Update1File()
**
**	update either the sounds or the images file
*/
DTSError
Update1File( DTSKeyFile * file, const char * name, int curversion, int newversion )
{
	DTSFileSpec oldstyle, newstyle;
	DTSError result = noErr;
		
	// init the spec for the Downloads folder
	// (should that be /tmp/, or $TMPDIR, or one of the other somewhat temporary
	// destinations reachable via FSFindFolder()?)
	oldstyle.GetCurDir();
	newstyle.SetFileName( "Downloads" );
	DTSError checknewresult = newstyle.SetDir();
	oldstyle.SetDirNoPath();
	
	// while the current version needs to be updated
	while ( curversion < newversion )
		{
		char buff[ 256 ];
		// search for the update file that jumps from the current version
		// to the version that is closest to the new version
		for ( int tempversion = newversion; tempversion > curversion; --tempversion )
			{
			// look for an update file in the app folder
			snprintf( buff, sizeof buff,
				"%s.update.%dto%d", name, curversion, tempversion );
			oldstyle.SetFileName( buff );
			result = ApplyDeleteOneFile( file, &oldstyle, tempversion, curversion );
			
			// success? then stop this circus and move on to the next incremental update.
			// any other error (except fnf) should be returned to our caller
			// fnfErr just means we need to look for the updater using a different
			// name and/or location 
			if ( noErr == result )
				break;
			else
			if ( result != fnfErr )
				return result;
			
			// look for a new style update file name in the app folder
			snprintf( buff, sizeof buff, "%s.%dto%d", name, curversion, tempversion );
			oldstyle.SetFileName( buff );
			result = ApplyDeleteOneFile( file, &oldstyle, tempversion, curversion );
			if ( noErr == result )
				break;
			else
			if ( result != fnfErr )
				return result;
			
			// didn't find one here
			// look in the Downloads folder
			if ( checknewresult == noErr )
				{
				snprintf( buff, sizeof buff, "%s.%dto%d", name, curversion, tempversion );
				newstyle.SetFileName( buff );
				result = ApplyDeleteOneFile( file, &newstyle, tempversion, curversion );
				if ( noErr == result )
					break;
				else
				if ( result != fnfErr )
					return result;
				}
			}
		
		// did we make any progress? if so, go back and look for the next increment
		if ( noErr == result )
			continue;
		
		// OK we found nothing and are stuck at v.current, which is < v.newversion
		// maybe the autodownloader had to settle for the CD updater?
		if ( noErr == checknewresult )
			{
			snprintf( buff, sizeof buff, "%s.%dto%d", name, int(kVersionCD >> 8), newversion );
			newstyle.SetFileName( buff );
			result = ApplyDeleteOneFile( file, &newstyle, newversion, curversion );
			if ( result != noErr && result != fnfErr )
				return result;
			}
		
		// sheesh, even that failed.
		break;
		}
	
	return result;
}


#if 0
/*
**	UpdateKeyFiles()
**
**	update the images and sounds files if necessary
*/
void
UpdateKeyFiles()
{
	DTSError result;
	// if ( noErr == result )
		result = Update1File( &gClientImagesFile, kClientImagesFName,
			kClientImagesUpdateFName, RemoveCrapFromImagesFile);
	if ( noErr == result )
		result = Update1File( &gClientSoundsFile, kClientSoundsFName,
			kClientSoundsUpdateFName, RemoveCrapFromSoundsFile);
	DTSShowArrowCursor();
	SetErrorCode( result );
}


/*
**	Update1File()
**
**	update one file by merging all of the .update.x files
*/
DTSError
Update1File( DTSKeyFile * file, const char * name, const char * fname,
			DTSError (*cleaner)( DTSKeyFile * file ) )
{
	int dataversion = kVersionNumber + 1;
	DTSError result;
	
	result = ReadFileVersion( file, &dataversion );
	
	// warn about old clients & new data
	if ( noErr == result
	&&	 dataversion > kVersionNumber )
		{
		GenericError( _(TXTCL_CLIENT_OLDER_THAN_DATA), 
					  kVersionNumber,
					  name,
					  dataversion );
		return +1;
		}
	
	// apply every updater file
	if ( noErr == result
	&&	 dataversion < kVersionNumber)
		{
		DTSShowWatchCursor();
		
		file->Close();		// close it; it was read only
		file->Open( name, kKeyReadWritePerm | kKeyDontCreateFile );
		
		// update until we're done
		while ( dataversion < kVersionNumber && not result)
			{
			// look for the most recent update file
			for ( int destversion = kVersionNumber;  destversion > dataversion;  --destversion )
				{
				DTSFileSpec spec;
				snprintf( spec.fileName, sizeof spec.fileName, "%s.%dto%d",
					fname, dataversion, destversion );
				result = MergeVersionFile( file, spec.fileName );
				if ( noErr == result )
					{
					dataversion = destversion;
					spec.Delete();
					break;
					}
				if ( fnfErr != result )
					break;
				}
			}
		
		// remove obsolete data after the update
		if ( cleaner
		&&	 ( noErr == result || fnfErr == result ) )
			{
			cleaner(file);
			}
		
		// Re-open it read-only; it has been updated.
		file->Close();
		UpdateVersionResource( name );
		file->Open( name, kKeyReadOnlyPerm | kKeyDontCreateFile );
		}
	
	if ( noErr != result )
		{
		GenericError( _(TXTCL_UNABLE_UPDATE_DATA),
			kVersionNumber,
			fname, dataversion, kVersionNumber, kVersionNumber );
		}
	
	return result;
}
#endif	// 0


/*
**	MergeVersionFile()
**
**	merge this update file
*/
DTSError
MergeVersionFile( DTSKeyFile * file, DTSFileSpec * spec )
{
	long numTypes;
	DTSKeyFile source;
	int32_t version;
	DTSKeyInfo info;
	DTSError result = source.Open( spec, kKeyReadOnlyPerm | kKeyDontCreateFile );
	if ( noErr == result )
		result = source.CountTypes( &numTypes );
	if ( noErr == result )
		result = source.GetInfo( &info );
	if ( noErr == result )
		{
		// create a progress window
		const char * destName;
		if ( file == &gClientImagesFile )
			destName = _(TXTCL_UPDATING_IMAGES);		// "images"
		else
		if ( file == &gClientSoundsFile )
			destName = _(TXTCL_UPDATING_SOUNDS);		// "sounds"
		else
			destName = _(TXTCL_UPDATING_DATA);			// "data"
		
		char buff[ 256 ];
		// "Updating %s..."
		snprintf( buff, sizeof buff, _(TXTCL_UPDATING_SOMETHING), destName );
		StProgressWindow pw( buff );
		
		int doneRecords = 0;
		for ( int typeIndex = 0;  typeIndex < numTypes;  ++typeIndex )
			{
			DTSKeyType ttype;
			result = source.GetType( typeIndex, &ttype );
			if ( result != noErr )
				break;
			
			// update the version resource last!
			if ( kTypeVersion == ttype )
				continue;
			
			long numRecords;
			result = source.Count( ttype, &numRecords );
			if ( result != noErr )
				break;
			for ( int index = 0;  index < numRecords;  ++index )
				{
				pw.SetProgress( doneRecords, info.keyNumRecords );
				DTSKeyID id;
				result = source.GetID( ttype, index, &id );
				if ( result != noErr )
					break;
				size_t size;
				result = source.GetSize( ttype, id, &size );
				if ( result != noErr )
					break;
				void * data = nullptr;
				result = TaggedReadAlloc( &source, ttype, id, data, "MergeVersionFile" );
				if ( noErr == result )
					result = file->Write( ttype, id, data, size );
				delete[] static_cast<char *>( data );
				if ( result != noErr )
					break;
				++doneRecords;
				}
			if ( result != noErr )
				break;
			}
		}
	
	// write the new version number last
	if ( noErr == result )
		result = source.Read( kTypeVersion, 0, &version, sizeof version );
	if ( noErr == result )
		result = file->Write( kTypeVersion, 0, &version, sizeof version );
	
	return result;
}


/*
**	SetFileType()
**
**	set a file's file type and creator
**	(OS9 holdover)
*/
static void
SetFileType( const char * fname, int frefid )
{
	DTSFileSpec spec;
	spec.SetFileName( fname );
	spec.SetTypeCreator( frefid, rClientSignFREF );
}


/*
**	OpenFind()
**
**	try real hard to locate the file.
**	then open it.
*/
DTSError
OpenFind( DTSKeyFile * file, const char * fname, int fref )
{
	size_t speclen;
	DTSError result = file->Open( fname, kKeyReadOnlyPerm | kKeyDontCreateFile );
	if ( fnfErr == result )
		{
		// dang, didn't find it.
		// look for something that looks promising
		DTSFileSpec spec, zipSpec;
		size_t len = std::strlen( fname );
		bool bHaveZip = false;
		DTSFileIterator iter;
		while ( iter.Iterate( spec ) )
			{
			// bail (w/error) when no more candidates (or filesystem problem)
			if ( result != noErr )
				break;
			
			// skip files that don't match
			if ( std::memcmp( spec.GetFileName(), fname, len ) )
				continue;
			
			// check for gzips
			speclen = std::strlen( spec.GetFileName() );
			if ( std::memcmp( spec.GetFileName() + speclen - 3, ".gz", 3 ) == 0 )
				{
				zipSpec = spec;
				bHaveZip = true;
				continue;
				}
			
			// abort (w/noErr) if we found a (non-.gz) match
			break;
			}
		
		// did we find a zip file, but no better candidate?
		if ( result != noErr )
			{
			if ( bHaveZip /* zipindex >= 0 */ )
				{
				// expand it
				spec = zipSpec;
				if ( noErr == result )
					result = ExpandFile( &spec );
				}
			}
		
		// rename it, if necessary
		if ( noErr == result )
			{
			if ( std::strcmp( fname, spec.GetFileName() ) )
				result = spec.Rename( fname );
			}
		
		// open the file if we found one
		if ( noErr == result )
			result = file->Open( fname, kKeyReadOnlyPerm | kKeyDontCreateFile );
		
		// reset the file not found error
		if ( result != noErr )
			result = fnfErr;
		}
	
	if ( noErr == result )
		SetFileType( fname, fref );
	
	return result;
}


/*
**	FillVersRec()
**	fill out the fields of a VersRec structure
**	another OS9 holdover
*/
static size_t
FillVersRec( int vers, VersRec& rec )
{
	int major = vers >> 8;
	uchar hiVersBCD = ( 16 * (major / 1000 % 10) + (major / 100 % 10) );
	uchar loVersBCD = ( 16 * (major /   10 % 10) + (major       % 10) );
	
	rec.numericVersion.majorRev			= hiVersBCD;
	rec.numericVersion.minorAndBugRev	= loVersBCD;
	rec.numericVersion.stage			= finalStage;
	rec.numericVersion.nonRelRev		= 0;
	rec.countryCode						= verUS;
	
	// first write the short version string
	size_t l1;
	char * dst = reinterpret_cast<char *>( rec.shortVersion ) + 1;
	int minor = vers & 0x0FF;
	if ( minor )
		l1 = snprintf( dst, sizeof rec.shortVersion - 1, "v%d.%d", major, minor );
	else
		l1 = snprintf( dst, sizeof rec.shortVersion - 1, "v%d", major );
	rec.shortVersion[0] = static_cast<uchar>( l1 );
	
	// advance to the start of the long string
	dst += l1; 
	
	// write that (leaving room for its length byte)
	size_t l2 = snprintf( dst + 1, sizeof rec.shortVersion - l1 - 2,
			"Version %d", kVersionNumber );
	
	// stash the 2nd length byte where it belongs
	* (uchar *) dst = static_cast<uchar>( l2 );
	
	// final size
	size_t len = offsetof(VersRec, shortVersion) + l1 + l2 + 2;
	return len;
}

	
/*
**	UpdateVersionResource()
**
**	add or update the file's 'vers' resource
**	OS9 forever! (Just kidding)
*/
void
UpdateVersionResource( const char * name, int newversion )
{
	// apologies in advance
	// not much in DTSLib appears to help with resource-wrangling
	SInt16 prevResFile = ::CurResFile();
	DTSError result = noErr;

	// follow aliases if need be
	FSRef itsFile, parentRef;
	HFSUniStr255 uFileName;
	
	{
	DTSFileSpec fss;
	fss.SetFileName( name );
	DTSFileSpec_CopyToFSRef( &fss, &itsFile );
	}
	
	// make sure it has the right creator code
	{
	FSCatalogInfo catinfo;
	FileInfo& itsInfo = * reinterpret_cast<FileInfo *>( catinfo.finderInfo );
	result = FSGetCatalogInfo( &itsFile, kFSCatInfoFinderInfo, &catinfo,
				&uFileName, nullptr, &parentRef );
	__Check_noErr( result );
	if ( noErr == result
	&&   gDTSApp->GetSignature() != itsInfo.fileCreator )
		{
		itsInfo.fileCreator = gDTSApp->GetSignature();
		result = FSSetCatalogInfo( &itsFile, kFSCatInfoFinderInfo, &catinfo );
		__Check_noErr( result );
		}
	}
	
	// open the file's resource fork
	HFSUniStr255 resForkName;
	__Verify_noErr( FSGetResourceForkName( &resForkName ) );
	SInt16 refNum = -1;
	result = FSOpenResourceFile( &itsFile, resForkName.length, resForkName.unicode,
				fsRdWrPerm, &refNum );
	__Check_noErr( result );
	if ( -1 == refNum )
		{
		// or make one if need be
		result = FSCreateResourceFile( &parentRef, uFileName.length, uFileName.unicode,
					kFSCatInfoNone, nullptr,
					resForkName.length, resForkName.unicode, &itsFile, nullptr );
		__Check_noErr( result );
		if ( noErr == result )
			{
			result = FSOpenResourceFile( &itsFile, resForkName.length, resForkName.unicode,
						fsRdWrPerm, &refNum );
			__Check_noErr( result );
			}
		
		// spit up on errors
		if ( noErr != result )
			return;
		}
	::UseResFile( refNum );
	
	{
	// cobble up a new version resource structure
	VersRec newVers;
	size_t len = FillVersRec( newversion, newVers );
	
	// get the existing resource, if there is one
	Handle h = ::Get1Resource( 'vers', 1 );
	if ( nullptr == h )
		{
		// none existed; we have to create one
		result = ::PtrToHand( &newVers, &h, len );
		if ( noErr == result )
			::AddResource( h, 'vers', 1, "\p" );
		}
	else
		{
		// got one; change it in situ
		result = ::PtrToXHand( &newVers, h, len );
		if ( noErr == result )
			::ChangedResource( h );
		}
	}
	
	::CloseResFile( refNum );
	::UseResFile( prevResFile );
}


/*
**	CreateDummyImages()
**
**	create a dummy images file
*/
DTSError
CreateDummyImages()
{
	// create the file
	DTSError result = gClientImagesFile.Open( kClientImagesFName,
		kKeyReadWritePerm, rClientImagesFREF );
	if ( result != noErr )
		return result;
	
	// write a version number
	int32_t version = 0;	// endian OK
	if ( noErr == result )
		result = gClientImagesFile.Write( kTypeVersion, 0, &version, sizeof version );
	
	// write large-window layout
	Layout layout;
	if ( noErr == result )
		{
		layout.layoWinHeight                 = 557;
		layout.layoWinWidth                  = 795;
		layout.layoPictID                    = 4;
		layout.layoFieldBox.rectTop          = 8;
		layout.layoFieldBox.rectLeft         = 240;
		layout.layoFieldBox.rectBottom       = 548;
		layout.layoFieldBox.rectRight        = 787;
		layout.layoInputBox.rectTop          = 428;
		layout.layoInputBox.rectLeft         = 8;
		layout.layoInputBox.rectBottom       = 548;
		layout.layoInputBox.rectRight        = 232;
		layout.layoHPBarBox.rectTop          = 10;
		layout.layoHPBarBox.rectLeft         = 104;
		layout.layoHPBarBox.rectBottom       = 20;
		layout.layoHPBarBox.rectRight        = 229;
		layout.layoSPBarBox.rectTop          = 39;
		layout.layoSPBarBox.rectLeft         = 104;
		layout.layoSPBarBox.rectBottom       = 48;
		layout.layoSPBarBox.rectRight        = 229;
		layout.layoAPBarBox.rectTop          = 25;
		layout.layoAPBarBox.rectLeft         = 104;
		layout.layoAPBarBox.rectBottom       = 34;
		layout.layoAPBarBox.rectRight        = 229;
		layout.layoTextBox.rectTop           = 62;
		layout.layoTextBox.rectLeft          = 12;
		layout.layoTextBox.rectBottom        = 417;
		layout.layoTextBox.rectRight         = 228;
		layout.layoLeftObjectBox.rectTop     = 8;
		layout.layoLeftObjectBox.rectLeft    = 8;
		layout.layoLeftObjectBox.rectBottom  = 50;
		layout.layoLeftObjectBox.rectRight   = 50;
		layout.layoRightObjectBox.rectTop    = 8;
		layout.layoRightObjectBox.rectLeft   = 51;
		layout.layoRightObjectBox.rectBottom = 50;
		layout.layoRightObjectBox.rectRight  = 93;

#if DTS_LITTLE_ENDIAN
		SwapEndian( layout );
#endif
		result = gClientImagesFile.Write( kTypeLayout, 0 /*kLargeLayoutID*/,
			&layout, sizeof layout );
		}

#if 0	// no more small window
	if ( noErr == result )
		{
		layout.layoWinHeight                 = 437;
		layout.layoWinWidth                  = 635;
		layout.layoPictID                    = 211;
		layout.layoFieldBox.rectTop          = 8;
		layout.layoFieldBox.rectLeft         = 206;
		layout.layoFieldBox.rectBottom       = 428;
		layout.layoFieldBox.rectRight        = 626;
		layout.layoInputBox.rectTop          = 309;
		layout.layoInputBox.rectLeft         = 8;
		layout.layoInputBox.rectBottom       = 428;
		layout.layoInputBox.rectRight        = 198;
		layout.layoHPBarBox.rectTop          = 10;
		layout.layoHPBarBox.rectLeft         = 104;
		layout.layoHPBarBox.rectBottom       = 20;
		layout.layoHPBarBox.rectRight        = 196;
		layout.layoSPBarBox.rectTop          = 39;
		layout.layoSPBarBox.rectLeft         = 104;
		layout.layoSPBarBox.rectBottom       = 48;
		layout.layoSPBarBox.rectRight        = 196;
		layout.layoAPBarBox.rectTop          = 25;
		layout.layoAPBarBox.rectLeft         = 104;
		layout.layoAPBarBox.rectBottom       = 34;
		layout.layoAPBarBox.rectRight        = 196;
		layout.layoTextBox.rectTop           = 62;
		layout.layoTextBox.rectLeft          = 12;
		layout.layoTextBox.rectBottom        = 297;
		layout.layoTextBox.rectRight         = 194;
		layout.layoLeftObjectBox.rectTop     = 8;
		layout.layoLeftObjectBox.rectLeft    = 8;
		layout.layoLeftObjectBox.rectBottom  = 50;
		layout.layoLeftObjectBox.rectRight   = 50;
		layout.layoRightObjectBox.rectTop    = 8;
		layout.layoRightObjectBox.rectLeft   = 51;
		layout.layoRightObjectBox.rectBottom = 50;
		layout.layoRightObjectBox.rectRight  = 93;
		
# if DTS_LITTLE_ENDIAN
		SwapEndian( layout );
# endif
		result = gClientImagesFile.Write( kTypeLayout, 1 /*kSmallLayoutID*/,
			&layout, sizeof layout );
		}
#endif  // 0
	
	// close it and re-open it read-only
	gClientImagesFile.Close();
	if ( noErr == result )
		result = gClientImagesFile.Open( kClientImagesFName,
			kKeyReadOnlyPerm | kKeyDontCreateFile );
	
	// update the version resource
	if ( noErr == result )
		UpdateVersionResource( kClientImagesFName, 0 );
	
	// completely bail by deleting the partially created file
	if ( result != noErr )
		{
		DTSFileSpec spec;
		spec.SetFileName( kClientImagesFName );
		spec.Delete();
		}
	
	return result;
}


/*
**	CreateDummySounds()
**
**	create a dummy sounds file
*/
DTSError
CreateDummySounds()
{
	// create the file
	DTSError result = gClientSoundsFile.Open( kClientSoundsFName,
		kKeyReadWritePerm, rClientSoundsFREF );
	if ( result != noErr )
		return result;
	
	// write a version number
	int32_t version = 0;	// endian OK
	if ( noErr == result )
		result = gClientSoundsFile.Write( kTypeVersion, 0, &version, sizeof version );
	
	// close it and re-open it read-only
	gClientSoundsFile.Close();
	if ( noErr == result )
		result = gClientSoundsFile.Open( kClientSoundsFName,
			kKeyReadOnlyPerm | kKeyDontCreateFile );
	
	// update the version resource
	if ( noErr == result )
		UpdateVersionResource( kClientSoundsFName, 0 );
	
	// completely bail by deleting the partially created file
	if ( result != noErr )
		{
		DTSFileSpec spec;
		spec.SetFileName( kClientSoundsFName );
		spec.Delete();
		}
	
	return result;
}

#pragma mark -
// Progress window stuff

/*
**	ProgressWin::SetProgress()
**	update the progress meter
*/
void
ProgressWin::SetProgress( ulong curr, ulong limit )
{
	// setup the DL statistics
	if ( pwin && pwin->label1 )
		pwin->UpdateDLStats( curr, limit );
	
	if ( 0 == limit )
		limit = 1;			// don't divide by 0
	if ( curr > limit )
		curr = limit;		// don't be absurd
	
	// set progress-control's value & limit
	int curVal = -1;
	if ( pwin && pwin->progressCtl )
		{
		__Verify_noErr( HIViewSetMaximum( pwin->progressCtl, limit ) );
		curVal = HIViewGetValue( pwin->progressCtl );
		__Verify_noErr( HIViewSetValue( pwin->progressCtl, curr ) );
		}
	
	// update meter if value changes
	if ( pwin && long(curr) != curVal )
		{
		// suppress revealing the window, unless:
		//	   a. this operation seems to be taking a while,
		//		[viz.: if, after 0.5 seconds, the operation has not yet reached 80% complete]
		// OR, b. this is a download window
		
		const EventTimeout kRevealDelay = 0.5F;  // in seconds
		if ( not pwin->GetVisible()
		&&	 ( (pwin->Age() >= kRevealDelay && (curr * 10) <= (limit * 8)) || pwin->label1 ) )
			{
			// OK, reveal it
			pwin->Show();
			}
		else
			{
			// either the window is already visible, or does not yet need to be.
			
			// normally I'd put up a watch cursor here (animated, even)
			// but CLApp::Idle() will step all over it
//			AdvanceWaitCursor();
			
			// claw ourselves back to the front if need be
			if ( pwin->GetVisible()
			&&	 pwin != gDTSApp->GetFrontWindow() )
				{
				pwin->GoToFront();
				}
			}
		}
	
	// avoid polling for events too often
	static CFAbsoluteTime timeCounter;
	if ( CFAbsoluteTimeGetCurrent() > timeCounter )
		{
		// ... not more than every 0.25 seconds
		timeCounter = 0.25 + CFAbsoluteTimeGetCurrent();
		
		// drive the main event loop once
		RunApp();
		}
}


/*
**	ProgressWin::Setup()		[static]
**	create the window (if it doesn't already exist)
*/
void
ProgressWin::Setup( const char * title )
{
	// erase a prior one if any (shouldn't normally happen)
	if ( pwin )
		Dispose();
	
	// create the new one
	pwin = NEW_TAG("ProgressWin") ProgressWin;
	if ( not pwin )
		return;
	
	// remember previous app sleep time
	pwin->prevSleep = gDTSApp->SetSleepTime( 0 );
	
	// create the actual Mac window
	DTSError result = pwin->Init( kProgressWindowKind );
	if ( noErr != result )
		return;
	
	// set title & chrome
	pwin->SetTitle( title );
	__Verify_noErr( SetThemeWindowBackground( DTS2Mac(pwin),
						kThemeBrushModelessDialogBackgroundActive, false ) );
	
	// set size & position
	pwin->Size( kPWinWidth, kPWinHeight );
	__Verify_noErr( RepositionWindow( DTS2Mac(pwin), nullptr, kWindowAlertPositionOnMainScreen ) );
	
	// install the meter
	pwin->InstallControls();
	
	// leave window invisible until shown
}


/*
**	ProgressWin::SetupDL()		[static]
**	This is the download version of the above.
**	[Assumes OSX 10.2 or better, because that's the only case where this can be called.
**	see comment at 'StProgressWindow::StProgressWindow( const HFSUniStr255& )'.]
*/
void
ProgressWin::SetupDL( CFStringRef fname )
{
	// erase a prior one if any (shouldn't normally happen)
	if ( pwin )
		Dispose();
	
	// create the new one
	pwin = NEW_TAG("ProgressWin") ProgressWin;
	if ( not pwin )
		return;
	
	// remember previous app sleep time
	// set new sleep to 20 ticks (versus 0, above), because in this case we'd rather
	// spend as much time as possible blocked in WNE(), while the download progresses
	// asynchronously. In the other case, it's the reverse: we want to grab as much
	// time as possible to do the underlying lengthy chore which the progress bar
	// is measuring; i.e. any time we give up to WNE() is a waste, as far as we're concerned.
	
	pwin->prevSleep = gDTSApp->SetSleepTime( 20 );
	
	// create the actual Mac window
	DTSError result = pwin->Init( kProgressWindowKind );
	if ( noErr != result )
		return;
	
	// set title & chrome
	WindowRef win = DTS2Mac(pwin);
	if ( CFStringRef fmt = CFCopyLocalizedString( CFSTR("Downloading \"%@\""),
							CFSTR("title for download-progress dialog") ) )
		{
		if ( CFStringRef title = CFStringCreateFormatted( fmt, fname ) )
			{
			__Verify_noErr( SetWindowTitleWithCFString( win, title ) );
			CFRelease( title );
			}
		CFRelease( fmt );
		}
	
	__Verify_noErr( SetThemeWindowBackground( win,
					kThemeBrushModelessDialogBackgroundActive, false ) );
	
	// set size & position
	pwin->Size( kPWinWidth, kPWinDLHeight );
	__Verify_noErr( RepositionWindow( win, nullptr, kWindowAlertPositionOnMainScreen ) );
	
	// install the meter
	pwin->InstallControls();
	
	// install the other controls:
	// bounds of label box
	Rect r;
	r.top    = kPWinBarTop + kPWinBarHeight + kPWinDLStatsOff;
	r.left   = kPWinHorzInset;
	r.bottom = r.top + kPWinDLStatsHt;
	r.right  = kPWinWidth - kPWinHorzInset;
	
	// use smaller text size for the static text labels, and center them
	const ControlFontStyleRec cfsr =
		{
		kControlUseFontMask | kControlUseJustMask,		// flags
		kControlFontSmallSystemFont,					// font
		0, 0, 0,										// style, size, mode
		teJustCenter,									// just
		{ 0, 0, 0 },
		{ 0, 0, 0 }
		};
	
	// make the static-text controls, initially empty: first the "x of y (zKB/s)" line ...
	__Verify_noErr( CreateStaticTextControl( win, &r, CFSTR(""), &cfsr, &pwin->label1 ) );
	
	// ... and the "time remaining" one, below it
//	OffsetRect( &r, 0, kPWinDLStats2Off + kPWinDLStats2Ht );
	r.bottom += kPWinDLStats2Off + kPWinDLStats2Ht;
	r.top    += kPWinDLStats2Off + kPWinDLStats2Ht;
	
	__Verify_noErr( CreateStaticTextControl( win, &r, CFSTR(""), &cfsr, &pwin->label2 ) );
}


/*
**	SetMenuModality()
**
**	adjust the menu bar: if progress window is up, disable all menus
**	else re-enable them
*/
static void
SetMenuModality( bool bIsModal )
{
	DTSMenu * menu = gDTSMenu;
	int flag = bIsModal ? kMenuDisabled : kMenuNormal;
	
	menu->SetFlags( MakeLong( rAppleMenuID, 0 ), flag );
	menu->SetFlags( MakeLong( rFileMenuID, 0 ), flag );
	menu->SetFlags( MakeLong( rEditMenuID, 0 ), flag );
	menu->SetFlags( MakeLong( rOptionsMenuID, 0 ), flag );
	menu->SetFlags( MakeLong( rCommandMenuID, 0 ), flag );
#ifdef DEBUG_VERSION
	menu->SetFlags( MakeLong( rDebugMenuID, 0 ), flag );
	menu->SetFlags( MakeLong( rGodMenuID, 0 ), flag );
#endif	// DEBUG_VERSION
}


/*
**	ProgressWin::Show()			[ virtual ]
**	reveal the window and put application into a modal state
*/
void
ProgressWin::Show()
{
	// call thru
	DTSWindow::Show();
	
	// make our window app-modal
	__Verify_noErr( SetWindowModality( DTS2Mac(this), kWindowModalityAppModal, nullptr ) );
	
	DTSShowWatchCursor();
	
	// disable every menu
	SetMenuModality( true );
}


/*
**	ProgressWin::Dispose()			[ static ]
**	get rid of the damn thing
*/
void
ProgressWin::Dispose()
{
	if ( pwin )
		{
		// revert everything we touched
		gDTSApp->SetSleepTime( pwin->prevSleep );
		
		__Verify_noErr( SetWindowModality( DTS2Mac(pwin), kWindowModalityNone, nullptr ) );
		
		// make it go away
		pwin->Close();
		delete pwin;
		pwin = nullptr;
		
		// re-enable every menu
		SetMenuModality( false );
		}
	DTSShowArrowCursor();
}


/*
**	ProgressWin::InstallControls()			[ static ]
**	create the progress bar
*/
OSStatus
ProgressWin::InstallControls()
{
	// safety check
	if ( not pwin )
		return memFullErr;
	
	// bounds of progress bar
	const Rect r =
		{
		kPWinBarTop,	// tlbr
		kPWinHorzInset,
		kPWinBarTop + kPWinBarHeight,
		kPWinWidth - kPWinHorzInset
		};
	
	// create the progress bar control
	OSStatus err = CreateProgressBarControl( DTS2Mac(pwin), &r,
						0, 0, 100, false,	// value; min; max; not-indeterminate
						&pwin->progressCtl );
	__Check_noErr( err );
	
	return err;
}

//	newly-added stuff for the download-progress variant.

/*
**	StProgressWindow ctor, download variant: takes a filename
**	because I needed a different signature than the
**	'const char *' ctor to distinguish the two flavors.
*/
StProgressWindow::StProgressWindow( const HFSUniStr255& fname )
{
	if ( not fname.length )
		return;
	
	if ( CFStringRef cname = FSCreateStringFromHFSUniStr( kCFAllocatorDefault, &fname ) )
		{
		ProgressWin::SetupDL( cname );
		CFRelease( cname );
		}
}


/*
**	BytesToEnglish()
**	converts numbers into an abbreviated form, for display purposes
*/
static void
BytesToEnglish( ulong num, char * buffer, size_t buflen )
{
	// For present purposes, 1K is 1024 (2^10) and 1M is 1048576 (2^20).
	// Yeah yeah, I know about the 1000-vs-1024 controversy.
	// Maybe I was in a little bit of a hurry here :)
	// If you have a better idea, please feel free to fix this.
	
	// more than 1MB: use "M".
	// (We do not ever expect to see files with sizes >= 1GB.
	// In fact, the largest value typically encountered would be ~70MB, for CL_Images.)
	// [However, this does get called with values that are not file sizes, e.g. 'bps' below...]
	if ( num > (1024 * 1024) )
		snprintf( buffer, buflen, "%.2fM", float( num ) / float( 1024 * 1024 ) );
	else
	// 9K or more: use "K"
	if ( num >= (9 * 1024) )
		snprintf( buffer, buflen, "%.1fK", float( num ) / 1024.0F );
	else
	// if > 1000, insert the comma
	// TODO: use printf's "'" format modifier? or use a CFNumberFormatter?
	if ( num >= 1000 )
		snprintf( buffer, buflen, "%d" _(TXTCL_PROGRESS_THOUSAND_SEPARATOR) "%.3d",
			int(num/1000), int(num % 1000) );
	else
	// below 1000: just use bytes
		snprintf( buffer, buflen, "%d", int(num) );
}


/*
**	ProgressWin::UpdateDLStats()
**
**	recalculate the download statistics, and update the UI accordingly --
**	basically everything except the progress bar itself.
**
**	Since we know when the window was born, we can calculate the overall bytes/second.
**	From that, we can also deduce the time remaining.
*/
void
ProgressWin::UpdateDLStats( ulong cur, ulong lim ) const
{
	// we'll assemble the messages from smaller pieces
	// all of these guys are pretty short, e.g. "3,456K" or "2:23:45".
	char curSize[ 16 ], totSize[ 16 ], speedBuf[ 16 ], timeBuf[ 16 ];
	
	// abbreviate 'cur' and 'lim' values into displayable strings
	BytesToEnglish( cur, curSize, sizeof curSize );
	BytesToEnglish( lim, totSize, sizeof totSize );
	
	// figure bytes per second. don't divide by 0.
	CFTimeInterval age = Age();
	float bps;
	if ( age > 0 )
		bps = float(cur) / age;
	else
		bps = 0;
	
	// translate BPS into displayable form too
	// (mustn't forget to add the "B/s" units suffix, later on)
	BytesToEnglish( ulong(bps), speedBuf, sizeof speedBuf );
	
	// figure remaining time. again, don't divide by 0
	// if things are -really- slow, we can't even begin to estimate it
	ulong timeLeft;
	ulong remaining = lim - cur;
	if ( bps > 1 )
		timeLeft = remaining / bps;
	else
		timeLeft = ULONG_MAX;		// "a long time"
	
	// convert to H:MM:SS
	int hours = timeLeft / (60 * 60);
	timeLeft -= (60 * 60 * hours);
	int minutes = timeLeft / 60;
	timeLeft -= minutes * 60;		// ... which leaves just the seconds
//	int seconds = timeLeft;
	
	// make HMS string
	if ( hours >= 10 )
		{
		// punt if excessively huge (including ULONG_MAX case above)
		std::strcpy( timeBuf, "---" );
		}
	else
	if ( hours > 0 )
		snprintf( timeBuf, sizeof timeBuf, "%d:%.2d:%.2d", hours, minutes, int(timeLeft) );
	else
		// don't bother showing zero hours
		snprintf( timeBuf, sizeof timeBuf, "%d:%.2d", minutes, int(timeLeft) );
	
	CFStringRef fmt, buf;
	
	// now put it all together
	// first line looks like... "32K of 1.22M (90KB/s)"
	// tweak the .strings file if you prefer, say, "o" rather than "B" for "octets" vs. bytes.
	fmt = CFCopyLocalizedString( CFSTR("%s of %s (%sB/s)"),
						CFSTR("format for 1st caption in download-progress dialog") );
	if ( fmt )
		{
		buf = CFStringCreateFormatted( fmt, curSize, totSize, speedBuf );
		if ( buf )
			{
			__Verify_noErr( HIViewSetText( label1, buf ) );
			CFRelease( buf );
			}
		CFRelease( fmt );
		}
	
	// second line looks like "Estimated time remaining: 00:15:04"
	// feel free to make this fancier, e.g. "about 14 minutes", "less than 10 seconds", etc...
	fmt = CFCopyLocalizedString( CFSTR("Estimated time remaining: %s"),
					CFSTR("label for 2nd caption in download-progress dialog") );
	if ( fmt )
		{
		buf = CFStringCreateFormatted( fmt, timeBuf );
		if ( buf )
			{
			__Verify_noErr( HIViewSetText( label2, buf ) );
			CFRelease( buf );
			}
		CFRelease( fmt );
		}
	
	UpdateControls( DTS2Mac(pwin), nullptr );	// not if it's compositing, though, right?
}

