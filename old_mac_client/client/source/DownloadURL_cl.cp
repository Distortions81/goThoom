/*
**	DownloadURL_cl.cp		ClanLord
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

#include <sys/wait.h>
#include <libgen.h>
#include <unistd.h>
#include <zlib.h>
#include <spawn.h>

#include "ClanLord.h"
#include "LaunchURL_cl.h"
#include "ProgressWin_cl.h"


/*
**	Entry Routines
*/
/*
DTSError AutoUpdate( LogOn * msg );
DTSError ExpandFile( DTSFileSpec * file );
*/


/*
**	Definitions
*/
const char * const	kClientPrefix		= TXTCL_APPNAME;
const char * const	kMacSubDirectory	= "mac";
const char * const	kDataSubDirectory	= "data";
const char * const	kDownloadFolder		= "Downloads";

const int			kVersionCD			= 64 << 8;


/*
**	Internal Routines
*/
static DTSError		AutoUpdateClient( LogOn * msg );
static DTSError		AutoUpdateImagesSounds( LogOn * msg );
static DTSError		AutoUpdateDataFile( const LogOn * msg, const char * dir,
						const char * filename );
static DTSError		ReadSubDirectory( const char * url, const char * dir,
						const char * & listing );
static const char *	ExtractFileName( const char * start, const char * search,
						char * filename, size_t size );
static DTSError		InternalDownloadURL( const char * url, DTSFileSpec * spec,
						CFMutableDataRef dest );
static DTSError		GotNewClient( DTSFileSpec& spec );
static DTSError		AbdicateToNewClient( CFURLRef u );
static DTSError		CreateURLForUnTarballedClient( CFURLRef inURL, CFURLRef& oURL );
static DTSError		MoveRenameDataFile( const char * filename );
static OSStatus		DoCFDownloadURL( CFURLRef inUrl,
						const FSRef& dstDir, const HFSUniStr255& fName,
						CFMutableDataRef destDataRef );


/*
**	Internal Variables
*/

// gNewClient (if valid) refers to a newly-downloaded, updated client app
// to which the currently-running instance must switch-launch when it exits.
static CFURLRef	gNewClient;		// easier to use than a DTSFileSpec


/*
**	AutoUpdate()
**
**	check for new files
*/
DTSError
AutoUpdate( LogOn * msg )
{
	// do we need to get a newer client?
	DTSError result;
	uint clientversion = gPrefsData.pdClientVersion;
	if ( clientversion < BigToNativeEndian( msg->logClientVersion ) )
		{
		result = AutoUpdateClient( msg );
		
		// we successfully auto updated the client
		// we're about to quit
		// we really need to return an error so
		// the caller doesn't try to do anything else
		// pick a funky error code, highly unlikely to emerge from the file mgr
		if ( noErr == result )
			result = appVersionTooOld;
		}
	// get the latest images and sounds
	else
		{
		// auto update the images and sounds
		// with only the latest client
		// older clients might remove the wrong crap
		// from the data files
		// which would be bad
		result = AutoUpdateImagesSounds( msg );
		}
	
	return result;
}


#ifdef ARINDAL
/*
**	ChooseLang()
**
**	Return a prefix according to the language this
**	client was compiled in.
**	(for English, return the empty string)
**
*/
static inline
const char *
ChooseLang()
{
# if CLT_LANG == CLT_LANG_GERMAN
	return "_de.";
# elif CLT_LANG == CLT_LANG_DUTCH
	return "_nl.";
# elif CLT_LANG == CLT_LANG_FRENCH
	return "_fr.";
# else
	return ".";	
# endif
}
#endif // ARINDAL


/*
**	AutoUpdateClient()
**
**	check for a new client
*/
DTSError
AutoUpdateClient( LogOn * msg )
{
	// get base download URL from the logOn message
	// typical value: "https://www.deltatao.com/downloads/clanlord/"
	const char * baseURL = msg->logData;
	
#if 0 && defined( DEBUG_VERSION )
	// during development of the MachO auto-update system, we remap the live server's
	// standard reply onto a separate testbed URL
	if ( 0 == strcmp( baseURL, "http://www.deltatao.com/downloads/clanlord/" )
	||   0 == strcmp( baseURL, "https://www.deltatao.com/downloads/clanlord/" ) )
		{
		baseURL = "https://www.deltatao.com/downloads/cltest/";
		}
#endif  // debug test
	
	char buff[ 512 ];
	// "Checking for program updates... refer to %s for help troubleshooting."
	snprintf( buff, sizeof buff, _(TXTCL_CHECKING_FOR_CLIENT_UPDATE), kTroubleURL );
	ShowInfoText( buff );
	
#ifdef ARINDAL
	const char * wantLang = ChooseLang();
#endif // ARINDAL
	
	// mix it all up to yield the full final URL
	// "https://www.deltatao.com/downloads/clanlord/mac/ClanLord.702.tgz"
	int major = BigToNativeEndian( msg->logClientVersion ) >> 8;
	int minor = BigToNativeEndian( msg->logClientVersion ) & 0x0FF;
	if ( 0 == minor )
		{
		// no minor version
#ifdef ARINDAL
		snprintf( buff, sizeof buff, "%s%s/%s%s%d.tgz",
			baseURL, kMacSubDirectory, kClientPrefix, wantLang, major );
#else
		snprintf( buff, sizeof buff, "%s%s/%s.%d.tgz",
			baseURL, kMacSubDirectory, kClientPrefix, major );
#endif // ARINDAL
		}
	else
		{
		// we DO have a (non-zero) minor version number
#ifdef ARINDAL
		snprintf( buff, sizeof buff, "%s%s/%s%s%d.%d.tgz", 
			baseURL, kMacSubDirectory, kClientPrefix, wantLang, major, minor );
#else
		snprintf( buff, sizeof buff, "%s%s/%s.%d.%d.tgz",
			baseURL, kMacSubDirectory, kClientPrefix, major, minor );
#endif
		}
	
	// try to go and grab it
	DTSFileSpec spec;
	
#define DEBUG_SKIP_DOWNLOAD	0
#if DEBUG_SKIP_DOWNLOAD
	// for testing; assume that a suitable "imitation" downloaded file
	// already exists at a certain fixed path
	DTSError result = spec.SetFileName( ":Downloads:ClanLord.test.tgz" );
#else
	DTSError result = InternalDownloadURL( buff, &spec, nullptr );
#endif  // DEBUG_SKIP_DOWNLOAD
	
	// and then install it!
	if ( noErr == result )
		result = GotNewClient( spec );
	
	// otherwise, whoops
	if ( result != noErr )
		{
		// overwrite the logon message with a failure notice;
		// this will be displayed to the user [in Comm_cl.cp:GetLogOnResponse()]
		snprintf( msg->logData, sizeof msg->logData,
			// "Could not download the new client. %d"
			_(TXTCL_FAILED_CLIENT_UPDATE),
			result );
		}
	
	return result;
}


/*
**	AutoUpdateImagesSounds()
**
**	auto update the images and sounds files
*/
DTSError
AutoUpdateImagesSounds( LogOn * msg )
{
	// get the version number of the images file
	uint imagesversion = gImagesVersion;
	uint soundsversion = gSoundsVersion;
	
	// brain damage for testing
//	imagesversion -= 0x200;
//	soundsversion -= 0x200;
	
	uint liveImgVers = BigToNativeEndian( msg->logImagesVersion );
	uint liveSndVers = BigToNativeEndian( msg->logSoundsVersion );
	// nothing new?
	bool bNeedImages = ( imagesversion < liveImgVers );
	bool bNeedSounds = ( soundsversion < liveSndVers );
	if ( not bNeedImages  &&  not bNeedSounds )
		return noErr;
	
	// "* Checking for data updates... refer to %s for help troubleshooting."
	char buff[ 512 ];
	snprintf( buff, sizeof buff, _(TXTCL_CHECKING_FOR_DATA_UPDATE), kTroubleURL );
	ShowInfoText( buff );
	
	// there -might- be something new
	// go check the directory
	const char * dir = nullptr;
	DTSError result = ReadSubDirectory( msg->logData, kDataSubDirectory, dir );
	
	// search for filenames
	if ( noErr == result )
		{
		char filename[ 40 ];
		int neededversion;
		bool bReOpenKeyFiles = false;
		if ( bNeedImages )
			{
			// find the perfect update file
			//
			// (We keep "perfect" updates for the last three versions on the
			// website, e.g. for v100 we'd have individual delta files for
			// 97->100, 94->100, and 91->100, in addition to the full complete
			// v100 image data file. So even if you haven't played for a couple
			// of months, we can still fix you up with a fairly small,
			// incremental patcher.
			//
			//	For a while we also maintained a v64->vCurrent incremental
			//	file, because v64 is what was pressed onto the physical Clan
			//	Lord CDs, and so we could assume that everyone had at least
			//	that. But due to image churn, there's no longer any space/time
			//	savings there, compared to fetching the complete images file
			//	from scratch.)
			//
			neededversion = liveImgVers >> 8;
			snprintf( filename, sizeof filename, "%s.%dto%d.gz",
				kClientImagesFName, (imagesversion >> 8), neededversion );
			result = AutoUpdateDataFile( msg, dir, filename );
			if ( noErr == result )
				result = UpdateImagesFile( neededversion );
			
#if 0	// we stopped making a CD-to-current images updater;
		// there were no file-size savings compared to the whole file updater
			// the perfect update file wasn't there
			// try updating from the cd
			if ( result > 0
			&&   imagesversion > (uint) kVersionCD )
				{
				snprintf( filename, sizeof filename, "%s.%dto%d.gz",
					kClientImagesFName, (kVersionCD >> 8), neededversion );
				result = AutoUpdateDataFile( msg, dir, filename );
				if ( noErr == result )
					result = UpdateImagesFile( neededversion );
				}
#endif  // 0
			
			// no perfect updater; try downloading the whole file
			if ( result > 0 )
				{
				snprintf( filename, sizeof filename,
					"%s.%d.gz", kClientImagesFName, neededversion );
				result = AutoUpdateDataFile( msg, dir, filename );
				if ( noErr == result )
					{
					// move old CL_Images from the app folder to the trash folder
					// rename filename to CL_Images
					// move the new CL_Images to the app folder
					// ...
					result = MoveRenameDataFile( filename );
					if ( noErr == result )
						bReOpenKeyFiles = true;
					}
				}
			}
		if ( noErr == result  &&  bNeedSounds )
			{
			// find the perfect update
			neededversion = liveSndVers >> 8;
			snprintf( filename, sizeof filename, "%s.%dto%d.gz",
				kClientSoundsFName, (soundsversion >> 8), neededversion );
			result = AutoUpdateDataFile( msg, dir, filename );
			if ( noErr == result )
				result = UpdateSoundsFile( neededversion );
			
			// the perfect update file wasn't available
			// try updating from the CD
			// (there IS a space savings using vCD-to-vCurrent sounds, unlike images)
#if 0	// time passes, and the claim above is no longer true; the
		// CD-to-current sound updater is no smaller than the real thing
			if ( result > 0
			&&   soundsversion > (uint) kVersionCD )
				{
				snprintf( filename, sizeof filename, "%s.%dto%d.gz",
					kClientSoundsFName, (kVersionCD >> 8), neededversion );
				result = AutoUpdateDataFile( msg, dir, filename );
				if ( noErr == result )
					result = UpdateSoundsFile( neededversion );
				}
#endif  // 0
			
			// last resort: try downloading the whole file
			if ( result > 0 )
				{
				snprintf( filename, sizeof filename, "%s.%d.gz",
					kClientSoundsFName, neededversion );
				result = AutoUpdateDataFile( msg, dir, filename );
				if ( noErr == result )
					{
					// move CL_Sounds from the app folder to the trash folder
					// rename filename to CL_Sounds
					// move the new CL_Sounds to the app folder
					// ...
					result = MoveRenameDataFile( filename );
					if ( noErr == result )
						bReOpenKeyFiles = true;
					}
				}
			}
		if ( noErr == result  &&  bReOpenKeyFiles )
			{
			CloseKeyFiles();
			OpenKeyFiles();
			FlushCaches( true );
			ChangeWindowSize();
			}
		
		delete[] dir;
		}
	
	// set the error message
	if ( result != noErr )
		{
		snprintf( msg->logData, sizeof msg->logData,
			// "Could not download data files. %d"
			_(TXTCL_FAILED_DATA_UPDATE),
			result );
		}
	
	// irrational sanity checks
	// the update files might be corrupt
	if ( noErr == result )
		{
		if ( ( bNeedImages && uint(gImagesVersion) < liveImgVers )
		||   ( bNeedSounds && uint(gSoundsVersion) < liveSndVers ) )
			{
			// "The downloaded update files seem to be corrupt. "
			//	"Please contact joe@deltatao.com."
			snprintf( msg->logData, sizeof msg->logData, "%s",
				_(TXTCL_CORRUPT_UPDATE_FILES) );
			result = -1;
			}
		}
	
	// we successfully auto updated the images and sounds files.
	// we need to return a special "error" number.
	// this tells the caller to try logging in again.
	if ( noErr == result )
		{
		snprintf( msg->logData, sizeof msg->logData, "%s",
			// "Successfully downloaded data files."
			_(TXTCL_DATA_FILES_SUCCESS) );
		result = kDownloadNewVersionLive;
		}
	
	return result;
}


/*
**	AutoUpdateDataFile()
**
**	parse the directory looking for the filename
**	download, unstuff, and move it if we find it
**
**	return >0 if the file was not found
*/
DTSError
AutoUpdateDataFile( const LogOn * msg, const char * dir, const char * findme )
{
	// get base download URL from the logOn message
	const char * baseURL = msg->logData;
	
	// search for a filename that looks promising
	size_t len = strlen( findme );
	char filename[ 40 ];
	const char * search = ExtractFileName( dir, nullptr, filename, sizeof filename );
	for(;;)
		{
		if ( not search )
			return 1;	// not found
		
		// check for the filename
		if ( 0 == memcmp( filename, findme, len ) )
			{
			// download it
			char url[ 512 ];
			snprintf( url, sizeof url, "%s%s/%s",
				baseURL, kDataSubDirectory, filename );
			DTSFileSpec spec;
			DTSError result = InternalDownloadURL( url, &spec, nullptr );
			if ( noErr == result )
				result = ExpandFile( &spec );
			return result;
			}
		
		search = ExtractFileName( nullptr, search, filename, sizeof filename );
		}
}


/*
**	ReadSubDirectory()
**
**	read a subdirectory
**	return a newly allocated pointer to the contents.
**	caller is responsible for deleting said pointer.
*/
DTSError
ReadSubDirectory( const char * url, const char * dir, const char * & listing )
{
	CFMutableDataRef dataRef = CFDataCreateMutable( kCFAllocatorDefault, 0 );
	__Check( dataRef );
	if ( not dataRef )
		return memFullErr;
	
	DTSError result = noErr;
//	if ( noErr == result )
		{
		char buff[ 512 ];
		// the trailing '/' is necessary for directories, I guess.
		snprintf( buff, sizeof buff, "%s%s/", url, dir );
		
		// download directory data into the CFData
		result = InternalDownloadURL( buff, nullptr, dataRef );
		}
	
	char * ptr = nullptr;
	if ( noErr == result )
		{
		// transfer CFData contents to a C++ pointer
		// kinda wasteful but what the hey.
		CFIndex len = CFDataGetLength( dataRef );
		ptr = NEW_TAG("SubDirectory") char[ len + 1 ];
		if ( ptr )
			{
			if ( len )
				{
				CFDataGetBytes( dataRef, CFRangeMake( 0, len ),
								reinterpret_cast<UInt8 *>( ptr ) );
				}
			ptr[ len ] = 0;
			}
		else
			result = memFullErr;
		}
	
	if ( dataRef )
		CFRelease( dataRef );
	
	listing = ptr;
	
	return result;
}


/*
**	ExtractFileName()
**
**	extract a file name from an html link
**
**	This assumes that your webserver is set up to serve directory index listings
**	(possibly autogenerated) for [at least] the "mac/" and "data/" subdirectories.
**	For Apache we used .htaccess files with directives like these:
**		Options +Indexes
**		IndexOptions FancyIndexing NameWidth=*
**	The key thing is that each valid file in the directory must be represented
**	in the returned HTML by (at least) an <a> element of the form:
**		<a href="CL_Images.1070to1079.gz">
**	We scan for the '<a href="' prefix, and slurp up everything until the next
**	double-quote. (What could possibly go wrong?!?)
*/
const char *
ExtractFileName( const char * start, const char * filename, char * buff, size_t sz )
{
	// we're possibly resuming a previous search
	if ( not start )
		start = filename + 1;
	
	// this is extraordinarily fragile
#define kSearchPrefix		"<A HREF=\""
	
	// locate start of next filename candidate
	const char * found = strcasestr( start, kSearchPrefix );
	if ( not found )
		return nullptr;
	
	found += sizeof kSearchPrefix - 1;
	
	// locate end of filename, assuming quote at end of href
	const char * end = strchr( found, '\"' );
	size_t len;
	if ( end )
		len = end - found;
	else
		len = strlen( found );
	
	// ignore candidates with impossibly long names
	if ( len >= sz )
		return nullptr;
	
	memcpy( buff, found, len );
	buff[ len ] = 0;
	
	found += len;
	
	// where to begin searching next time, if this candidate file isn't suitable
	return found;
}


/*
**	GotNewClient()
**
**	We've successfully retrieved a new client app from the server.
**	Now decompress and decode it, install it in the right place,
**	and hand over the reins.
*/
DTSError
GotNewClient( DTSFileSpec& spec )
{
	// uncompress it
	CFURLRef newClientURL = nullptr;
	DTSError result;
	if ( CFURLRef compressedURL = DTSFileSpec_CreateURL( &spec ) )
		{
		result = CreateURLForUnTarballedClient( compressedURL, newClientURL );
		CFRelease( compressedURL );
		}
	else
		result = coreFoundationUnknownErr;
	
	// arrange to run it
	if ( noErr == result && newClientURL )
		result = AbdicateToNewClient( newClientURL );
	
	return result;
}


/*
**	MoveFileSafer()
**
**	like CatMove, but won't return dupFnErr -- instead it renames the
**	conflicting file in the destination folder to something vaguely helpful
*/
static DTSError
MoveFileSafer( DTSFileSpec * source, DTSFileSpec * dest )
{
	DTSError result = noErr;
	for (;;)
		{
		// try the move
		result = source->Move( dest );
		
		// if successful, or any error except dupFN, we're done
		if ( noErr == result  ||  dupFNErr != result )
			break;
		
		// someone's been sleeping in my bed.
		// and they're still here!
		
		// get the name of the offending file
		// (this breaks if the source is actually a directory, not a file...
		// e.g. if trying to move the bundled MachO client app.
		// But in that case, we'll never even get here.)
		DTSFileSpec interloper = *dest;
		const char * srcName = source->GetFileName();
		interloper.SetFileName( srcName );
		
		// HFS names are 31 characters long, max, I believe.
		// please correct me if I'm wrong!
		const short kHFSFileNameMax = 31;
		
		// construct a new name of the form "nn.oldname"
		// where nn starts at 1 and increases. If we get to 99 without
		// finding a free slot, we better give up.
		int counter = 0;
		do {
			char name[ kHFSFileNameMax + 1 ];
			snprintf( name, sizeof name, "%.2d.%s", ++counter, srcName );
			result = interloper.Rename( name );
			}
		while ( noErr != result  &&  counter < 100 );
		
		// if that worked, go back and try the CatMove again
		// else there was a file system error, and
		// we might as well stop everything now.
		if ( result != noErr )
			break;
		}
	
	return result;
}


/*
**	AbdicateToNewClient()
**
**	terminate self in deference to the new client.
**	store a URL to the new client in preparation for LaunchNewClient()
**	which is the very last thing done by main()
*/
DTSError
AbdicateToNewClient( CFURLRef u )
{
	if ( not u )
		return noErr;
	
	// save the URL
	gNewClient = u;
	
#if 0
	// tell 'em what's about to happen
	if ( CFStringRef leaf = CFURLCopyLastPathComponent( u ) )
		{
//		"A new version of the " TXTCL_APP_NAME " client with the name "
//		LDQUO "%@" RDQUO " has been downloaded, and your current client"
//		"has been moved into the trash. This old client will now quit, "
//		"and the new one will start up."
		if ( CFStringRef fmt = CFCopyLocalizedString( CFSTR("NEW_CLIENT_RESTART"),
									CFSTR("Auto-update progress message") ) )
			{
			if ( CFStringRef msg = CFStringCreateFormatted( fmt, leaf  ) )
				{
				ShowMsgDlg( msg );
				CFRelease( msg );
				}
			CFRelease( fmt );
			}
		CFRelease( leaf );
		}
#endif  // 0
	
	// quit the current client
	// (this is a -very- roundabout way of saying "gDoneFlag = true" !!)
	const ProcessSerialNumber kThisProcess = { 0, kCurrentProcess };
	AppleEvent quitMe = { typeNull, nullptr };
	DTSError result;
//	if ( noErr == result )
		{
		result = AEBuildAppleEvent( kCoreEventClass, kAEQuitApplication,
					typeProcessSerialNumber,
					&kThisProcess, sizeof kThisProcess,
					kAutoGenerateReturnID,
					kAnyTransactionID,
					&quitMe,
					nullptr,	// err
					"" );		// no params
		__Check_noErr( result );
		}
	
	if ( noErr == result )
		{
		AppleEvent reply = { typeNull, nullptr };
		result = AESend( &quitMe, &reply, kAENoReply, kAENormalPriority,
					kNoTimeOut, nullptr, nullptr );
		__Check_noErr( result );
		AEDisposeDesc( &reply );
		}
	
	AEDisposeDesc( &quitMe );
	
	return result;
}


/*
**	IsGoodApplication()
**
**	does it smell like a valid application?
*/
static bool
IsGoodApplication( CFURLRef ref )
{
	if ( not ref )
		return false;
	
	LSItemInfoRecord info;
	OSStatus err = LSCopyItemInfoForURL( ref, kLSRequestBasicFlagsOnly, &info );
	__Check_noErr( err );
	
	// does LaunchServices think it's an app?
	return noErr == err &&
			((info.flags & kLSItemInfoIsApplication) != 0);
}


// our internal helper tool acts as a sort of trampoline or stepping-stone
// between the current client and a newly-downloaded replacement one.
// Its source is in CLLaunchHelper.c.
#define HELPER_TOOL_NAME	"CLLaunchHelper"

// when this is on, the helper tool spins in a slow idle loop at start,
// so that you can attach to it with a debugger.
#define ENABLE_DEBUG_FOR_HELPER		0

// are we using fork()/exec(), or the newer preferred alternative,
// namely posix_spawn()?
#define	USE_POSIX_SPAWN		1

/*
**	LaunchHelperTool()
**
**	invoke our internal helper tool, via fork/exec (or posix_spawn)
*/
static void
LaunchHelperTool( const char * helper_path, char * old_path, char * new_path )
{
	// usage: CLLaunchHelper [-d] <old> <new>
	
#if USE_POSIX_SPAWN
	
	// the argv array needs non-const members
	char helperToolName[] = HELPER_TOOL_NAME;
# if ENABLE_DEBUG_FOR_HELPER
	char debugFlag[] = "-d";
# endif
	
	// helper's argv[]
	char * theArgs[] = {
		helperToolName,
# if ENABLE_DEBUG_FOR_HELPER
		debugFlag,
# endif
		old_path,
		new_path,
		nullptr
		};
	
	int result = posix_spawn( nullptr,	// don't need child's PID
			helper_path,			// full path to new process executable
			nullptr,				// no special file actions
			nullptr,				// no special attributes
			theArgs,				// argv[]
			nullptr );				// no special environ
	if ( result )
		perror( "spawn" );
	
#else  // ! USE_POSIX_SPAWN
	
	pid_t pid = vfork();
	if ( -1 == pid )
		perror( "vfork" );
	else
	if ( 0 == pid )
		{
		// child
		int result = execl( helper_path,
							HELPER_TOOL_NAME,
# if ENABLE_DEBUG_FOR_HELPER
							"-d",
# endif
							old_path,
							new_path,
							nullptr );
		if ( -1 == result )
			{
			perror( "execl" );
			
			// if execl() failed, we're super hosed, because of the vfork().
			// the only safe thing left is to _exit() -- even plain old exit()
			// is too much.
			_exit( EXIT_FAILURE );
			}
		}
#endif  // USE_POSIX_SPAWN
	
	// (parent will self-destruct in ~0.03 femtoseconds)
}


/*
**	LaunchNewClient()
**
**	Call this as the last dying action of the current client
**	after all files, endpoints, connections are closed.
**
**	It invokes a separate minimal helper tool to do the dirtiest deeds
**	(in particular, the OS doesn't seem to appreciate it if the current,
**	live client attempts to move _itself_ into the trash).
*/
void
LaunchNewClient()
{
	// check gNewClient for validity
	if ( not IsGoodApplication( gNewClient ) )
		return;
	
	// get paths to old (i.e. current) and new (replacement) client apps
	char old_path[ PATH_MAX + 1 ];
	char new_path[ PATH_MAX + 1 ];
	CFBundleRef bndl = CFBundleGetMainBundle();
	OSStatus err = noErr;
//	if ( noErr == err )
		{
		if ( not CFURLGetFileSystemRepresentation( gNewClient, true,
					reinterpret_cast<uchar *>( new_path ), sizeof new_path ) )
			{
			err = coreFoundationUnknownErr;
			__Check_noErr( err );
			}
		}
	if ( noErr == err )
		{
		if ( CFURLRef me = CFBundleCopyBundleURL( bndl ) )
			{
			if ( not CFURLGetFileSystemRepresentation( me, true,
						reinterpret_cast<uchar *>( old_path ), sizeof old_path ) )
				{
				err = coreFoundationUnknownErr;
				}
			CFRelease( me );
			}
		else
			err = fnfErr;	// ??? CFBCBU() failed?
		}
	
	// get path to internal helper tool
	char helper_path[ PATH_MAX + 1 ];
	if ( noErr == err )
		{
		if ( CFURLRef u = CFBundleCopyAuxiliaryExecutableURL( bndl,
							CFSTR(HELPER_TOOL_NAME) ) )
			{
			if ( not CFURLGetFileSystemRepresentation( u, true,
						reinterpret_cast<uchar *>( helper_path ),
						sizeof helper_path ) )
				{
				err = coreFoundationUnknownErr;
				}
			CFRelease( u );
			}
		else
			err = fnfErr;
		}
	
	// fire it up!
	if ( noErr == err )
		LaunchHelperTool( helper_path, old_path, new_path );
}


#pragma mark -

/*
**	InternalDownloadURL()
**
**	Downloads a given URL to a destination of your choice.
**
**	If destSpec is non-NULL, then the url is downloaded as a file,
**	and *destSpec will point to the result. destData is ignored in this case.
**	This is the scenario for downloading app & data-file updates.
**
**	Alternately, pass NULL for destSpec and a valid CFMutableData in destData.
**	That will download the url into the CFData. You're responsible for
**	disposing of the CFData after you're done with it, obviously.
**	This mode is typically used when scanning directory listings.
*/
DTSError
InternalDownloadURL( const char * url, DTSFileSpec * destSpec,
	CFMutableDataRef destData )
{
#if 0 && defined( DEBUG_VERSION )
	ShowMessage( "DownloadURL: %s", url );
#endif
	
	DTSError result = noErr;
	
	FSRef parDir;
	DTSFileSpec saveSpec;
	saveSpec.GetCurDir();
	const char * fname = nullptr;
	HFSUniStr255 destName;
	
	// if downloading as a file, figure out where it will go
	if ( destSpec )
		{
		// put a message into the sidebar
		// BULLET " Downloading updater data..."
		ShowInfoText( _(TXTCL_DOWNLOADING_DATA) );

#if DOWNLOAD_INTO_TEMP_FOLDER
		// put it into the system's Temporary Items folder.
		// that implies a possible cross-volume copy, later on...
		result = FSFindFolder( kOnAppropriateDisk,
							   kTemporaryFolderType, kCreateFolder, &parDir );
		
		// make it current
		if ( noErr == result )
			{
			DTSFileSpec_CopyFromFSRef( destSpec, &parDir );
			result = destSpec->SetDir();
			}
#else
		// use local "Downloads" folder
		// no cross-device copies, but could break if the client
		// is on a read-only volume
		destSpec->SetFileName( kDownloadFolder );
		(void) destSpec->CreateDir();
		result = destSpec->SetDir();
		if ( noErr != result )
			{
			// "Unable to create a download directory. (%d)"
			GenericError( _(TXTCL_UNABLE_CREATE_DOWNLOAD_DIR), result );
			}
#endif	// DOWNLOAD_INTO_TEMP_FOLDER
		
		// create an FSRef to the directory in which we want the file to end up
		if ( noErr == result )
			{
			// TO DO: double-check that we're not outsmarting ourselves
			// with the following.
			
			// if the URL does not end with a slash character
			// then we want to use the item's actual name from the URL.
			// Otherwise, it's a directory, and so we
			// do need to provide a real name.
			
			destSpec->GetCurDir();
			
			fname = strrchr( url, '/' );
			if ( not fname )
				result = -1;
			}
		if ( noErr == result )
			{
			// make ref to parent dir
			destSpec->SetFileName( "" );
			result = DTSFileSpec_CopyToFSRef( destSpec, &parDir );
			__Check_noErr( result );
			
			// grab dest name
			++fname;
			if ( '\0' == fname[0] )
				fname = "DirectoryListing";
			
			destSpec->SetFileName( fname );
			
			// ensure there's no pre-existing file with that name
			destSpec->Delete();
			
			// convert leafname to Unicode
			if ( CFStringRef cfName = CreateCFString( fname ) )
				{
				result = FSGetHFSUniStrFromString( cfName, &destName );
				__Check_noErr( result );
				CFRelease( cfName );
				}
			else
				result = memFullErr;
			}
		}
	else
		{
		// no destSpec. Let's zero out the destName, though, to prevent errors
		destName.length = 0;
		}
	
	if ( noErr == result )
		{
		// make a new absolute URL from the supplied address
		if ( CFURLRef urlRef = CFURLCreateWithBytes( kCFAllocatorDefault,
									reinterpret_cast<const uchar *>( url ),
									strlen( url ),
									kCFStringEncodingASCII, nullptr ) )
			{
			// ask CFNetwork to do the downloading
			result = DoCFDownloadURL( urlRef, parDir, destName, destData );
			CFRelease( urlRef );
			}
		else
			result = memFullErr;
		}
	
	saveSpec.SetDirNoPath();
	
	return result;
}


/*
**	scaffolding for expanding .gz files.
*/
static void
zerror( const char * s )
{
#ifndef DEBUG_VERSION
# pragma unused( s )
#else
	VDebugStr( "%s", s );
#endif
}


/*
**	ZLibExpand()
**
**	expand a gzipped file
**	on successful exit, 'spec' points to the new, expanded file.
*/
static DTSError
ZLibExpand( DTSFileSpec * spec, bool inDeleteOriginal /* =false */ )
{
	DTSError result = noErr;
	
	// make sure the filename ends with ".gz"
	size_t namelen = strlen( spec->GetFileName() );
	if ( strcmp( spec->GetFileName() + namelen - 3, ".gz" ) != 0 )
		return bdNamErr;
	
	// open the input file for reading
	FILE * input_file = spec->fopen( "rb" );
	if ( not input_file )
		return fnfErr;
	
	// get its file descriptor
	int infile = fileno( input_file );
	if ( -1 == infile )
		{
		// how the heck did that happen?
		fclose( input_file );
		return fnfErr;
		}
	
	// and prepare the gzip decoder to read from it
	gzFile z_file = gzdopen( infile, "rb" );
	
	// now prepare the output side of the equation
	// same volume, directory, name ...
	DTSFileSpec outfile = *spec;
	
	// ... but trim off the .gz extension
	{
	char buff[ 256 ];
	StringCopySafe( buff, outfile.GetFileName(), sizeof buff );
	buff[ namelen - 3 ] = '\0';
	outfile.SetFileName( buff );
	}
	
	// delete previous leftovers
	(void) outfile.Delete();
	
	// open 'er up
	FILE * output_file = outfile.fopen( "wb" );
	if ( not output_file )
		{
		fclose( input_file );
		return fnOpnErr;
		}
	
	// get a buffer to decompress into
	const size_t kBufferSize = 128 * 1024;
    char * buf = NEW_TAG( "zlib buffer" ) char[ kBufferSize ];
	if ( not buf )
		{
		fclose( output_file );
		fclose( input_file );
		return memFullErr;
		}
	
	// your basic read-write loop
	int zerr = 0;	// gzerror() specifically wants an 'int'
    for (;;)
    	{
        int len = gzread( z_file, buf, kBufferSize );
        if ( len < 0 )
        	{
        	zerror( gzerror( z_file, &zerr ) );
        	break;
        	}
        else
        if ( 0 == len )
        	// all done
        	break;
		
		// write 'len' bytes
        if ( fwrite( buf, 1, len, output_file ) != static_cast<size_t>( len ) )
        	{
		    zerror( "failed fwrite" );
		    // concoct an error code
		    if ( not zerr )
		    	zerr = ioErr;
		    }
    	}
    result = zerr;
    
    // close up shop
    if ( fclose( output_file ) )
    	zerror( "failed fclose" );
	
    if ( Z_OK != gzclose( z_file ) )
    	zerror( "failed gzclose" );
	
	delete[] buf;
	
#if 0	// we no longer macbinarize anything
	
	// OK, now try to demacbinarize it
	// ** should we check for ".bin" suffix here first?
	FSRef binRef;
	if ( noErr == result )
		result = DTSFileSpec_CopyToFSRef( &outfile, &binRef );
	if ( noErr == result )
		{
		// the type code of 0 is meant to make the Finder treat
		// the file as "temporary" (see TN 1142, "Mac OS 8.5")
		result = FSDecodeMacBinary( &binRef, gDTSApp->GetSignature(),
					0, smSystemScript );
		if ( kNotMacBinaryErr == result )
			result = noErr;
		}
#endif  // 0
	
	// politely erase the evidence
	if ( noErr == result
	&&	 inDeleteOriginal )
		{
		(void) spec->Delete();
		}
	
	// update the filespec
	if ( noErr == result )
		{
		// strip off ".bin", if present
		const char * outname = outfile.GetFileName();
		namelen = strlen( outname ) - 4;
		if ( 0 == memcmp( outname + namelen, ".bin", 4 ) )
			{
			char buff[ 256 ];
			StringCopySafe( buff, outname, sizeof buff );
			buff[ namelen ] = 0;
			outfile.SetFileName( buff );
			}
		
		*spec = outfile;
		}
	
	return result;
}


/*
**	ExpandFile()
**
**	unstuff a file using zlib
**	if successful, 'file' is updated to the new file
*/ 
DTSError
ExpandFile( DTSFileSpec * file )
{
	__Check( file );
	if ( not file )
		return paramErr;
	
	// show visible progress
	// BULLET " Decompressing update file..."
	ShowInfoText( _(TXTCL_DECOMPRESS_UPDATE_FILE) );
	
	// un zlib it, deleting original if successful
	DTSError result = ZLibExpand( file, true );
	
	// absorb soft errors
	if ( result >= 0 )
		result = noErr;
	
	return result;
}


/*
**	PerformSystemCommand()
**
**	a simple wrapper around system(3)
*/
static DTSError
__attribute__(( __nonnull__ ))
PerformSystemCommand( const char * command )
{
	int res = ::system( command );
	if ( WIFEXITED( res ) )
		{
		if ( 0 == WEXITSTATUS( res ) )
			return noErr;	// all's well
		}
	
	// something went wrong somewhere along the way
	return -1;
}


/*
**	CreateURLForUnTarballedClient()
**
**	the MachO client is distributed as a tarball.
**	expand it, and return a URL for the outer bundle wrapper directory.
*/
DTSError
CreateURLForUnTarballedClient( CFURLRef compressed, CFURLRef& outURL )
{
	DTSError err = noErr;
	
	// get full path to tarball file
	char path[ PATH_MAX + 1 ];
	if ( not CFURLGetFileSystemRepresentation( compressed, true,
				reinterpret_cast<uchar *>( path ), sizeof path ) )
		err = coreFoundationUnknownErr;
	__Check_noErr( err );
	
	// find a temporary folder (the Downloads dir, i.e., the parent of 'ref')
	char tmpPath[ PATH_MAX + 1 ];
	StringCopySafe( tmpPath, path, sizeof tmpPath );
	if ( const char * dir = dirname( tmpPath ) )
		StringCopySafe( tmpPath, dir, sizeof tmpPath );
	else
		err = -1;		// ???
	
	// expand the tarball into the temp folder
	char * cmd = nullptr;
	if ( noErr == err )
		{
		// ask shell to perform the following:
		/*
			#!/bin/sh
			cd "<path to downloads dir>"
			/usr/bin/tar -xzf "<name of tarball file>"
		*/
		const char * base = basename( path );
		if ( not base )
			base = path;
		
		int result = asprintf( &cmd,
						"cd \"%s\"; /usr/bin/tar xzf \"%s\"",
						tmpPath, base );
		if ( -1 == result || not cmd )
			err = memFullErr;
		__Check_noErr( err );
		}
	if ( noErr == err )
		err = PerformSystemCommand( cmd );
	
	if ( cmd )
		::free( cmd );
	
	// if OK, delete the tarball (unless we're testing)
#if not DEBUG_SKIP_DOWNLOAD
	if ( noErr == err )
		CFURLDestroyResource( compressed, nullptr );
#endif
	
	// generate the output URL
	if ( noErr == err )
		{
		// FIXME: get true client-bundle name
		//	sh -c 'basename $(tar tzf <path> | head -n 1)'
#if 1
# define NEW_CLIENT_NAME	"ClanLord.app"
#else
# define NEW_CLIENT_NAME	"ClanLord+.app"
#endif
	
		// e.g. "/some/path/to/<client dir>/Downloads/"
		if ( CFURLRef u = CFURLCreateCopyDeletingLastPathComponent(
							kCFAllocatorDefault, compressed ) )
			{
			// ".../Downloads/ClanLord.app/"
			outURL = CFURLCreateCopyAppendingPathComponent(
						kCFAllocatorDefault,
						u, CFSTR(NEW_CLIENT_NAME), true );
			CFRelease( u );
			}
		}
	
	return err;
}


/*
**	MoveRenameDataFile()
**
**	move and rename the images and sounds files
*/
DTSError
MoveRenameDataFile( const char * filename )
{
	// build a file spec
	char buff[ 256 ];
	snprintf( buff, sizeof buff, ":%s:%s", kDownloadFolder, filename );
	
	// chop off the .gz because the expander doesn't
	size_t len = strlen( buff ) - 3;
	if ( 0 == memcmp( buff + len, ".gz", 3 ) )
		buff[ len ] = 0;
	
#if 0 // haven't used MacBinary in ages
	// also chop off .bin if present
	len -= 4;
	if ( 0 == memcmp( buff + len, ".bin", 4 ) )
		buff[ len ] = 0;
#endif
	
	DTSFileSpec spec;
	spec.SetFileName( buff );
	
	// crush interlopers
	DTSFileSpec interloper = spec;
	
	StringCopySafe( buff, interloper.GetFileName(), sizeof buff );
	if ( char * suffix = strchr( buff, '.' ) )
		{
		*suffix = 0;
		interloper.SetFileName( buff );
		(void) interloper.Delete();
		}
	
	// rename it
	DTSError result = spec.Rename( interloper.GetFileName() );
	
	// get the current directory
	DTSFileSpec curDirSpec;
	if ( noErr == result )
		result = curDirSpec.GetCurDir();
	
	// move it
	if ( noErr == result )
		{
		// prevent fnfErr during update
		// See remarks about GetCurDir() above, at the
		// end of SwapPlacesWith...()
		curDirSpec.SetFileName( "" );
		result = MoveFileSafer( &spec, &curDirSpec );
		}
	
	return result;
}


#pragma mark -

// Newer, better OSX downloading routine, using CFNetwork and friends.
// Requires OS X 10.2 or better.
// Much of this was lifted from Apple's "CFNetworkHTTPDownload" sample code,
// circa 2005, but apparently no longer available online.

// how long to wait for URL data before giving up on the connection (seconds)
/*
	originally this was just 10 seconds. But it seems that on very slow
	connections (eg. noisy dialup), and if the OS internally uses very large
	data buffers, at it might, then there can be an arbitrarily long time
	between each HasBytesAvailable notice. So I've decided to use a ridiculously
	long timeout -- effectively, none at all -- on the theory that
		a) if the connection _truly_ dies, we'll get an Error or End event,
			and can close things down cleanly;
		b) if the user, watching the progress bar, decides things are wedged,
			she can always cancel the download manually; and therefore
		c) if neither of the above happens, we should let the transfer continue,
			no matter how slow.
*/
#define kMaxURLDrySpell		kEventDurationDay


//	Download support vars
static EventLoopTimerRef	sNetworkTimer;
		// timer to terminate a download that fails in midstream
static volatile bool		bCFDownloadInProgress;
		// true iff we're doing a download

// call this to stop/abort the current download
static void		TerminateDownload( CFReadStreamRef stream, volatile void * ctxt );

// Information about the current download and its progress.
// we need to D/L either into a file or a CFData.
// This context object tells us which it is.
// Also it exposes progress info to top-level callers, to enable status dialogs etc.
struct DLContext
{
	SInt32		refnum;			// refnum of file being written, or <= 0
	CFMutableDataRef	h;		// holder for data being read, if no refnum
	CFIndex		bytesRead;		// so far
	OSStatus	lastErr;		// most recent error result
	OSStatus	cumulativeErr;	// worst-yet result
	CFIndex		expectedLen;	// from remote HTTP server. 
								//		0: unknown, -1: not available
};

// another context record, used by the watchdog timer to interrupt a stuck D/L.
struct TimerCtxt
{
	CFReadStreamRef			stream;
	volatile DLContext *	ctxt;
};


/*
**	GetLenFromHeader()
**
**	after we get our first trickle of data from the CFReadStream,
**	we can interrogate it to see if there's a "Content-Length" header
**	in the HTTP response. If so, that'll tell us the final expected length
**	of the stream. (barring any catastrophes...)
*/
static void
GetLenFromHeader( CFReadStreamRef stream, volatile DLContext * c )
{
	bool got = false;
	
	// extract the HTTP response header object
	if ( CFTypeRef prop = CFReadStreamCopyProperty( stream,
							kCFStreamPropertyHTTPResponseHeader ) )
		{
		// extract the Content-Length header field
		if ( CFStringRef ss = CFHTTPMessageCopyHeaderFieldValue( 
								(CFHTTPMessageRef) prop,
								CFSTR("Content-Length") ) )
			{
			// parse out the length from that
			int contentLength = CFStringGetIntValue( ss );
			if ( contentLength > 0 )
				{
				// yay! we got something
				got = true;
				c->expectedLen = contentLength;
#if 0 && defined( DEBUG_VERSION )
				ShowMessage( "DL expected length is %d", contentLength );
#endif
				}
			CFRelease( ss );
			}
		CFRelease( prop );
		}
	
	if ( not got )
		{
		// There was no length header, or we couldn't parse it.
		// Record that fact in the context, to prevent further
		// length-measurement attempts.
		c->expectedLen = -1;
		}
}


/*
**	ReadStreamClientCallback()
**
**	called by the stream (or rather by the runloop on which the
**	stream is scheduled) whenever any notable stream event takes place.
**	If there is some fresh data, we'll save it and update the
**	progress values. If anything goes wrong, or we arrive at the end
**	of the transfer, close up shop. For now, just ignore everything else.
*/
static void
ReadStreamClientCallback( CFReadStreamRef stream,
	CFStreamEventType evtType, void * ud )
{
	try
		{
		volatile DLContext * c = static_cast<DLContext *>( ud );
		
		switch ( evtType )
			{
				//
				// get yer nice fresh data now!
				//
			case kCFStreamEventHasBytesAvailable:
				{
				if ( 0 == c->expectedLen )
					{
					// if we haven't yet looked for the stream length, do so now.
					GetLenFromHeader( stream, c );
					}
				
				// now try to read the data.
				// First we'll try to read whatever (internal) data the stream
				// object may already have cached for us. If that fails, we'll
				// do an explicit read into our own buffer, at the cost of some
				// duplicative memcpy()'s.

				uchar buffer[ 8192 ];		// 8k fallback buffer
				CFIndex bytesRead;
				
				// request the maximum possible amount of data
				const CFIndex kAsMuchAsYouCan = -1;
				const uchar * data = CFReadStreamGetBuffer( stream,
										kAsMuchAsYouCan, &bytesRead );
				if ( not data )
					{
					// fallback: read into our own buffer
					bytesRead = CFReadStreamRead( stream, buffer, sizeof buffer );
					data = buffer;
					}
				
				// did we get anything?
				if ( bytesRead > 0 )
					{
					OSStatus err = noErr;
					
					// still alive; postpone the deadman switch
					if ( sNetworkTimer )
						{
						// FIXME/TODO?
						// maybe recalibrate this timeout based on current
						// overall data rate? We'd have to know the size of
						// CFNetwork's internal buffers, though...
						
					 	__Verify_noErr( SetEventLoopTimerNextFireTime( sNetworkTimer,
											kMaxURLDrySpell ) );
						}
					
					// append the data to its destination: file or CFData
					if ( c->refnum > 0 )
						{
						err = FSWriteFork( c->refnum, fsAtMark, 0,
								bytesRead, data, nullptr );
						__Check_noErr( err );
						}
					else
					if ( c->h )
						CFDataAppendBytes( c->h, data, bytesRead );
					
#if 0 && defined( DEBUG_VERSION )
					ShowMessage( "ReadStream got data: chunk %ld total %ld",
						bytesRead, c->bytesRead );
#endif
					c->bytesRead += bytesRead;
					c->lastErr = err;
					
					// update worst error -- or, more accurately, "first error"
					if ( err != noErr && noErr == c->cumulativeErr )
						c->cumulativeErr = err;
					}
				else
				if ( bytesRead < 0 )
					{
					// some kind of error condition
//					CFStreamError streamErr = CFReadStreamGetError( stream );
					if ( CFErrorRef streamErr = CFReadStreamCopyError( stream ) )
						{
#if defined( DEBUG_VERSION )
//						ShowMessage( "ReadStream got %ld, domain %ld, err %ld",
//							bytesRead, streamErr.domain, streamErr.error );
						ShowMessage( "ReadStream got %ld, err %ld",
							bytesRead, CFErrorGetCode( streamErr ) );
						// maybe diplay this, too, somehow:
//						CFStringRef errDomain = CFErrorGetDomain( streamErr );
//						CFShow( errDomain );
#endif
//						c->lastErr = streamErr.error;
						c->lastErr = CFErrorGetCode( streamErr );
						
						CFRelease( streamErr );
						}
					else
						{
						//	no CFError, but bytes < 0 ?!? How'd that happen?
						c->lastErr = coreFoundationUnknownErr;
						}
					
					// this is surely the wrong constant, but
					// I want to be able to recognize when this happens
					c->cumulativeErr = kNetworkDisconnect;
					TerminateDownload( stream, c );
					}
				else
					{
					// must have been 0 bytes, i.e. end-of-stream.
#if 0 && defined( DEBUG_VERSION )
					ShowMessage( "ReadStream got EOF" );
#endif
					TerminateDownload( stream, c );
					}
				}
				break;
			
			case kCFStreamEventEndEncountered:
#if 0 && defined( DEBUG_VERSION )
				ShowMessage( "ReadStream got end-encountered" );
#endif
				TerminateDownload( stream, c );
				break;
			
			case kCFStreamEventErrorOccurred:
				{
//				CFStreamError streamErr = CFReadStreamGetError( stream );
				if ( CFErrorRef streamErr = CFReadStreamCopyError( stream ) )
					{
#if defined( DEBUG_VERSION )
//					ShowMessage( "ReadStream got error domain %ld, err %ld",
//						streamErr.domain, streamErr.error );
					ShowMessage( "ReadStream got error %ld",
						CFErrorGetCode( streamErr ) );
					// error domain?
#endif
//					c->lastErr = streamErr.error;
					c->lastErr = CFErrorGetCode( streamErr );
					CFRelease( streamErr );
					}
				else
					{
					// error occurred, but no CFErrorRef?
					// You got some 'splainin' to do!
					c->lastErr = coreFoundationUnknownErr;
					}
				
				c->cumulativeErr = kNetworkDisconnect;
					// again, maybe not the best error code
				TerminateDownload( stream, c );
				}
				break;
			
			default:
				break;
			}
		}
	catch ( ... )
		{
		}
}


/*
**	NetworkTimeoutProc()
**
**	called by the event loop timer after X seconds of inactivity on the download.
**	Kills off the stream.
*/
static void
NetworkTimeoutProc( EventLoopTimerRef /*inTimer*/, void * ud )
{
	try
		{
#if defined( DEBUG_VERSION )
		ShowMessage( "ReadStream timed out." );
#endif
		
		__Check( ud );
		assert( ud );
		
		if ( TimerCtxt * c = static_cast<TimerCtxt *>( ud ) )
			{
			CFReadStreamRef stream = c->stream;
			__Check( c->ctxt );
			assert( c->ctxt );
			
			c->ctxt->cumulativeErr = kNetworkHandshakeTimeout;	// sorta
			TerminateDownload( stream, c->ctxt );
			}
		}
	catch ( ... )
		{
		}
}


/*
**	TerminateDownload()
**
**	close the D/L stream and finish off the download
*/
void
TerminateDownload( CFReadStreamRef stream, volatile void * ctxt )
{
	__Check( stream );
	__Check( ctxt );
	assert( stream );
	assert( ctxt );
	
	// unschedule the dead-man switch
	__Verify_noErr( SetEventLoopTimerNextFireTime( sNetworkTimer,
						kEventDurationForever ) );
	
	// unhook the stream callbacks, to prevent being called at a bad moment
	(void) CFReadStreamSetClient( stream, nullptr, nullptr, nullptr );
	
	// unhook it from the runloop too
	CFReadStreamUnscheduleFromRunLoop( stream, 
			(CFRunLoopRef) GetCFRunLoopFromEventLoop( GetCurrentEventLoop() ),
			kCFRunLoopCommonModes );
	
	// we're done with the stream: close it and lose it.
	CFReadStreamClose( stream );

	CFRelease( stream );
	
	// we're no longer downloading
	bCFDownloadInProgress = false;
	
	// close the output file, if there was one
	volatile DLContext * info = static_cast<volatile DLContext *>( ctxt );
	if ( info && info->refnum > 0 )
		{
		__Verify_noErr( FSCloseFork( info->refnum ) );
		
		info->refnum = -1;
		}
}


/*
**	DoCFDownloadURL()
**
**	do the download via CFNetwork.
**	Essentially the same semantics as InternalDownloadURL(): that is,
**	if 'destDataRef' is non-NULL, we'll download into it,
**	and 'destRef' & 'fname' are ignored.
**	Otherwise, the latter two identify the destination file.
*/
OSStatus
DoCFDownloadURL( CFURLRef url,
				 const FSRef& destRef, const HFSUniStr255& fname,
				 CFMutableDataRef destDataRef )
{
	OSStatus result = noErr;
	
	// Set up the progress/context record
	volatile DLContext info;
	info.refnum = -1;			// no file opened yet
	info.h = destDataRef;		// can be null
	info.bytesRead = 0;			// nothing read so far
	info.lastErr = noErr;		// no errors encountered
	info.cumulativeErr = noErr;	// ditto
	info.expectedLen = 0;		// dunno how big it will be
	
	// are we downloading to memory or to a file?
	if ( not destDataRef )
		{
		FSIORefNum refnum = -1;		// pessimism
		
		// make (and open) the file now, please.
		result = FSCreateFileAndOpenForkUnicode( &destRef,
					fname.length, fname.unicode,
					kFSCatInfoNone, nullptr,	// no special catInfo
					0, nullptr,					// data-fork
					fsRdWrPerm, &refnum, nullptr );
		__Check_noErr( result );
		
		// bail now if we broke something
		if ( noErr != result )
			return result;
		
		info.refnum = refnum;
		}
	
	// stream context record (we'll only be using its 'info' field)
	CFStreamClientContext ctxt;
	memset( &ctxt, 0, sizeof ctxt );
	
	// These are the network events we're interested in.
	const CFOptionFlags kOurNetworkEvents =
			/* kCFStreamEventOpenCompleted | */
			kCFStreamEventHasBytesAvailable |
			kCFStreamEventEndEncountered |
			kCFStreamEventErrorOccurred;
	
	// user-data for the timer callback:
	// what it needs to be able to kill off the D/L.
	TimerCtxt timerCtxt;
	
	// the CFRunloop on which our stream will be scheduled
	CFRunLoopRef curLoop = (CFRunLoopRef)
		GetCFRunLoopFromEventLoop( GetCurrentEventLoop() );
	
	// create an HTTP "GET" request for the desired URL
	CFReadStreamRef readStreamRef = nullptr;
	CFHTTPMessageRef messageRef = CFHTTPMessageCreateRequest(
						kCFAllocatorDefault,
						CFSTR("GET"), url, kCFHTTPVersion1_1 );
	if ( not messageRef )
		goto Bail;
	
	// create the stream for reading the request's response.
	// (the request itself will be sent as soon as we open the read-stream)
	readStreamRef = CFReadStreamCreateForHTTPRequest( kCFAllocatorDefault,
						messageRef );
	if ( not readStreamRef )
		goto Bail;
	
	// turn on autoredirection, just in case the URL is stale
	// (don't really care if the property wasn't settable)
	(void) CFReadStreamSetProperty( readStreamRef,
				kCFStreamPropertyHTTPShouldAutoredirect, kCFBooleanTrue );
	
	// set up the stream's client context record
	ctxt.info = const_cast<DLContext *>( &info );	// the _real_ context info
	
	// install the callback onto the stream
	if ( not CFReadStreamSetClient( readStreamRef, kOurNetworkEvents,
				ReadStreamClientCallback, &ctxt ) )
		{
		goto Bail;
		}
	
	// Schedule the stream on the current runloop
	CFReadStreamScheduleWithRunLoop( readStreamRef,
		curLoop, kCFRunLoopCommonModes );
	
	// Start the HTTP connection, send the request, await response
	if ( not CFReadStreamOpen( readStreamRef ) )
	  goto Bail;
	
	bCFDownloadInProgress = true;
	
	// Set up a watchdog timer to terminate the download after X seconds.
	// zorch any leftover timer, from previous DLs
	if ( sNetworkTimer )
		{
		__Verify_noErr( RemoveEventLoopTimer( sNetworkTimer ) );
		}
	
	// user-data for the timer callback:
	// what it needs to be able to kill off the D/L.
	timerCtxt.stream = readStreamRef;
	timerCtxt.ctxt   = &info;
	
	// install the timer and start it running
	result = InstallEventLoopTimer( GetCurrentEventLoop(),
					kMaxURLDrySpell,			// initial delay
					kEventDurationNoWait,		// no repeat interval
					NetworkTimeoutProc,
					&timerCtxt,
					&sNetworkTimer );
	__Check_noErr( result );
	
	if ( messageRef )
		CFRelease( messageRef );	// the stream owns this now
	
	if ( noErr == result )
		{
		// now sit around and do nothing until the download terminates
//		long oldSleep = gDTSApp->SetSleepTime( 30 );
		
		// show the progress window (download variant)
		StProgressWindow pw( fname );
		
		while ( bCFDownloadInProgress )
			{
			RunApp();
			
			// show progress, if any
//			ShowMessage( "Bytes read: %ld of %ld",
//				info.bytesRead, info.expectedLen );
			if ( info.expectedLen <= 0 )
				{
//				ShowMessage( "Expected Length is %ld", info.expectedLen );
				pw.SetProgress( 0, 0 );			// indeterminate
				}
			else
				{
#if 0 && defined( DEBUG_VERSION )
 				ShowMessage( "Rcv %ld of %ld = %d%%",
					info.bytesRead, info.expectedLen,
					int(( 100 * info.bytesRead ) / info.expectedLen) );
#endif
				pw.SetProgress( info.bytesRead, info.expectedLen );
				}
			
			// user might get bored...
			if ( gDoneFlag )
				{
#if 0 && defined( DEBUG_VERSION )
				ShowMessage( "D/L canceled." );
#endif
				info.cumulativeErr = userCanceledErr;
				TerminateDownload( readStreamRef, &info );
				break;
				}
			}
		}
	
	result = info.cumulativeErr;
#if 0 && defined( DEBUG_VERSION )
	ShowMessage( "D/L done; total bytes: %ld, error: %d",
		info.bytesRead, int(result) );
#endif
	
	// remove the timer
	if ( sNetworkTimer )
		{
		__Verify_noErr( RemoveEventLoopTimer( sNetworkTimer ) );
		sNetworkTimer = nullptr;
		}
	
	return result;
	
	// something awful happened
Bail:
	if ( info.refnum > 0 )
		{
		// close the destination file, if any
		__Verify_noErr( FSCloseFork( info.refnum ) );
		}
	
	// release the HTTP message
	if ( messageRef )
		CFRelease( messageRef );
	
	// and the read stream
	if ( readStreamRef )
		{
		// don't forget to unhook the callbacks 'n stuff!
		CFReadStreamSetClient( readStreamRef, nullptr, nullptr, nullptr );
		CFReadStreamUnscheduleFromRunLoop( readStreamRef,
			curLoop, kCFRunLoopCommonModes );
		CFReadStreamClose( readStreamRef );
		CFRelease( readStreamRef );
		}
	
	return -1;
}
