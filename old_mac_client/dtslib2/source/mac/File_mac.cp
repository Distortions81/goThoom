/*
**	File_mac.cp		dtslib2
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

#include <sys/param.h>
#include <unistd.h>

#include "File_mac.h"
#include "Shell_mac.h"


/*
**	Entry Classes and Routines
**
**	class DTSImplementFixed<DTSFileSpecPriv>;
**	class DTSFileSpecPriv;
**	
**	CountFiles();
**	GetFileType();
**	InitCurVolDir();
**	DTSFileSpec_CopyTo();
**	DTSFileSpec_CopyFrom();
**	DTSFileSpec_CopyToFSRef();
**	DTSFileSpec_CopyFromFSRef()
**
**	DTS_create();
**	DTS_open();
**	DTS_close();
**	DTS_seek();
**	DTS_seteof();
**	DTS_geteof();
**	DTS_read();
**	DTS_write();
*/

// consider reimplementing DTSFileSpecPriv in terms of CFURLRefs

/*
**	Definitions
*/

//	file "type" for directories
const OSType	kFileTypeDirectory		= 'fldr';

// used in ResolvePaths()
enum
{
	kNameInSpec,	// basename is in the DTSFileSpecPriv
	kNameInPath,	// basename is in local buffer
	kNameInFSS		// basename is in the FSRef [implicitly]
};


/*
**	Internal Routines
*/
static inline DTSError	ResolvePaths( DTSFileSpecPriv * spec, bool bResolveLastAlias = true );

static bool			GetNavDialog(
						OSType			fType,
						AEDesc *		reply,
						const char *	prompt,
						UInt32			prefsKey );
static OSStatus		PutNavDialog(
						FSRef *			parDir,
						HFSUniStr255 *	ioName,
						const char *	prompt,
						UInt32			prefsKey );

static void			StringToHFSUniStr255(
						const char * text,			// always MacRoman encoded
						HFSUniStr255 * result );
					
static void			HFSUniStr255ToString(
						const HFSUniStr255 * text,
						char * outString,		// MacRoman (if possible)
						size_t outLen );


/*
**	Internal Variables
*/
// DTSLib has a notion of a "current directory", similar -- but not identical -- to
// either the Unix getcwd(3) or old-HFS's HSetVol().
// Under HFS+, DTSLib's current directory can be represented by an FSRef.
static FSRef			gCurrDir;
static NavEventUPP		sNavEventProc;


/*
**	InitCurVolDir()
**
**	initialize the current volume and directory
**	to be this application's folder
*/
DTSError
InitCurVolDir()
{
	const ProcessSerialNumber psn = { 0, kCurrentProcess };
	
	// we are a bundled app
	FSRef fsr;
	OSStatus result = GetProcessBundleLocation( &psn, &fsr );
	__Check_noErr( result );
	if ( noErr == result )
		{
		// get parent directory of the app bundle
		result = FSGetCatalogInfo( &fsr, kFSCatInfoNone, nullptr, nullptr, nullptr, &gCurrDir );
		__Check_noErr( result );
		}
	return result;
}


/*
**	DTSFileSpecPriv
**
**	implement this class as quickly and as quietly as possible.
*/
DTSDefineImplementFirm(DTSFileSpecPriv)


/*
**	DTSFileSpecPriv::DTSFileSpecPriv()
**
**	constructor
*/
DTSFileSpecPriv::DTSFileSpecPriv() :
	fileDirectory(),
	fileVolume(),
	mState( fsUnresolved )		// trivially
{
	fileName[0] = '\0';
}


/*
**	DTSFileSpecPriv::DTSFileSpecPriv()
**
**	constructor
*/
DTSFileSpecPriv::DTSFileSpecPriv( const DTSFileSpecPriv & rhs ) :
	fileDirectory( rhs.fileDirectory ),
	fileVolume( rhs.fileVolume ),
	mState( rhs.mState )
{
	StringCopySafe( fileName, rhs.fileName, sizeof fileName );
}


/*
**	DTSFileSpecPriv::operator=()
**
**	assignment
*/
DTSFileSpecPriv &
DTSFileSpecPriv::operator=( const DTSFileSpecPriv & rhs )
{
	if ( &rhs != this )
		{
		fileDirectory = rhs.fileDirectory;
		fileVolume = rhs.fileVolume;
		mState = rhs.mState;
		StringCopySafe( fileName, rhs.fileName, sizeof fileName );
		}
	return *this;
}


/*
**	DTSFileSpecPriv::~DTSFileSpecPriv()		[destructor]
**	in header file, as (empty) inline
*/
/*
DTSFileSpecPriv::~DTSFileSpecPriv()
{
}
*/


/*
**	DTSFileSpec::SetFileName()
**
**	set the file name (and path optionally)
*/
DTSError
DTSFileSpec::SetFileName( const char * fname )
{
	__Check( fname );
	if ( not fname )
		return paramErr;
	
	DTSFileSpecPriv * p = priv.p;
	__Check( p );
	if ( not p )
		return -1;
	
	p->SetFileName( fname );
	return noErr;
}


/*
**	DTSFileSpecPriv::SetFileName()
**
**	set the file name, from a C string
**
**	normally, changing the name causes the filespec to become, at best, partially-resolved.
**	for operations that are _known_ to be resolution-neutral, you can
**	pass 'true' for the bStillResolved argument.
*/
void
DTSFileSpecPriv::SetFileName( const char * fname, bool bStillResolved /* = false */ )
{
	StringCopySafe( fileName, fname, sizeof fileName );
	mState = bStillResolved ?
				( fileName[0] ? fsPartial : fsResolved )
				: fsUnresolved;
}


/*
**	DTSFileSpecPriv::SetFileName()
**
**	set the file name, from a Unicode string
**
**	See previous comment about bStillResolved.
*/
void
DTSFileSpecPriv::SetFileName( ConstHFSUniStr255Param fname, bool bStillResolved /* = false */ )
{
	// translate to C string (we assume MacRoman)
	HFSUniStr255ToString( fname, fileName, sizeof fileName );
	mState = bStillResolved ?
				( fname->length ? fsPartial : fsResolved )
				: fsUnresolved;
}


/*
**	DTSFileSpec::GetFileName()
**
**	return the file name
*/
const char *
DTSFileSpec::GetFileName() const
{
	if ( const DTSFileSpecPriv * p = priv.p )
		return p->GetFileName();
	
	return nullptr;
}


/*
**	DTSFileSpec::GetCurDir()
**
**	make this filespec point to the "current" directory, as per SetDir[NoPath]().
**	
**	Traditionally this routine used to put the current dir's NAME into the filespec too
**	-- which is really the wrong thing to do.
**	Now we leave the name empty; all the necessary info is in the vRefNum and dirID.
**	
**	We maybe ought to have a separate DTSFileSpec::GetDirectoryName() method?
*/
DTSError
DTSFileSpec::GetCurDir()
{
	DTSFileSpecPriv * p = priv.p;
	__Check( p );
	if ( not p )
		return -1;
	
	FSCatalogInfo info;
	OSStatus err = FSGetCatalogInfo( &gCurrDir,
					kFSCatInfoVolume | kFSCatInfoNodeID,
					&info, nullptr, nullptr, nullptr );
	__Check_noErr( err );
	if ( noErr == err )
		{
		p->SetVolDir( info.volume, info.nodeID );
		p->SetFileName( "", true /* p->IsResolved() */ );
		}
	return err;
//	return p->CopyFromRef( &gCurrDir );
}


/*
**	DTSFileSpec::SetDirNoPath()
**
**	set the current volume and directory
**	ignore anything in fileName
**
**	This function probably owes its existence to the traditional misfeature of GetCurDir(),
**	mentioned above.  Now that that's been fixed, SetDir() probably works in all cases,
**	and this function could very likely be excised as redundant.  But right now,
**	SetDir() is implemented in terms of SetDirNoPath(), so maybe not.
*/
DTSError
DTSFileSpec::SetDirNoPath() const
{
	const DTSFileSpecPriv * p = priv.p;
	__Check( p );
	if ( not p )
		return -1;
	
	FSVolumeRefNum vol = p->GetVolume();
	UInt32 dir = p->GetDirectory();
	
	if ( vol )	// paranoia
		{
		__Verify_noErr( FSResolveNodeID( vol, dir, &gCurrDir ) );
		}
	
	return noErr;
}


/*
**	DTSFileSpec::SetDir()
**
**	resolve any paths in fileName
**	set the current directory
*/
DTSError
DTSFileSpec::SetDir()
{
	DTSFileSpecPriv * p = priv.p;
	__Check( p );
	if ( not p )
		return -1;
	
	DTSError result = ResolvePaths( p );
	if ( noErr == result )
		{
		// after successful resolution, the volume & directory portions
		// of the filename should now be encoded in the vRefNum and dirID.
		// So, if the filename isn't empty, we have ipso facto failed to 
		// fully resolved the desired path, which is an error.
		if ( p->GetFileName()[0] )
			result = -1;
		}
	if ( noErr == result )
		result = SetDirNoPath();
	
	return result;
}


/*
**	DTSCountFiles()
**
**	return the number of files in the current directory,
**	or a negative error code
*/
int
CountFiles()
{
	int count = 0;
	
	FSCatalogInfo info;
	OSStatus result = FSGetCatalogInfo( &gCurrDir,
					kFSCatInfoNodeFlags | kFSCatInfoValence,
					&info,
					nullptr,
					nullptr,
					nullptr );
	__Check_noErr( result );
	if ( noErr == result )
		{
		if ( info.nodeFlags & kFSNodeIsDirectoryMask )
			count = static_cast<int>( info.valence );
		else
			result = errFSNotAFolder;	// "should never happen..."
		}
	
	if ( result < 0 )
		count = result;
	
	return count;
}


/*
**	class DTSFileIteratorPriv
**	abstract base class for private file-iterators
*/
class DTSFileIteratorPriv
{
public:
			DTSFileIteratorPriv();
			~DTSFileIteratorPriv();
	
	bool	Iterate( DTSFileSpecPriv& outSpec ) const;
	
	FSIterator		mIterator;
};


/*
**	DTSFileIteratorPriv::DTSFileIteratorPriv()
**	constructor
**	begins an iteration of the current default directory.
*/
DTSFileIteratorPriv::DTSFileIteratorPriv()
	: mIterator( nullptr )
{
	// create the internal iterator
	__Verify_noErr( FSOpenIterator( &gCurrDir, kFSIterateFlat, &mIterator ) );
}


/*
**	DTSFileIteratorPriv::~DTSFileIteratorPriv()
**	destructor -- close the underlying API iterator
*/
DTSFileIteratorPriv::~DTSFileIteratorPriv()
{
	if ( mIterator )
		__Verify_noErr( FSCloseIterator( mIterator ) );
}


/*
**	DTSFileIteratorPriv::Iterate()
**
**	advance the iteration by a single step
**	or return false when finished.
*/
bool
DTSFileIteratorPriv::Iterate( DTSFileSpecPriv& outSpec ) const
{
	// paranoia
	if ( not mIterator )
		return false;
	
	// we advance by doing a bulk Get, for a very tiny value of "bulk" (namely, 1)
	FSRef ref;
	ItemCount numGot;
//	Boolean changed;	// useless since 10.2
	OSStatus result = FSGetCatalogInfoBulk( mIterator,
							1,
							&numGot,
							nullptr,	// &changed,
							kFSCatInfoNone,
							nullptr,
							&ref,
							nullptr,
							nullptr );
	if ( noErr == result /* && not changed && numGot == 1 */ )
		return noErr == outSpec.CopyFromRef( &ref );
	
	return false;
}

#pragma mark -

// create the DTSImplement machinery
DTSDefineImplementFirmNoCopy( DTSFileIteratorPriv )


/*
**	DTSFileIterator constructor -- no-op
*/
DTSFileIterator::DTSFileIterator()
{
}


/*
**	DTSFileIterator destructor -- no-op
*/
DTSFileIterator::~DTSFileIterator()
{
}


/*
**	DTSFileIterator::Iterate()
**
**	advance the iteration by a single step
**	or return false when finished.
*/
bool
DTSFileIterator::Iterate( DTSFileSpec& outSpec ) const
{
	// we just forward the call onward, to our private-implement object.
	DTSFileIteratorPriv * p = priv.p;
	DTSFileSpecPriv * fsp = outSpec.priv.p;
	if ( not p || not fsp )
		return false;
	
	return p->Iterate( *fsp );
}

#pragma mark -


/*
**	DTSFileSpec::SetToPrefs
**
**	(re)initialize this filespec so it points to the user's preference folder
*/
DTSError
DTSFileSpec::SetToPrefs()
{
	DTSFileSpecPriv * p = priv.p;
	__Check( p );
	if ( not p )
		return -1;
	
	FSRef ref;
	OSStatus result = FSFindFolder( kUserDomain, kPreferencesFolderType, kCreateFolder, &ref );
	__Check_noErr( result );
	if ( noErr == result )
		{
		FSCatalogInfo info;
		result = FSGetCatalogInfo( &ref, kFSCatInfoNodeID | kFSCatInfoVolume, &info,
						nullptr, nullptr, nullptr );
		__Check_noErr( result );
		if ( noErr == result )
			{
			p->SetVolDir( info.volume, info.nodeID );
			p->SetFileName( "", true );
			}
		}
	
	return result;
}


/*
**	DTSFileSpec::PutDialog()
**
**	Solicit the user to specify the name and location of a file to be created.
**	Return true if they hit "OK" (or "Save" or whatever...), false if they canceled.
**	'prompt' is explanatory text to be shown in the put-file dialog.
**	'prefKey' is an arbitrary OSType (or UInt32) which can be used to distinguish
**	different "classes" of file destinations.
*/
bool
DTSFileSpec::PutDialog( const char * prompt, ulong prefKey /* =0 */ )
{
	// prompt can be NULL
	DTSFileSpecPriv * p = priv.p;
	__Check( p );
	if ( not p )
		return false;
	
	// get suggested parent directory and filename
	HFSUniStr255 name;
	FSRef parDir;
	DTSError err = p->CopyToRef( &parDir, &name );
	
	// run the dialog
	if ( noErr == err )
		err = PutNavDialog( &parDir, &name, prompt, prefKey );
	
	// update filespec's fields according
	if ( noErr == err )
		{
		p->CopyFromRef(	&parDir );
		p->SetFileName( &name, true );
		return true;
		}
	
	return false;
}


/*
**	DTSFileSpec::GetDialog()
**
**	Ask the user to identify and select an existing file.
**	'fileTypeID' is the FREF ID denoting which files are eligible, based on their fileType.
**	(-1 means "any and everything". Values that don't identify an existing FREF act as if
**	they specified files of type 'TEXT'.  To the extent that old-fashioned HFS metadata like
**	type & creator codes are increasingly unsupported by the OS, this argument has less & less
**	utility.)
**	'prompt' is explanatory text, to be shown in the file-picker dialog box.
**	'prefKey' is as in PutDialog() above.
*/
bool
DTSFileSpec::GetDialog( int fileTypeID, const char * prompt /* =nullptr */, ulong prefKey /* =0 */ )
{
	DTSFileSpecPriv * p = priv.p;
	__Check( p );
	if ( not p )
		return false;
	
	// prompt can be NULL
	
	bool result = false;
	
	// get the desired file type
	OSType fileType = 'TEXT';
	GetFileType( fileTypeID, &fileType );
	
	AEDesc answer = { typeNull, nullptr };
	
	// Run the dialog. If it succeeds, 'answer' should contain an FSRef for the chosen file
	OSStatus err = noErr;
	bool okay = GetNavDialog( fileType, &answer, prompt, prefKey );
	if ( not okay )
		err = userCanceledErr;
	
	if ( noErr == err )
		{
		FSRef ref;
		err = AEGetDescData( &answer, &ref, sizeof ref );
		__Check_noErr( err );
		if ( noErr == err )
			{
			/* err = */ p->CopyFromRef( &ref );
			result = true;
			}
		}
	
	__Verify_noErr( AEDisposeDesc( &answer ) );
	
	return result;
}


/*
**	MyGetVolRef()
**
**	return an FSRef to the named volume.
**	'name' is of the form "VolumeName:".
**	Used by ResolvePathsPlus() when it finds an absolute pathname.
*/
static DTSError
MyGetVolRef( const char * volname, FSRef * outRef )
{
	
#if 0 && defined( DEBUG_VERSION )
	// verify that the name is sensible
	if ( volname[0] == '\0'
	||   volname[ strlen(volname) ] != ':' )
		{
		return paramErr;
		}
#endif  // 0
	
	OSStatus result;
	
	// how to convert the old-style volume name into an FSRef?
#if 1		// via CFURL
	// this way seems least deprecated
	result = nsvErr;	// pessimism
	if ( CFStringRef cs = CreateCFString( volname ) )
		{
		if ( CFURLRef uu = CFURLCreateWithFileSystemPath( kCFAllocatorDefault,
							cs, kCFURLHFSPathStyle, true ) )
			{
			if ( CFURLGetFSRef( uu, outRef ) )
				result = noErr;
			
			CFRelease( uu );
			}
		
		CFRelease( cs );
		}
#elif 1	// via PBMakeFSRef
	Str255 buff;
	CopyCStringToPascal( volname, buff );
	
	FSRefParam pb;
	memset( &pb, 0, sizeof pb );
//	pb.ioCompletion = nullptr;
//	pb.ioVRefNum = 0;
//	pb.ioDirID = 0;
	pb.ioNamePtr = buff;
	
	pb.newRef = outRef;
	
	result = PBMakeFSRefSync( &pb );
#else	// via FSpMakeFSRef
	FSSpec fss;
	fss.vRefNum = 0;
	fss.parID = 0;
	CopyCStringToPascal( volname, fss.name );
	
	result = FSpMakeFSRef( &fss, outRef );
#endif  // 0
	
	return result;
}


// some preliminary noodling around with "bookmarks", the non-deprecated replacement for Aliases
#if MAC_OS_X_VERSION_MAX_ALLOWED >= 1060
# define USE_BOOKMARKS_FOR_ALIAS		1	// unset this if we crash on 10.5 machines...
#endif  // 10.6+ SDK only

#if USE_BOOKMARKS_FOR_ALIAS

// SDK 10.6 appears to annotate these APIs incorrectly
# if MAC_OS_X_VERSION_MAX_ALLOWED < 1070
extern "C" {
CFDataRef	CFURLCreateBookmarkDataFromFile( CFAllocatorRef allocator,
				CFURLRef fileURL, CFErrorRef * errorRef ) AVAILABLE_MAC_OS_X_VERSION_10_6_AND_LATER;
Boolean		CFURLWriteBookmarkDataToFile( CFDataRef bookmarkRef, CFURLRef fileURL,
				CFURLBookmarkFileCreationOptions options,
				CFErrorRef *errorRef ) AVAILABLE_MAC_OS_X_VERSION_10_6_AND_LATER;
};
# endif	// 10.6 SDK


/*
**	IsFolder()
**
**	tests whether a given URL points to a directory
*/
static bool __attribute__(( nonnull( 1 ) ))
IsFolder( CFURLRef u )
{
	bool folder = false;
	CFBooleanRef isFolder = nullptr;
//	CFErrorRef error = nullptr;
	
	// this seems more reliable than CFURLHasDirectoryPath()
	if ( CFURLCopyResourcePropertyForKey( u, kCFURLIsDirectoryKey,
			 &isFolder, nullptr /* &error */ ) )
		{
		folder = CFBooleanGetValue( isFolder );
		}
	
//	if ( error )
//		CFRelease( error );
	
	if ( isFolder )
		CFRelease( isFolder );
	
	return folder;
}
#endif  // USE_BOOKMARKS_FOR_ALIAS


/*
**	MyResolveAliasFile()
**
**	Given an arbitrary FSRef, attempt to resolve it (if it's a Finder Alias file).
**	Uses CFURL Bookmark APIs if available, to mimic the old Alias Manager's
**	FSResolveAliasFileWithMountFlags() API.  If bookmarks are not available,
**	this function becomes nothing more than a trivial wrapper for that call.
*/
static
#if ! USE_BOOKMARKS_FOR_ALIAS
inline
#endif
OSStatus __attribute__(( nonnull(1,2,3) ))
MyResolveAliasFile( FSRef * ref, Boolean * oIsFolder, Boolean * oWasAliased )
{
	OSStatus result;
	
#if USE_BOOKMARKS_FOR_ALIAS
	/*
	Notes on aliases:
	1. Aliases are deprecated
	2. Bookmarks, the replacement technology, isn't available until 10.6, but we still
	have users on 10.4 (!)
	3. Finder aliases may have a resource fork (containing an 'alis' and some icons)
	and/or a data fork (containing bookmark data). The bookmark portion first appears
	with 10.6. The resource part no longer exists as of 10.10.
	4. Moreover, if the 10.10 Finder so much as looks at a "resourceful" Finder alias
	file, it'll delete the resource fork.
	5. FSResolveAliasFile() fails on "resourceless" Finder aliases, but it does
	resolve Posix-level symlinks.
	6. CFURLCreateBookmarkDataFromFile() + CFURLCreateByResolvingBookmarkData() doesn't
	resolve symlinks.
	*/
	
	// must do a weak-link test since we can't guarantee OS 10.6+
	if ( nullptr != &CFURLCreateBookmarkDataFromFile )
		{
		result = coreFoundationUnknownErr;	// provisionally
		
		// convert the FSRef to a URL
		if ( CFURLRef url = CFURLCreateFromFSRef( kCFAllocatorDefault, ref ) )
			{
			CFBooleanRef isAlias = nullptr;
			CFErrorRef error = nullptr;
			
			// don't even bother if it's not a Finder alias file
			if ( CFURLCopyResourcePropertyForKey( url,
						kCFURLIsAliasFileKey, &isAlias, &error )
			&&   CFBooleanGetValue( isAlias ) )
				{
				// well, at least we know it was an alias! (or a symlink)
				*oWasAliased = true;
				
				// read in its bookmark data
				CFDataRef dr = CFURLCreateBookmarkDataFromFile( kCFAllocatorDefault,
									url, &error );
				if ( dr )
					{
//					*oWasAliased = true;
					Boolean isStale( false );
					
					// we're always going to want to fetch the is-directory property
					static CFArrayRef sWantedProps;
					if ( not sWantedProps )
						{
						const void * theProp = kCFURLIsDirectoryKey;
						sWantedProps = CFArrayCreate( kCFAllocatorDefault,
											&theProp, 1,
											&kCFTypeArrayCallBacks );
						__Check( sWantedProps );
						}
					
					// resolve it
					CFURLRef u2 = CFURLCreateByResolvingBookmarkData( kCFAllocatorDefault,
									dr,
									kCFBookmarkResolutionWithoutUIMask
									| kCFBookmarkResolutionWithoutMountingMask,
									nullptr,	// base
									sWantedProps,	// properties to get
									&isStale,
									&error );
					if ( u2 )
						{
						// update if necessary
						if ( isStale )
							{
							if ( CFDataRef dr2 = CFURLCreateBookmarkData( kCFAllocatorDefault,
													u2,
													kCFURLBookmarkCreationSuitableForBookmarkFile,
													nullptr, nullptr, nullptr ) )
								{
								CFURLWriteBookmarkDataToFile( dr2, url, nullptr, nullptr );
								CFRelease( dr2 );
								}
							}
						
						*oIsFolder = IsFolder( u2 );
						
						// fetch the target as an FSRef
						bool got = CFURLGetFSRef( u2, ref );
						if ( got )
							result = noErr;
						CFRelease( u2 );
						}
					CFRelease( dr );
					}
				else
					{
					// original ref did not contain valid bookmark data
					// (might have been a symlink)
					
					// maybe try to resolve it the old-fashioned way?
					// I don't know how to ask CFURLs to resolve symlinks.
					result = FSResolveAliasFileWithMountFlags( ref,
								true,
								oIsFolder,
								oWasAliased,
								kResolveAliasFileNoUI );
					}
				}
			else
				{
				// not an alias file at all
				*oWasAliased = false;
				result = noErr;
				}
			
			if ( isAlias )
				CFRelease( isAlias );
			if ( error )
				CFRelease( error );
			
			CFRelease( url );
			}
		}
	else
#endif  // USE_BOOKMARKS_FOR_ALIAS
		{
#if MAC_OS_X_VERSION_MIN_REQUIRED < 1060
		// we don't have the bookmark APIs, so fall back to the legacy version
		result = FSResolveAliasFileWithMountFlags( ref,
						true,	// resolve chains
						oIsFolder, oWasAliased,
						kResolveAliasFileNoUI );
#endif  // < 10.6
		}
	
	return result;
}


/*
**	ResolvePathsPlus()		[ HFSPlus edition ]
**
**	resolve any and all path info in the file name.
**	account for relative paths, absolute paths, and aliases.
*/
static DTSError
ResolvePathsPlus( DTSFileSpecPriv * p, bool bResolveLastAlias )
{
	__Check( p );
	if ( not p )
		return paramErr;
	
	DTSError result = noErr;
	
	FSRef ref;
	FSCatalogInfo info;
	
	// use the current volume and directory if necessary
	if ( 0 == p->GetVolume() )
		{
		ref = gCurrDir;
		
		// handle a trivial case
		if ( 0 == p->GetFileName()[0] )
			{
			result = FSGetCatalogInfo( &ref, kFSCatInfoVolume | kFSCatInfoNodeID,
						&info, nullptr, nullptr, nullptr );
			__Check_noErr( result );
			if ( noErr != result )
				return result;
			
			p->SetVolDir( info.volume, info.nodeID );
			p->SetResolved( true );
			return noErr;
			}
		}
	else
		{
		// p->vol is non-zero, so we must set ref to match it
		// but we can take a shortcut in the trivial case
		if ( 0 == p->GetFileName()[0] )
			{
			p->SetResolved( true );
			return noErr;
			}
		
		// set 'ref' from p's vol/dir
		result = FSResolveNodeID( p->GetVolume(), p->GetDirectory(), &ref );
		__Check_noErr( result );
		if ( noErr != result )
			return result;
		}
	
	// starting info
	char buff[ 256 ];
	size_t len;
	int loc = kNameInSpec;
	const char * path = p->GetFileName();
	FSRef newRef;
	
	// check for colons
	// check for an absolute path
	// ie "DiskName:FolderName:Etc:FileName"
	const char * next = strchr( path, ':' );
	if ( next
	&&   next != path )
		{
		len = next - path + 1;
		memcpy( buff, path, len );
		buff[ len ] = '\0';
		
		// set ref to the top of volume "DiskName"
		result = MyGetVolRef( buff, &ref );
		if ( noErr != result )
			return result;
		path = next;	// do not consume the ':'
		loc  = kNameInPath;
		}
	
	// resolve all steps
	HFSUniStr255 uniName;
	for(;;)
		{
		// stop when we're done
		if ( 0 == path[0] )
			break;
		
		// handle ":" prefixes special
		if ( ':' == path[0] )
			{
			// handle single colon
			if ( path[1] != ':' )
				{
				++path;
				loc = kNameInPath;
				continue;
				}
			
			// handle double colon
			// go up a directory
			result = FSGetCatalogInfo( &ref, kFSCatInfoNone,
							nullptr,	// info
							nullptr,	// Unicode name
							nullptr,	// FSSpec
							&newRef );	// parent FSRef
			if ( noErr != result )
				break;
			ref = newRef;
			++path;
			loc = kNameInPath;
			continue;
			}
		
		// determine the length of the first step
		next = strchr( path, ':' );
		if ( not next )
			{
			len  = strlen( path );
			next = path + len;
			}
		else
			len = next - path;		// do not include the trailing :
		
		// build the file or directory name
		memcpy( buff, path, len );
		buff[ len ] = '\0';
		StringToHFSUniStr255( buff, &uniName );
		
		result = FSMakeFSRefUnicode( &ref,
						uniName.length, uniName.unicode, kTextEncodingUnknown, &newRef );
		
		// stop if the file does not exist
		// this is not an error condition
		if ( fnfErr == result )
			{
			result = noErr;
			loc = kNameInPath;
			break;
			}
		
		ref = newRef;
		// stop if there is any other error
		if ( noErr != result )
			break;
		
		// Is this a file or a directory?
		result = FSGetCatalogInfo( &ref, kFSCatInfoNodeFlags,
						&info,
						nullptr,			// no name
						nullptr,			// no FSSpec
						nullptr );			// no parent FSRef
		
		// stop if there is any other error
		if ( noErr != result )
			break;
		
		// test if this is a directory
		if ( info.nodeFlags & kFSNodeIsDirectoryMask )
			{
			// remember the directory
			// point at the next step
			loc  = kNameInPath;
			path = next;
			continue;
			}
		
		// do we skip the last alias?
		if ( not bResolveLastAlias )
			{
			// no more colons in the path
			// we're done
			if ( not next || 0 == next[0] )
				break;
			}
		
		// now we have to deal with aliases
		if ( true )
			{
			Boolean targetIsFolder( false ), wasAliased( false );
			result = MyResolveAliasFile( &ref, &targetIsFolder, &wasAliased );
			if ( noErr != result )
				break;
			
			// it was an ordinary file
			if ( not wasAliased )
				break;
			
			// it was an alias to a folder
			// continue parsing
			if ( targetIsFolder )
				{
				loc = kNameInPath;
				path = next;
				continue;
				}
			
			// it was an alias to a file
			// we're done
			// the real name of the resolved file is now (implicitly) in the FSRef
			loc = kNameInFSS;
			break;
			}
		}
	
	// check for lingering colons
	// this would be an error
	if ( noErr == result )
		{
		next = strchr( path, ':' );
		if ( next )
			result = -1;
		}
	
	// update the information
	// (this is why 'p' can't be a <const DTSFileSpecPriv *>, which would
	// otherwise be a great improvement, and allow many more DTSFileSpec methods
	// to be marked const.)
	
	if ( noErr == result )
		{
		// we require (at the very least) the vRefNum and the parent directory ID...
		FSCatalogInfoBitmap whichFlags = kFSCatInfoVolume
										| kFSCatInfoParentDirID;
		
		// but if the name is "in the path", AND the item is a folder, then the
		// real directoryID is the info's nodeID, not its parentDirID. Furthermore, in that case,
		// before we can tell whether the item is a folder, we must fetch the nodeFlags.
		
		if ( kNameInPath == loc )
			whichFlags |= kFSCatInfoNodeFlags | kFSCatInfoNodeID;
		
		// last complication: if the name is "in the FSS" (i.e., implicitly embedded
		// within the FSRef itself), then we need to extract it too.
		
		HFSUniStr255 * destName = nullptr;
		if ( kNameInFSS == loc )
			destName = &uniName;
		
		// OK, now we're good to get the info
		result = FSGetCatalogInfo( &ref,
					whichFlags,
					&info,
					destName,	// might need name
					nullptr,	// don't need FSSpec
					nullptr		// don't need parent FSRef either
					);
		if ( noErr != result )
			return result;
		
		// assume the FSRef really does refer to the desired item,
		// so remember its parent dirID for later
		UInt32 dirID = info.parentDirID;
		
		switch ( loc )
			{
			case kNameInSpec:
				break;
			
			case kNameInPath:
				// whups, the FSRef refers to the target's parent, not the target itself.
				// use the FSRef's actual nodeID as the spec's parent
				if ( info.nodeFlags & kFSNodeIsDirectoryMask )
					dirID = info.nodeID;
				p->SetFileName( path );			// don't care about the is-resolved flag now;
				break;							// we'll set it for good right after the switch
			
			case kNameInFSS:
				p->SetFileName( &uniName );		// ditto
				break;
			}
		
		p->SetVolDir( info.volume, dirID );
		p->SetResolved( true );
		}
	
	return result;
}


/*
**	ResolvePaths()
**
**	resolve aliases & pathnames.
**	if needed, delegate the tough work to the helper routine
*/
DTSError
ResolvePaths( DTSFileSpecPriv * p, bool bResolveLastAlias /* =true */ )
{
	if ( p->IsResolved() )
		return noErr;
	
	return ResolvePathsPlus( p, bResolveLastAlias );
}


/*
**	DTSFileSpec_CopyToFSRef()
**
**	This makes *'outRef' point to the file-system object indicated by 'inSpec'.
**	The target object MUST exist in the filesystem, since FSRefs cannot refer to
**	non-existent entities.
*/
DTSError
DTSFileSpec_CopyToFSRef( DTSFileSpec * inSpec, FSRef * outRef, bool bResolveLast /* = true */ )
{
	__Check( inSpec );
	__Check( outRef );
	if ( not inSpec || not outRef )
		return paramErr;
	
	DTSFileSpecPriv * p = inSpec->priv.p;
	__Check( p );
	if ( not p )
		return -1;
	
	return p->CopyToRef( outRef, bResolveLast );
}


/*
**	DTSFileSpec_CopyToFSRef()
**
**	This one is similar to the above, but it copies the spec's parent directory
**	into the FSRef, and the spec's filename into the HFSUniStr255.
**	Hence, the target spec object need not exist yet, although the parent directory must.
*/
DTSError
DTSFileSpec_CopyToFSRef(
	DTSFileSpec *	spec,
	FSRef *			parDirRef,
	HFSUniStr255 *	leafName,
	bool			bResolveLast /*= true*/ )
{
	__Check( spec );
	__Check( parDirRef );
	__Check( leafName );
	if ( not spec || not parDirRef || not leafName )
		return paramErr;
	
	DTSFileSpecPriv * p = spec->priv.p;
	__Check( p );
	if ( not p )
		return -1;
	
	return p->CopyToRef( parDirRef, leafName, bResolveLast );
}


/*
**	DTSFileSpec_CopyFromFSRef()
**
**	Sets the filespec to point to the same entity as the FSRef
**	(and which, therefore, must already exist)
*/
DTSError
DTSFileSpec_CopyFromFSRef( DTSFileSpec * outSpec, const FSRef * inRef )
{
	__Check( outSpec );
	__Check( inRef );
	if ( not outSpec || not inRef )
		return paramErr;
	
	DTSFileSpecPriv * p = outSpec->priv.p;
	__Check( p );
	if ( not p )
		return -1;
	
	return p->CopyFromRef( inRef );
}


/*
**	DTSFileSpec_Set()
**
**	Like DTSFileSpec_CopyFromFSRef() above, but takes two arguments:
**	an FSRef for the parent directory, and a CFString for the leafname.
**	Convenient for use with, e.g., NavServices.
*/
DTSError
DTSFileSpec_Set( DTSFileSpec * outSpec, const FSRef * parDir, CFStringRef leafName )
{
	__Check( outSpec );
	__Check( parDir );
	__Check( leafName );
	if ( not outSpec || not parDir || not leafName )
		return paramErr;
	
	DTSFileSpecPriv * p = outSpec->priv.p;
	__Check( p );
	if ( not p )
		return -1;
	
	DTSError result = p->CopyFromRef( parDir );
	HFSUniStr255 uName;
	if ( noErr == result )
		{
		result = FSGetHFSUniStrFromString( leafName, &uName );
		__Check_noErr( result );
		}
	
	if ( noErr == result )
		p->SetFileName( &uName, true );
	
	return result;
}


/*
**	DTSFileSpecPriv::CopyToRef()
**
**	Copy a DTSFileSpecPriv to an FSRef (of its parent directory) and an HFSUniStr255 (for the name)
**	Either of 'outRef' and 'name' can be NULL, if you only want a partial conversion.
**	But I can't imagine any scenario where having -both- NULL would be useful.
*/
DTSError
DTSFileSpecPriv::CopyToRef( FSRef * outRef, HFSUniStr255 * name, bool resolve /* =true */ )
{
//	__Check( outRef );
//	__Check( name );
	if ( not outRef && not name )
		return paramErr;
	
	// cope with aliases and funky names
	DTSError result = ResolvePaths( this, resolve );
	
	// create parent FSRef, from input vRefNum and parent DirID. Ignore the name.
	if ( noErr == result && outRef )
		{
		result = FSResolveNodeID( fileVolume, fileDirectory, outRef );
		__Check_noErr( result );
		}
	
	// translate the (MacRoman?) name, in the DTSFileSpecPriv, into a UniCode name
	if ( name )
		{
		if ( noErr == result )
			StringToHFSUniStr255( fileName, name );
		else
			{
			// uh oh...
			name->length = 0;
			name->unicode[0] = 0;
			}
		}
	
	return result;
}


/*
**	DTSFileSpecPriv::CopyToRef()
**
**	Copy a DTSFileSpecPriv to an FSRef, only.
**	Differs from immediately preceding variant in that, here, the filename is NOT separated out:
**	the output FSRef will be a direct reference to the DTSSpec's target.
**	This implies that the target file/folder MUST exist!
**	(The other version requires only that the _parent directory_ must exist, and can thus
**	safely be used for files that haven't been created yet.)
*/
DTSError
DTSFileSpecPriv::CopyToRef( FSRef * outRef, bool resolveLastAlias /* =true */ )
{
	__Check( outRef );
	if ( not outRef )
		return paramErr;
	
	// cope with aliases and funky names
	DTSError result = ResolvePaths( this, resolveLastAlias );
	
	// first, make an FSRef to just the parent directory
	FSRef parent;
	if ( noErr == result )
		{
		result = FSResolveNodeID( fileVolume, fileDirectory, &parent );
		__Check_noErr( result );
		}
	
	// then make our final FSRef, using the parent ref and the Unicodified name
	if ( noErr == result )
		{
		HFSUniStr255 name;
		StringToHFSUniStr255( fileName, &name );
		
		result = FSMakeFSRefUnicode( &parent,
						name.length, name.unicode, kTextEncodingUnknown, outRef );
		if ( fnfErr != result )
			{
			__Check_noErr( result );
			}
		}
	
	return result;
}


/*
**	DTSFileSpecPriv::CopyFromRef()
**	
**	Set the spec to point to the object denoted by 'inRef'
*/
DTSError
DTSFileSpecPriv::CopyFromRef( const FSRef * inRef )
{
	__Check( inRef );
	if ( not inRef )
		return paramErr;
	
	// fetch the requisite data
	// NB: if we knew ahead of time whether this object is a file or a folder,
	// we could optimize this next call to only retrieve the minimal amount of info needed:
	// viz., either the (file's) parentDirID and name, or the (folder's) nodeID
	enum {
		kFlags = kFSCatInfoParentDirID	| kFSCatInfoVolume |
				 kFSCatInfoNodeFlags	| kFSCatInfoNodeID
	};
	
	FSCatalogInfo info;
	HFSUniStr255 name;
	DTSError result = FSGetCatalogInfo( inRef, kFlags,
							&info,
							&name,
							nullptr,	// don't want an FSSpec
							nullptr		// nor parent FSRef
							);
	__Check_noErr( result );
	
	// copy those into the DTSFileSpecPriv object
	if ( noErr == result )
		{
		fileVolume = info.volume;
		
		// is it a folder or a file?
		if ( info.nodeFlags & kFSNodeIsDirectoryMask )
			{
			// for folders, we store the directory's own dirID, but no name
			fileDirectory = info.nodeID;
			fileName[ 0 ] = 0;
			}
		else
			{
			// for files, we store the parent dirID, and the leaf name
			fileDirectory = info.parentDirID;
			SetFileName( &name );
			}
		}
	
	// No; it's already "resolved enough", and anyway it'll get re-resolved
	// as soon as someone tries to use this fileSpecPriv.
#if 0
	if ( noErr == result )
		result = ResolvePaths( this );
#endif
	
	return result;
}


/*
**	DTSFileSpecPriv::CreateURL()
**
**	create a new CFURLRef which points to the same entity as this spec;
**	or NULL on failure
*/
CFURLRef
DTSFileSpecPriv::CreateURL()
{
	CFURLRef u( nullptr );
	
	FSRef tmp;
	HFSUniStr255 name;
	name.length = 0;
	
	// this version of CopyToRef() fetches the parent-directory FSRef and leafname separately;
	// that way we can create URLs that refer to as-yet-nonexistent files, as long as they're
	// in pre-existing folders. For the full URL generality, where the intermediate pathnames
	// might not [yet] exist, you'd probably have to construct from scratch, e.g. via
	// CFURLCreateWithFileSystemPath(), CFURLCreateCopyAppendingPathComponent(), etc., etc.
	
	if ( noErr == CopyToRef( &tmp, &name, true ) )
		{
		// Make URL to parent dir
		u = CFURLCreateFromFSRef( kCFAllocatorDefault, &tmp );
		if ( u )
			{
			// do we have a leafname also?
			if ( name.length )
				{
				// append it to the URL
				if ( CFStringRef leaf = FSCreateStringFromHFSUniStr( kCFAllocatorDefault, &name ) )
					{
					if ( CFURLRef u2 = CFURLCreateCopyAppendingPathComponent( kCFAllocatorDefault,
											u, leaf, false ) )
						{
						// switch u2 and u
//						CFRetain( u2 );
						CFRelease( u );
						u = u2;
						}
					CFRelease( leaf );
					}
				}
			}
		}
	return u;
}


/*
**	DTSFileSpecPriv::SetFromURL()
**
**	initialize the spec from a given URLRef (which had better be a local "file:///" one...)
*/
DTSError
DTSFileSpecPriv::SetFromURL( CFURLRef u )
{
	DTSError err = fnfErr;	// provisionally
	
	FSRef tmp;
	if ( CFURLGetFSRef( u, &tmp ) )
		return CopyFromRef( &tmp );
	
	// OK that didn't work. Maybe the URL points to a valid directory,
	// but with a leafname that doesn't yet exist?
	if ( not CFURLHasDirectoryPath( u ) )
		{
		// make FSRef to parent directory
		if ( CFURLRef parent = CFURLCreateCopyDeletingLastPathComponent( CFGetAllocator( u ), u ) )
			{
			if ( CFURLGetFSRef( parent, &tmp ) )
				{
				// set filespec to the parent directory
				if ( noErr == CopyFromRef( &tmp ) )
					{
					// now tack on the leaf-name
					if ( CFStringRef leaf = CFURLCopyLastPathComponent( u ) )
						{
						HFSUniStr255 l2;
						if ( noErr == FSGetHFSUniStrFromString( leaf, &l2 ) )
							{
							SetFileName( &l2 );
							err = noErr;
							}
						CFRelease( leaf );
						}
					}
				}
			
			CFRelease( parent );
			}
		}
	
	return err;
}


/*
**	DTSFileSpec_CreateURL()
**
**	Returns a CFURLRef that names the same entity as the fileSpec, or NULL on failure
*/
CFURLRef
DTSFileSpec_CreateURL( const DTSFileSpec * spec )
{
	if ( DTSFileSpecPriv * p = spec->priv.p )
		return p->CreateURL();
	
	return nullptr;
}


/*
**	DTSFileSpec_SetFromURL()
**
**	Sets the filespec to point to the same entity as the URL, which should be a local file
**	(or at the very least, should name an as-yet-nonexistent file, within an existing directory)
*/
DTSError
DTSFileSpec_SetFromURL( DTSFileSpec * oSpec, CFURLRef inURL )
{
	if ( DTSFileSpecPriv * p = oSpec->priv.p )
		return p->SetFromURL( inURL );
	
	return -1;
}


/*
**	StringToHFSUniStr255()
**
**	translate a MacRoman-encoded C-style string to an HFSUniStr255, suitable for the File Manager.
*/
void
StringToHFSUniStr255(
	const char *	text,
	HFSUniStr255 *	result )
{
	CFStringRef cs = CreateCFString( text );
	__Check( cs );
	if ( cs )
		{
		if ( noErr == FSGetHFSUniStrFromString( cs, result ) )
			{
			CFRelease( cs );
			return;
			}
		
		// would be pretty unlikely to get here
		UniCharCount len = CFStringGetLength( cs );
		result->length = static_cast<UInt16>( len );
		CFStringGetCharacters( cs, CFRangeMake( 0, len ), result->unicode );
		CFRelease( cs );
		}
	else
		{
		// how can this even happen?
		result->length = 0;
		result->unicode[0] = 0;
		}
}


/*
**	HFSUniStr255ToString()
**
**	Translate an HFSUniStr255, from the File Manager, into a plain C string:
**	in short, the inverse of StringToHFSUniStr255(), above. 
**	Once again, we make the dubious decision to use the MacRoman text encoding.
*/
void
HFSUniStr255ToString(
		const HFSUniStr255 *	text,
		char *					outString,
		size_t					outLen )
{
	bool got = false;
	CFStringRef cs = FSCreateStringFromHFSUniStr( kCFAllocatorDefault, text );
	__Check( cs );
	if ( cs )
		{
		// this might fail, since the name could contain non-MacRoman characters
		got = CFStringGetCString( cs, outString, outLen, kCFStringEncodingMacRoman );
		
#if 0	// should we retry it as UTF8? Nothing else in dtslib understands that...
		if ( not got )
			{
			// try again as UTF8
			got = CFStringGetCString( cs, outString, outLen, kCFStringEncodingUTF8 );
			}
#endif  // UTF8 retry
		
		CFRelease( cs );
		}
	
	// it might _still_ fail, e.g. if the name is too long...
	if ( not got )
		outString[0] = '\0';
}


/*
**	DTSFileSpec::Delete()
**
**	delete the file
*/
DTSError
DTSFileSpec::Delete()
{
	DTSFileSpecPriv * p = priv.p;
	__Check( p );
	if ( not p )
		return -1;
	
	FSRef ref;
	DTSError result = p->CopyToRef( &ref, true );
	if ( noErr == result )
		{
		result = FSDeleteObject( &ref );
		__Check_noErr( result );
		}
	
	return result;
}


/*
**	DTSFileSpec::GetModifiedDate()
**
**	return the date the file was modified.
**	As an optimization, if you just want to use the date.dateInSeconds value only, and
**	don't care about the rest of the DTSDate's fields, then pass true for 'bJustWantSeconds'.
*/
DTSError
DTSFileSpec::GetModifiedDate( DTSDate * date, bool bJustWantSeconds /* = false */ )
{
	__Check( date );
	if ( not date )
		return paramErr;
	
	DTSFileSpecPriv * p = priv.p;
	__Check( p );
	if ( not p )
		return -1;
	
	FSRef ref;
	DTSError result = p->CopyToRef( &ref, true );
	if ( noErr == result )
		{
		FSCatalogInfo info;
		result = FSGetCatalogInfo( &ref,
						kFSCatInfoContentMod,
						&info,
						nullptr, nullptr, nullptr );
		__Check_noErr( result );
		if ( noErr == result )
			date->dateInSeconds = info.contentModDate.lowSeconds;
		}
	
	if ( noErr == result && not bJustWantSeconds )
		date->SetFromSeconds();
	
	return result;
}


/*
**	fopen( CFURLRef )
**
**	like std::fopen(), but takes a file URL rather than a POSIX path
*/
FILE *
fopen( CFURLRef url, const char * mode )
{
	FILE * stream = nullptr;

	// paranoia
	if ( url )
		{
		// no reading directories, please!
		bool bIsDir = CFURLHasDirectoryPath( url );
		__Check( not bIsDir );
		if ( not bIsDir )
			{
			char path[ MAXPATHLEN + 1 ];
			bool got = CFURLGetFileSystemRepresentation( url, true, (uchar *) path, sizeof path );
			if ( got )
				stream = std::fopen( path, mode );
			}
		}
	
	return stream;
}


/*
**	fopen()
**
**	just like the previous variant, but takes arguments that are conveniently available
**	as a result of, e.g., a NavPutFile() call (notably the NavReply's saveFileName field,
**	which is a CFString not an HFSUniStr255).
**	If 'fname' is NULL or empty, then it's assumed that 'parDir' actually denotes an existing
**	_file_, not just a directory.
*/
FILE *
fopen( const FSRef * parDir, CFStringRef fname, const char * mode )
{
	FILE * stream = nullptr;
	CFURLRef u = CFURLCreateFromFSRef( kCFAllocatorDefault, parDir );
	if ( u )
		{
		CFURLRef u2;
		if ( fname && CFStringGetLength( fname ) > 0 )
			u2 = CFURLCreateCopyAppendingPathComponent( kCFAllocatorDefault, u, fname, false );
		else
			CFRetain( u2 = u );	// just to balance the two releases
		
		if ( u2 )
			{
			stream = fopen( u2, mode );		// calls the above fopen() variant
			CFRelease( u2 );
			}
		CFRelease( u );
		}
	
	return stream;
}


/*
**	DTSFileSpec::fopen()
**
**	open the file using fopen(3)
*/
FILE *
DTSFileSpec::fopen( const char * perms )
{
	__Check( perms );
	if ( not perms )
		return nullptr;
	
	DTSFileSpecPriv * p = priv.p;
	if ( not p )
		return nullptr;
	
	FILE * stream = nullptr;
	
	// for non-"r" perms, the target file might not even exist yet, so we must use
	// the 3-argument version of CopyToRef()
	FSRef parentRef;
	HFSUniStr255 leafName;
	DTSError result = p->CopyToRef( &parentRef, &leafName, true );
	if ( noErr == result )
		{
		// interchange HFS and Posix path separators
		UniChar * pp = leafName.unicode;
		for ( uint nch = leafName.length; nch > 0; --nch )
			{
			if ( '/' == *pp )
				*pp = ':';
			else
			if ( ':' == *pp )
				*pp = '/';
			++pp;
			}
		
		// get leafname as CFString
		if ( CFStringRef cs = FSCreateStringFromHFSUniStr( kCFAllocatorDefault, &leafName ) )
			{
			stream = ::fopen( &parentRef, cs, perms );	// calls the above variant
			CFRelease( cs );
			}
		}
	
	return stream;
}


/*
**	GetFileType()
**
**	convert an FREF resource-ID to an OS file type
*/
DTSError
GetFileType( int id, OSType * filetype )
{
	__Check( filetype );
	if ( not filetype )
		return paramErr;
	
	// a fileTypeID of -1 shall mean "any type of file at all"
	if ( -1 == id )
		{
		*filetype = typeWildCard;
		return noErr;
		}
	
	DTSError result = resNotFound;
	if ( Handle h = GetResource( 'FREF', static_cast<ResID>( id ) ) )
		{
		if ( GetHandleSize( h ) >= static_cast<int>( sizeof *filetype ) )
			{
			*filetype = * reinterpret_cast<OSType *>( *h );
			result = noErr;
			}
		
		// leave the FREF loaded -- may need it again later, and it's < 10 bytes long
//		ReleaseResource( h );
		}
	
	return result;
}


/*
**	DTSFileSpec::CreateDir()
**
**	create a new folder.
**	this does NOT modify the DTSFileSpec itself, except via ResolvePaths().
**	If you want the spec to truly point to the new directory, you must manually
**	call DTSFileSpec::SetDir() on it, after this routine returns successfully.
*/
DTSError
DTSFileSpec::CreateDir()
{
	DTSFileSpecPriv * p = priv.p;
	if ( not p )
		return -1;
	
	FSRef ref;
	HFSUniStr255 name;
	DTSError result = p->CopyToRef( &ref, &name, true );
	if ( noErr == result )
		{
		// already exists?
		if ( 0 == name.length )
			return noErr;
		
		result = FSCreateDirectoryUnicode( &ref,
					name.length,
					name.unicode,
					kFSCatInfoNone,
					nullptr,	// no cat info
					nullptr,	// no new ref
					nullptr,	// no new spec
					nullptr		// ignore new dirID
					);
		
		// not an error if that folder already exists
		if ( dupFNErr == result )
			result = noErr;
		
		__Check_noErr( result );
		}
	
	// the spec is no longer resolved
	if ( noErr == result )
		p->SetResolved( false );
	
	return result;
}


/*
**	DTSFileSpec::SetTypeCreator()
**
**	set the type and creator of the file.
**	less & less useful, if not downright unhelpful, post-10.4 
*/
DTSError
DTSFileSpec::SetTypeCreator( int typeFREFID, int creatorFREFID )
{
	DTSFileSpecPriv * p = priv.p;
	if ( not p )
		return -1;
	
	OSType fType    = kUnknownType;
	OSType fCreator = gDTSApp->GetSignature();
	if ( typeFREFID )
		GetFileType( typeFREFID, &fType );
	if ( creatorFREFID )
		GetFileType( creatorFREFID, &fCreator );
	
	FSRef ref;
	FSCatalogInfo catinfo;
	FileInfo& info = * reinterpret_cast<FileInfo *>( &catinfo.finderInfo );
	
	DTSError result = p->CopyToRef( &ref, true );
	if ( noErr == result )
		{
		// get pre-existing FileInfo
		result = FSGetCatalogInfo( &ref, kFSCatInfoFinderInfo, &catinfo, nullptr, nullptr, nullptr );
		__Check_noErr( result );
		}
	if ( noErr == result )
		{
		// then tweak the type & creator
		info.fileType    = fType;
		info.fileCreator = fCreator;
		result = FSSetCatalogInfo( &ref, kFSCatInfoFinderInfo, &catinfo );
		__Check_noErr( result );
		}
	
	return result;
}


/*
**	DTSFileSpec::GetType()
**
**	return an FREF id for the type of this file,
**	including kFileTypeDirectory if it's a folder.
**	or -1 if there's no FREF resource for the type.
*/
DTSError
DTSFileSpec::GetType( int * typeFREFID )
{
	__Check( typeFREFID );
	if ( not typeFREFID )
		return paramErr;
	
	DTSFileSpecPriv * p = priv.p;
	__Check( p );
	if ( not p )
		return -1;
	
	// determine actual OSType (or kFileTypeDirectory, if such is the case)
	OSType fType = kUnknownType;
	FSRef ref;
	DTSError result = p->CopyToRef( &ref, true );
	if ( noErr == result )
		{
		FSCatalogInfo catinfo;
		FileInfo& info = * reinterpret_cast<FileInfo *>( &catinfo.finderInfo );
		
		result = FSGetCatalogInfo( &ref, kFSCatInfoNodeFlags | kFSCatInfoFinderInfo,
						&catinfo, nullptr, nullptr, nullptr );
		__Check_noErr( result );
		if ( noErr == result )
			{
			if ( catinfo.nodeFlags & kFSNodeIsDirectoryMask )
				fType = kFileTypeDirectory;
			else
				fType = info.fileType;
			}
		}
	
	// now translate the real type into an FREF index
	// (this breaks if there are multiple open resource files, such that the desired FREF
	// is inadvertently "shadowed" by another.)
	// [? FIX/TODO: possibly use UTIs instead?]
	ResID id;
	if ( noErr == result )
		{
		SInt16 count = CountResources( 'FREF' );
		result = ResError();
		__Check_noErr( result );
		if ( noErr == result )
			{
			result = -1;
			while ( count > 0 )
				{
				if ( Handle hdl = GetIndResource( 'FREF', count ) )		// this is 1-based
					{
					if ( * reinterpret_cast<OSType *>( *hdl ) == fType )
						{
						ResType rtype;	// ignored
						Str255 name;	// ignored
						GetResInfo( hdl, &id, &rtype, name );
						result = ResError();
						__Check_noErr( result );
						break;
						}
					}
				--count;
				}
			}
		}
	
	if ( noErr == result )
		*typeFREFID = id;
	
	return result;
}


/*
**	DTSFileSpec::Rename()
**
**	rename the file
*/
DTSError
DTSFileSpec::Rename( const char * newFileName )
{
	__Check( newFileName );
	if ( not newFileName )
		return paramErr;
	DTSFileSpecPriv * p = priv.p;
	if ( not p )
		return -1;
	
	FSRef ref;
	
	// don't resolve that last alias
	// assume we want to rename aliases
	DTSError result = p->CopyToRef( &ref, false );
	if ( noErr == result )
		{
		HFSUniStr255 buff;
		
		// assume the new name is in MacRoman encoding
		StringToHFSUniStr255( newFileName, &buff );
		FSRef movedRef;
		result = FSRenameUnicode( &ref,
						buff.length,
						buff.unicode,
						kTextEncodingUnknown,	// maybe should match the conversion encoding?
						&movedRef				// updated ref
						);
		__Check_noErr( result );
		if ( noErr == result )
			result = p->CopyFromRef( &movedRef );
		}
	
	return result;
}


/*
**	DTSFileSpec::Move()
**
**	move the file to another directory on this same volume
*/
DTSError
DTSFileSpec::Move( DTSFileSpec * newDir )
{
	__Check( newDir );
	if ( not newDir )
		return paramErr;
	
	DTSFileSpecPriv * p = priv.p;
	if ( not p )
		return -1;
	
	DTSFileSpecPriv * newDirMac = newDir->priv.p;
	if ( not newDirMac )
		return -1;
	
	// get FSRef for source object, and another for destination's directory
	FSRef srcref, dstref;
	DTSError result = p->CopyToRef( &srcref, true );
	if ( noErr == result )
		{
		// get dest's parent dir; ignore the leaf name
		result = newDirMac->CopyToRef( &dstref, nullptr, true );
		}
	
	// do the move
	if ( noErr == result )
		{
		result = FSMoveObject( &srcref, &dstref, nullptr );
		__Check_noErr( result );
		}
	
	// balance the books
	if ( noErr == result )
		result = p->CopyFromRef( &srcref );
	
	return result;
}


#pragma mark -- Navigation Services --

/*
**	MyNavEventProc()
**
**	handle callbacks from Navigation Services dialogs
*/
static void 
MyNavEventProc( NavEventCallbackMessage		callBackSelector,
				NavCBRecPtr					callBackParms,
				NavCallBackUserData			/* callBackUD */ )
{
	try
		{
		// we only have to handle update events
		// but let's be ambitious
		if ( kNavCBEvent == callBackSelector )
			{
			// HandleEvent() is pretty helpless when it comes to activate events
			// let's give it a hand
			if ( activateEvt == callBackParms->eventData.eventDataParms.event->what )
				{
				gEvent = *callBackParms->eventData.eventDataParms.event;
				WindowRef actWin = reinterpret_cast<WindowRef>( gEvent.message );
				if ( kDialogWindowKind == GetWindowKind( actWin ) )
					{
					// force a redraw of the underlying modal dialog (if any)
					Rect b;
					GetWindowPortBounds( actWin, &b );
					InvalWindowRect( actWin, &b );
					}
				ActivateWindows();
				}
			else
			if ( updateEvt == callBackParms->eventData.eventDataParms.event->what )
				{
				gEvent = *callBackParms->eventData.eventDataParms.event;
				HandleEvent();
				}
			}
		}
	catch ( ... ) {}
}


/*
**	TypeListForType()
**
**	allocate & return a NavTypeListHandle with one entry
**	or NULL if we're looking for files of all types
*/
static NavTypeListHandle
TypeListForType( OSType fileType )
{
	NavTypeListHandle typeList = nullptr;
	if ( fileType != '****' )
		{
		typeList = reinterpret_cast<NavTypeListHandle>( NewHandleClear( sizeof(NavTypeList) ) );
		
		// populate the list
		if ( typeList )
			{
			NavTypeList * ntlp		 = *typeList;
			ntlp->componentSignature = gDTSApp->GetSignature();
			ntlp->osTypeCount		 = 1;
			ntlp->osType[ 0 ]		 = fileType;
			}
		}
	
	return typeList;
}


/*
**	GetNavDialog()
**
**	Choose a file to open using Navigation Services.
**	On input, 'reply' points to a null AEDesc; on output, it's
**	the result from the NavSvcs reply record, as a typeFSRef.
*/
bool
GetNavDialog( OSType fileType, AEDesc * reply, const char * prompt, UInt32 prefKey )
{
	bool bIsValid = false;
	
	// get the starting dialog options, so we can modify them
	NavDialogCreationOptions options;
	OSStatus err = NavGetDefaultDialogCreationOptions( &options );
	__Check_noErr( err );
	
	// bail now if we couldn't initialize the options
	if ( noErr != err )
		{
		// maybe Beep() here? or SetErrorCode( err ) ??
		return false;
		}
	
	// turn off option(s) we don't want
	options.optionFlags &= ~ kNavAllowMultipleFiles;
	
	// turn on option(s) we do want
//	options.optionFlags |= kNavNoTypePopup;
	
	// tell it our application name
	if ( CFTypeRef appNm = CFBundleGetValueForInfoDictionaryKey( CFBundleGetMainBundle(),
								kCFBundleNameKey ) )
		options.clientName = static_cast<CFStringRef>( appNm );
	
	// and the prompt string, if there is one
	if ( prompt )
		options.message = CreateCFString( prompt );
	
	// and the prefkey
	options.preferenceKey = prefKey;
	
	// set up the static callback UPP
	 if ( not sNavEventProc )
	 	{
		sNavEventProc = NewNavEventUPP( MyNavEventProc );
		__Check( sNavEventProc );
		}
	
	// prepare a type list (or NULL for '****')
	// if they're asking for TEXT files, we can use a UTI instead of a type-list
	NavTypeListHandle typeList = nullptr;
	CFArrayRef utis = nullptr;
	if ( 'TEXT' != fileType )
		typeList = TypeListForType( fileType );
	else
		{
		const void * anUTI = kUTTypePlainText;
		utis = CFArrayCreate( kCFAllocatorDefault, &anUTI, 1, &kCFTypeArrayCallBacks );
		}
	
	// create the file-chooser dialog
	NavDialogRef dlg;
	if ( noErr == err )
		{
		err = NavCreateChooseFileDialog(
			&options,								// options
			typeList,								// type(s) we want to get
			sNavEventProc,							// event-handler callback
			nullptr,								// no preview proc
			nullptr,								// no filter proc
			nullptr,								// no refcon
			&dlg );
		__Check_noErr( err );
		}
	if ( noErr != err )
		goto couldntCreateDialog;
	
	// add UTI filter, if choosing TEXT files
	if ( utis )
		{
		err = NavDialogSetFilterTypeIdentifiers( dlg, utis );
		__Check_noErr( err );
		}
	
	// run the dialog, modally [for now]
	err = NavDialogRun( dlg );
	
#ifdef DEBUG_VERSION
	// make sure we know about unanticipated errors
	if ( noErr != err && userCanceledErr != err )
		__Check_noErr( err );
#endif
	
	// record user's selection, if any
	if ( noErr == err
	&&   kNavUserActionChoose == NavDialogGetUserAction( dlg ) )
		{
		// zap the input AEDesc so the real answer can go in it
		__Verify_noErr( AEDisposeDesc( reply ) );
		
		NavReplyRecord navReply;
		err = NavDialogGetReply( dlg, &navReply );
		__Check_noErr( err );
		
		// we got a good result
		if ( noErr == err )
			{
			if ( navReply.validRecord )
				{
				// ask AE manager for the first item in the list
				err = AEGetNthDesc( &navReply.selection, 1, typeFSRef, nullptr, reply );
				__Check_noErr( err );
				
				bIsValid = ( noErr == err );
				}
			
			// very important to dispose of the rest of the reply
			__Verify_noErr( NavDisposeReply( &navReply ) );
			}
		}
	
	// dispose of temporary things
	NavDialogDispose( dlg );
	
couldntCreateDialog:
	if ( typeList )
		{
		DisposeHandle( reinterpret_cast<Handle>( typeList ) );
		__Check_noErr( MemError() );
		}
	
	if ( utis )
		CFRelease( utis );
	
	if ( options.message )
		CFRelease( options.message );
	
	return ( err == noErr && bIsValid );
}


/*
**	PutNavDialog()
**
**	choose a new file using navigation services
**	On entry, 'ioName' contains the suggested name for the new file.
**	On return, 'parDir' and 'name' identify the location selected.
*/
OSStatus
PutNavDialog(
		FSRef *			parDir,
		HFSUniStr255 *	ioName,
		const char *	prompt,
		UInt32			prefKey )
{
	// set up the static callback UPP
	 if ( not sNavEventProc )
		{
		sNavEventProc = NewNavEventUPP( MyNavEventProc );
		__Check( sNavEventProc );
		}
	
	// get default options for the dialog
	NavDialogCreationOptions options;
	OSStatus err = NavGetDefaultDialogCreationOptions( &options );
	__Check_noErr( err );
	
	// bail early if it failed
	if ( noErr != err )
		return err;
	
	// modify the options to our tastes
	// can't use preserveSaveFileExt until we solve the NavCompleteSave() dilemma
//	options.optionFlags |= kNavNoTypePopup
//						| kNavDontAutoTranslate
//						| kNavPreserveSaveFileExtension
//						;
	
	// tell it our application name
	if ( CFTypeRef appName = CFBundleGetValueForInfoDictionaryKey( CFBundleGetMainBundle(),
								kCFBundleNameKey ) )
		options.clientName = static_cast<CFStringRef>( appName );
	
	// and the prompt string, if there is one
	if ( prompt )
		options.message = CreateCFString( prompt );
	
	// and the suggested file name
	if ( CFStringRef suggest = FSCreateStringFromHFSUniStr( kCFAllocatorDefault, ioName ) )
		options.saveFileName = suggest;
	
	// and the prefs key
	options.preferenceKey = prefKey;
	
	// create the put-file dialog, using those options
	NavDialogRef dlg;
	if ( noErr == err )
		{
		err = NavCreatePutFileDialog( &options, kUnknownType, kNavGenericSignature,
					sNavEventProc, nullptr, &dlg );
		__Check_noErr( err );
		}
	if ( noErr != err )
		goto couldntCreateDialog;
	
	// run the dialog, modally [for now]
	err = NavDialogRun( dlg );
	
#ifdef DEBUG_VERSION
	// make sure we know about unanticipated errors whilst debugging
	if ( noErr != err && userCanceledErr != err )
		__Check_noErr( err );
#endif
	
	// what was the outcome?
	if ( noErr == err )
		{
		NavUserAction act = NavDialogGetUserAction( dlg );
		if ( kNavUserActionSaveAs == act )
			{
			NavReplyRecord navReply;
			err = NavDialogGetReply( dlg, &navReply );
			__Check_noErr( err );
			
			// extract user's chosen file
			if ( noErr == err )
				{
//				if ( navReply.validRecord )  -- we wouldn't get a SaveAs unless it _were_ valid
					{
					// get the parent-directory
					err = AEGetNthPtr( &navReply.selection, 1, typeFSRef,
								nullptr, nullptr, parDir, sizeof *parDir, nullptr );
					__Check_noErr( err );
					
					// and the filename
					if ( noErr == err )
						{
						err = FSGetHFSUniStrFromString( navReply.saveFileName, ioName );
						__Check_noErr( err );
						}
					
#if 0
					// FIXME: This is totally the wrong place for this call.
					// It shouldn't happen until -after- the new file has been 
					// completely written out to disk.
					// -- So I turned it off.
					// Ideally I guess we'd invoke a callback to client code here,
					// to have it write its data.
					(void) NavCompleteSave( &navReply, kNavTranslateInPlace );
#endif	// 0
					}
				
				// dispose of the reply
				__Verify_noErr( NavDisposeReply( &navReply ) );
				}
			}
		else
		if ( kNavUserActionCancel == act )
			err = userCanceledErr;
		}
	
	NavDialogDispose( dlg );
	
couldntCreateDialog:
	if ( options.saveFileName )
		CFRelease( options.saveFileName );
	if ( options.message )
		CFRelease( options.message );
	
	return err;
}

#pragma mark -- Low-level file access --


/*
**	DTS_create()
**
**	create a file
**	platform dependent
*/
DTSError
DTS_create( DTSFileSpec * spec, ulong fType )	// an OSType, not an indirect FREF resID!
{
	__Check( spec );
	if ( not spec )
		return paramErr;
	
	DTSFileSpecPriv * p = spec->priv.p;
	__Check( p );
	if ( not p )
		return -1;
	
	// get parent ref and name
	FSRef parentRef;
	HFSUniStr255 name;
	OSStatus result = p->CopyToRef( &parentRef, &name, false );
	if ( noErr == result )
		{
		// initialize the (few) known pieces of metadata
		FSCatalogInfo info;
		info.textEncodingHint = kTextEncodingUnknown;
		memset( &info.finderInfo, 0, sizeof info.finderInfo );
		FileInfo& finfo = * reinterpret_cast<FileInfo *>( &info.finderInfo );
		finfo.fileCreator = gDTSApp->GetSignature();
		finfo.fileType = fType;
		
		// (try to) create it
		result = FSCreateFileUnicode( &parentRef,
						name.length,
						name.unicode,
						kFSCatInfoFinderInfo | kFSCatInfoTextEncoding,
						&info,
						nullptr,	// don't need new FSRef
						nullptr );	// nor new FSSpec
		__Check_noErr( result );
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
DTS_open( DTSFileSpec * spec, bool bWritePerm, int * fileref )
{
	__Check( spec );
	__Check( fileref );
	if ( not spec || not fileref )
		return paramErr;
	
	DTSFileSpecPriv * p = spec->priv.p;
	__Check( p );
	if ( not p )
		return -1;
	
	SInt16 refNum;
	SInt8 perm = bWritePerm ? fsRdWrPerm : fsRdPerm;
	FSRef ref;
	OSStatus result = p->CopyToRef( &ref, true );
	if ( noErr == result )
		{
		result = FSOpenFork( &ref, 0, nullptr, perm, &refNum );	// i.e., the data fork
		__Check_noErr( result );
		}
	
	if ( noErr != result )
		refNum = -1;
	
	*fileref = refNum;
	
	return result;
}


/*
**	DTS_close()
**
**	close a file
**	platform dependent
*/
void
DTS_close( int fileref )
{
	if ( fileref > 0 )
		__Verify_noErr( FSCloseFork( fileref ) );
}


/*
**	DTS_seek()
**
**	seek to the position in the file
**	platform dependent
*/
DTSError
DTS_seek( int fileref, ulong position )		// or off_t/fpos_t/size_t?
{
	OSStatus result = FSSetForkPosition( fileref, fsFromStart, position );
	__Check_noErr( result );
	
	return result;
}


/*
**	DTS_seteof()
**
**	set the end of the file
**	platform dependent
*/
DTSError
DTS_seteof( int fileref, ulong eof )
{
	OSStatus result = FSSetForkSize( fileref, fsFromStart, eof );
	__Check_noErr( result );
	
	return result;
}


/*
**	DTS_geteof()
**
**	get the end of the file
**	platform dependent
*/
DTSError
DTS_geteof( int fileref, ulong * peof )
{
	__Check( peof );
	if ( not peof )
		return paramErr;
	
	SInt64 feof;
	OSStatus result = FSGetForkSize( fileref, &feof );
	__Check_noErr( result );
	
	*peof = static_cast<ulong>( feof );
	return result;
}


/*
**	DTS_read()
**
**	read data
**	platform dependent
**	This really should have a way to report the number of bytes actually read.
*/
DTSError
DTS_read( int fileref, void * buffer, size_t size )
{
	__Check( buffer );
	if ( not buffer )
		return paramErr;
	
	OSStatus result = FSReadFork( fileref, fsAtMark, 0, size, buffer, nullptr );
	__Check_noErr( result );
	
	return result;
}


/*
**	DTS_write()
**
**	write data
**	platform dependent
**	This really should have a way to report the number of bytes actually written.
*/
DTSError
DTS_write( int fileref, const void * buffer, size_t size )
{
	__Check( buffer );
	if ( not buffer )
		return paramErr;
	
	OSStatus result = FSWriteFork( fileref, fsAtMark, 0, size, buffer, nullptr );
	__Check_noErr( result );
	
	return result;
}


/*
**	Support for Navigation Services 3.0; largely borrowed from Metrowerks' PowerPlantX.
*/

#pragma mark --NavSrvHandler--

// in lieu of declaring these next ones "static"
namespace {

/*
**	NavHandleEvent()
**
**	NavSvcs event-callback handler and dispatcher
*/
void
NavHandleEvent( NavEventCallbackMessage sel, NavCBRec * parms, void * ud )
{
	NavSrvHandler * cp = static_cast<NavSrvHandler *>( ud );
	try {
		switch ( sel )
			{
			case kNavCBUserAction:
				cp->Action( parms );
				break;
			
			case kNavCBTerminate:
				cp->Terminate( parms );
				NavDialogDispose( parms->context );
				break;
			
			default:
				cp->Event( sel, parms );
			}
		}
	catch (...) {}
}


/*
**	GetNavEventUPP()
**
**	returns a singleton UPP for the above event dispatcher.
*/
#if __MACH__
# define GetNavEventUPP()	NavHandleEvent
#else
NavEventUPP
GetNavEventUPP()
{
	static NavEventUPP upp;
	if ( not upp )
		upp = NewNavEventUPP( NavHandleEvent );
	return upp;
}
#endif  // __MACH__


/*
**	NavHandleFilter()
**
**	NavSvcs filter-callback handler and dispatcher
*/
Boolean
NavHandleFilter( AEDesc * desc, void * info, void * ud, NavFilterModes mode )
{
	__Check( ud );
	NavSrvHandler * cp = static_cast<NavSrvHandler *>( ud );
	bool bDisplayIt = true;
	
	__Check( desc );
	assert( desc );
	
	DescType itemType = desc->descriptorType;
	try
		{
		const NavFileOrFolderInfo * ffi = nullptr;
		if ( typeFSRef == itemType
		||   typeFSS == itemType )
			{
			ffi = static_cast<const NavFileOrFolderInfo *>( info );
			}
		
		bDisplayIt = cp->Filter( desc, ffi, mode );
		}
	catch (...) {}
	
	return bDisplayIt;
}


/*
**	GetFilterUPP()
**
**	returns a singleton UPP for the above filter dispatcher.
*/
#if __MACH__
# define GetFilterUPP()		NavHandleFilter
#else
NavObjectFilterUPP
GetFilterUPP()
{
	static NavObjectFilterUPP upp;
	if ( not upp )
		upp = NewNavObjectFilterUPP( NavHandleFilter );
	return upp;
}
#endif  // __MACH__


/*
**	NavHandlePreview()
**
**	NavSvcs preview-callback handler and dispatcher
*/
Boolean
NavHandlePreview( NavCBRec * parms, void * ud )
{
	NavSrvHandler * cp = static_cast<NavSrvHandler *>( ud );
	__Check( cp );
	bool bDrawn = false;
	try {
		bDrawn = cp->Preview( parms );
		}
	catch ( ... ) {}
	
	return bDrawn;
}


/*
**	GetPreviewUPP()
**
**	returns a singleton UPP for the above preview dispatcher.
*/
#if __MACH__
#define GetPreviewUPP()		NavHandlePreview
#else
NavPreviewUPP
GetPreviewUPP()
{
	static NavPreviewUPP upp;
	if ( not upp )
		upp = NewNavPreviewUPP( NavHandlePreview );
	return upp;
}
#endif  // __MACH__


/*
**	SetNavFilterUTIs()
**
**	sets the list of acceptable type-identifiers
**
**	we assume that Nav will retain the array while it's in use, so we
**	release it here ourselves.
**
**	Don't forget the fact that UTI filtration is applied _only_ to
**	ChooseFile and GetFile calls; all other Nav dialogs ignore UTIs completely!
*/
OSStatus
SetNavFilterUTIs( NavDialogRef dlg, CFArrayRef utis )
{
	OSStatus result = NavDialogSetFilterTypeIdentifiers( dlg, utis );
	__Check_noErr( result );
	
	if ( utis )
		CFRelease( utis );
	
	return result;
}


}
// end of anon namespace


/*
**	NavSrvHandler::Event()					[virtual]
**
**	subclasses should override this to provide the desired behavior
**	upon receiving a NavSvcs event notification, if they're interested in same.
**	All event notifications come through here *except*
**	kNavCBUserActions and terminations -- those have their own [virtual] handlers.
*/
void
NavSrvHandler::Event( NavEventCallbackMessage sel, NavCBRec * parms )
{
	// we only react to update & activate events; subclasses must handle anything else.
	// that includes EventMgr events like mouseDown, keyDown, autoKey, etc.
	// and also Nav-level events such as kNavCBCustomize and kNavCBStart
	if ( kNavCBEvent != sel )
		return;
	
	// perform the standard DTSLib event histrionics
	const EventRecord * er = parms->eventData.eventDataParms.event;
	if ( updateEvt == er->what )
		{
		gEvent = *er;
		HandleEvent();
		}
	else
	if ( activateEvt == er->what )
		{
		gEvent = *er;
		WindowRef actWin = WindowRef( er->message );
		if ( kDialogWindowKind == GetWindowKind( actWin ) )
			{
			// force a redraw of the underlying modal dialog (if any)
			Rect b;
			GetWindowPortBounds( actWin, &b );
			InvalWindowRect( actWin, &b );
			}
		ActivateWindows();
		}
}


/*
**	NavSrvHandler::Filter()				[virtual]
**
**	respond to a NavSvcs filtering callback.
**	subclasses may override this to do their own finer-grained filtering.
**	This default no-op implementation simply allows everything to appear.
*/
bool
NavSrvHandler::Filter( AEDesc *		/* desc */,
		const NavFileOrFolderInfo *	/* ffi */,
		NavFilterModes				/* mode */ )
{
	return true;	// show everything
}


/*
**	NavSrvHandler::Preview()				[virtual]
**
**	Respond to a NavSvcs preview callback.
**	Subclasses should override this if they wish to display their own previews.
**	This default implementation draws nothing.
*/
bool
NavSrvHandler::Preview( NavCBRec * /*parm */ )
{
	return false;	// nothing to draw
	
	// in recent OS's this isn't even used, I think.
	// apparently all previews are now handled by QuickLook instead?
}


/*
**	NavSrvHandler::Terminate()				[virtual]
**	
**	Respond to termination of the Nav dialog.
**	Subclasses may override this if they need to do any cleanup.
*/
void
NavSrvHandler::Terminate( NavCBRec * /*parms*/ )
{
}


/*
**	NavSrvHandler::CopyFilterTypeIdentifiers()	[virtual]
**
**	return a CFArray of UTIs to be filtered out.
**	Subclasses may override this in order to specify more restrictive lists;
**	this default implementation filters out none.
**
**	NOTE: the returned CFArrayRef (if not null) will be released
**	shortly after this function is called.
*/
CFArrayRef
NavSrvHandler::CopyFilterTypeIdentifiers() const
{
	return nullptr;	// don't filter anything
}


/*
**	AskChooseFile()
**
**	fire off a ChooseFile dialog and set it running.
**	Callers must ensure that the NavSrvHandler argument stays alive for the entire lifetime
**	of the dialog: don't make it a stack object!
**	Typically, your NavSrvHandler subclass will be mixed-in to some larger, more persistent
**	object, such as a Window or Document class.
*/
OSStatus
AskChooseFile( NavSrvHandler& handler, const NavDialogCreationOptions& opt )
{
	NavDialogRef dlg;
	OSStatus err = NavCreateChooseFileDialog(
		&opt,
		nullptr,	// no NavTypeListHandle
		GetNavEventUPP(),
		(opt.optionFlags & kNavAllowPreviews) ? GetPreviewUPP() : nullptr,
		GetFilterUPP(),
		&handler,
		&dlg );
	__Check_noErr( err );
	
	// request list of filterable UTIs from subclass, and use 'em
	if ( noErr == err )
		err = SetNavFilterUTIs( dlg, handler.CopyFilterTypeIdentifiers() );
	if ( noErr == err )
		{
		err = NavDialogRun( dlg );
		__Check_noErr( err );
		}
	return err;
}
		

/*
**	AskChooseFolder()
**
**	fire off a ChooseFolder dialog and set it running.
**	see warnings in AskChooseFile() above.
*/
OSStatus
AskChooseFolder( NavSrvHandler& handler, const NavDialogCreationOptions& opt )
{
	NavDialogRef dlg;
	OSStatus err = NavCreateChooseFolderDialog(
		&opt,
		GetNavEventUPP(),
		GetFilterUPP(),
		&handler,
		&dlg );
	__Check_noErr( err );
	if ( noErr == err )
		{
		err = NavDialogRun( dlg );
		__Check_noErr( err );
		}
	return err;
}
		

/*
**	AskChooseVolume()
**
**	fire off a ChooseVolume dialog and set it running.
**	see warnings in AskChooseFile() above.
*/
OSStatus
AskChooseVolume( NavSrvHandler& handler, const NavDialogCreationOptions& opt )
{
	NavDialogRef dlg;
	OSStatus err = NavCreateChooseVolumeDialog(
		&opt,
		GetNavEventUPP(),
		GetFilterUPP(),
		&handler,
		&dlg );
	__Check_noErr( err );
	if ( noErr == err )
		{
		err = NavDialogRun( dlg );
		__Check_noErr( err );
		}
	return err;
}
		

/*
**	AskChooseObject()
**
**	fire off a ChooseObject dialog and set it running.
**	see warnings in AskChooseFile() above.
*/
OSStatus
AskChooseObject( NavSrvHandler& handler, const NavDialogCreationOptions& opt )
{
	NavDialogRef dlg;
	OSStatus err = NavCreateChooseObjectDialog(
		&opt,
		GetNavEventUPP(),
		(opt.optionFlags & kNavAllowPreviews) ? GetPreviewUPP() : nullptr,
		GetFilterUPP(),
		&handler,
		&dlg );
	__Check_noErr( err );
	if ( noErr == err )
		{
		err = NavDialogRun( dlg );
		__Check_noErr( err );
		}
	return err;
}
		

/*
**	AskCreateNewFolder()
**
**	fire off a CreateNewFolder dialog and set it running.
**	see warnings in AskChooseFile() above.
*/
OSStatus
AskCreateNewFolder( NavSrvHandler& handler, const NavDialogCreationOptions& opt )
{
	NavDialogRef dlg;
	OSStatus err = NavCreateNewFolderDialog(
		&opt,
		GetNavEventUPP(),
		&handler,
		&dlg );
	__Check_noErr( err );
	if ( noErr == err )
		{
		err = NavDialogRun( dlg );
		__Check_noErr( err );
		}
	return err;
}
		

/*
**	AskGetFile()
**
**	fire off a GetFile dialog and set it running.
**	see warnings in AskChooseFile() above.
*/
OSStatus
AskGetFile( NavSrvHandler& handler, const NavDialogCreationOptions& opt )
{
	NavDialogRef dlg;
	OSStatus err = NavCreateGetFileDialog(
		&opt,
		nullptr,		// no NavTypeListHandle
		GetNavEventUPP(),
		(opt.optionFlags & kNavAllowPreviews) ? GetPreviewUPP() : nullptr,
		GetFilterUPP(),
		&handler,
		&dlg );
	__Check_noErr( err );
	
	// set UTIs
	if ( noErr == err )
		err = SetNavFilterUTIs( dlg, handler.CopyFilterTypeIdentifiers() );
	if ( noErr == err )
		{
		err = NavDialogRun( dlg );
		__Check_noErr( err );
		if ( noErr != err )
			NavDialogDispose( dlg );
		}
	return err;
}


/*
**	AskPutFile()
**
**	fire off a PutFile dialog and set it running.
**	see warnings in AskChooseFile() above.
*/
OSStatus
AskPutFile( NavSrvHandler& handler, OSType inType, OSType inCreator,
	const NavDialogCreationOptions& opt )
{
	NavDialogRef dlg;
	OSStatus err = NavCreatePutFileDialog(
		&opt,
		inType,
		inCreator,
		GetNavEventUPP(),
		&handler,
		&dlg );
	__Check_noErr( err );
	if ( noErr == err )
		{
		err = NavDialogRun( dlg );
		__Check_noErr( err );
		if ( noErr != err )
			NavDialogDispose( dlg );
		}
	return err;
}


/*
**	AskSaveChanges()
**
**	fire off an AskSaveChanges dialog and set it running.
**	see warnings in AskChooseFile() above.
*/
OSStatus
AskSaveChanges( NavSrvHandler& handler, NavAskSaveChangesAction inAction,
	const NavDialogCreationOptions& opt )
{
	NavDialogRef dlg;
	OSStatus err = NavCreateAskSaveChangesDialog(
		&opt,
		inAction,
		GetNavEventUPP(),
		&handler,
		&dlg );
	__Check_noErr( err );
	if ( noErr == err )
		{
		err = NavDialogRun( dlg );
		__Check_noErr( err );
		if ( noErr != err )
			NavDialogDispose( dlg );
		}
	return err;
}


/*
**	AskDiscardChanges()
**
**	fire off an AskDiscardChanges dialog and set it running.
**	see warnings in AskChooseFile() above.
*/
OSStatus
AskDiscardChanges( NavSrvHandler& handler, const NavDialogCreationOptions& opt )
{
	NavDialogRef dlg;
	OSStatus err = NavCreateAskDiscardChangesDialog(
		&opt,
		GetNavEventUPP(),
		&handler,
		&dlg );
	__Check_noErr( err );
	if ( noErr == err )
		{
		err = NavDialogRun( dlg );
		__Check_noErr( err );
		if ( noErr != err )
			NavDialogDispose( dlg );
		}
	return err;
}


/*
**	AskReviewDocuments()
**
**	fire off an AskReviewDocuments dialog and set it running.
**	see warnings in AskChooseFile() above.
*/
OSStatus
AskReviewDocuments( NavSrvHandler& handler, const NavDialogCreationOptions& opt, UInt32 numDocs )
{
	NavDialogRef dlg;
	OSStatus err = NavCreateAskReviewDocumentsDialog(
		&opt,
		numDocs,
		GetNavEventUPP(),
		&handler,
		&dlg );
	__Check_noErr( err );
	if ( noErr == err )
		{
		err = NavDialogRun( dlg );
		__Check_noErr( err );
		if ( noErr != err )
			NavDialogDispose( dlg );
		}
	return err;
}


