/*
**	CLLaunchHelper.c		ClanLord
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

#define __ASSERT_MACROS_DEFINE_VERSIONS_WITHOUT_UNDERSCORES		0

#include <unistd.h>
#include <syslog.h>
#include <ApplicationServices/ApplicationServices.h>

// Definitions

// Backward compatibility for <AssertMacros.h> when building with a pre-10.6 SDK.
#if defined( MAC_OS_X_VERSION_MAX_ALLOWED ) && MAC_OS_X_VERSION_MAX_ALLOWED < 1060
# define __Check( x )			check( x )
# define __Verify( x )			verify( x )
# define __Check_noErr( x )		check_noerr( x )
# define __Verify_noErr( x )	verify_noerr( x )
# define __Debug_String( x )	debug_string( x )
#endif  // MAC_OS_X_VERSION_MAX_ALLOWED < 1060

// turn this on to enable the debugging (-d) command-line flag
#define ALLOW_GDB_ATTACH	0


// things to think about...
//	if the old client is inside the /Applications folder, and the user is not an admin...
//	

// prototypes
static OSStatus		MoveToTrash( const FSRef * );


/*
**	main()
*/
int
main( int argc, char * argv[] )
{
#if ALLOW_GDB_ATTACH
	// pass "-d" flag for debugability
	if ( argc > 1 && 0 == strcmp( "-d", argv[1] ) )
		{
		// skip the flag
		--argc;
		++argv;
		
		// idles here until you attach with GDB, and do: "p unlockme=1"
		volatile int unlockme = 0;
		while ( ! unlockme )
			{
			sleep( 5 );
			}
		}
#endif  // ALLOW_GDB_ATTACH
	
	// usage: CLLaunchHelper <old> <new>
	if ( argc != 3 )
		return EXIT_FAILURE;
	
	const UInt8 * oldclient = (const UInt8 *) argv[1];
	const UInt8 * newclient = (const UInt8 *) argv[2];
	
	FSRef oldc, newc;
	Boolean isDir;
	OSStatus err;	// = noErr;
//	if ( noErr == err )
		{
		err = FSPathMakeRef( oldclient, &oldc, &isDir );
		__Check_noErr( err );
		if ( noErr == err && ! isDir )
			err = dirNFErr;
		}
	if ( noErr == err )
		{
		err = FSPathMakeRef( newclient, &newc, &isDir );
		__Check_noErr( err );
		if ( noErr == err && ! isDir )
			err = dirNFErr;
		}
	
	// get old client's parent
	FSRef parent;
	if ( noErr == err )
		{
		err = FSGetCatalogInfo( &oldc, kFSCatInfoNone, NULL, NULL, NULL, &parent );
		__Check_noErr( err );
		}
	
	// move old to trash
	if ( noErr == err )
		err = MoveToTrash( &oldc );
	
	// move new to old
	if ( noErr == err )
		{
		err = FSMoveObject( &newc, &parent, &newc );
		
		// TODO: handle a dupFNErr from here.
		// not very likely to happen in the field, but quite possible during development.
		// 
		// Here's the scenario:
		//	- Clan Lord 621.app		// current client, arbitrarily renamed by user
		//	- ClanLord.app			// correctly named client, leftover from prior fiddling
		//  - Downloads/ClanLord.app	// new client in DL folder
		//
		// After MoveToTrash() above, the "CL621" app is gone, but the FSMoveObject()
		// nevertheless fails, thanks to the leftover, and results in a dupFNErr.
		//
		// To fix this, just FSDeleteObject()'ing the innocent leftover isn't enough
		// (it'd need the equivalent of "rm -R"); but it might be OK to either
		// MoveToTrash() it, or rename it in place ("ClanLord copy NN.app").
		
		__Check_noErr( err );
		if ( err )
			syslog( LOG_WARNING, "Emplace new client: %d", (int) err );
		}
	
	// "touch" the new client so the OS notices it
	if ( noErr == err )
		{
		FSCatalogInfo info;
		err = FSGetCatalogInfo( &newc, kFSCatInfoContentMod, &info, NULL, NULL, NULL );
		__Check_noErr( err );
		if ( noErr == err )
			{
			err = UCConvertCFAbsoluteTimeToUTCDateTime( CFAbsoluteTimeGetCurrent(),
						&info.contentModDate );
			__Check_noErr( err );
			}
		if ( noErr == err )
			{
			__Verify_noErr( FSSetCatalogInfo( &newc, kFSCatInfoContentMod, &info ) );
			}
		
		__Verify_noErr( FNNotify( &parent, kFNDirectoryModifiedMessage, kNilOptions ) );
		}
	
	// create the special "auto-join" parameter (via keyAEPropData parameter to 'oapp' event)
	AERecord parms = { typeNull, NULL };
	if ( noErr == err )
		{
		err = AEBuildDesc( &parms, NULL, "{'AuDL':1}" );
		__Check_noErr( err );
		}
	
	// fire it up!
	if ( noErr == err )
		{
		LSLaunchFSRefSpec launchSpec;
		launchSpec.appRef = &newc;
		launchSpec.numDocs = 0;
		launchSpec.itemRefs = NULL;
		launchSpec.passThruParams = &parms;
		launchSpec.launchFlags = kLSLaunchDefaults;
		launchSpec.asyncRefCon = NULL;
		
		err = LSOpenFromRefSpec( &launchSpec, NULL );
		__Check_noErr( err );
		}
	
	// clean up after ourselves
	__Verify_noErr( AEDisposeDesc( &parms ) );
	
	return noErr == err ? EXIT_SUCCESS : EXIT_FAILURE;
}


/*
**	MoveToTrash()
**	tries semi-hard to move an object into the user's Trash.
**	handles the case where a similarly-named object already exists in the trash;
**	might not handle the case where the trash is on a different volume.
*/
OSStatus
MoveToTrash( const FSRef * fsr )
{
	// paranoia
//	if ( ! fsr )
//		return paramErr;
	
	// get actual old client name, just in case we need to rename it
	// also its volumeID
	HFSUniStr255 fname;
	FSCatalogInfo info;
	FSRef target = *fsr;
	CFStringRef oldName = NULL;
	OSStatus err;
//	if ( noErr == err )
		{
		err = FSGetCatalogInfo( &target, kFSCatInfoVolume, &info, &fname, NULL, NULL );
		__Check_noErr( err );
		if ( noErr == err )
			{
			oldName = FSCreateStringFromHFSUniStr( kCFAllocatorDefault, &fname );
			__Check( oldName );
			if ( ! oldName )
				err = memFullErr;
			}
		}
	
	// locate trash folder -- hopefully this will be on the same volume as the file being deleted
	FSRef trash;
	if ( noErr == err )
		{
		err = FSFindFolder( /*kUserDomain*/ info.volume, kTrashFolderType, kCreateFolder, &trash );
		__Check_noErr( err );
		}
	
	// move it
	if ( noErr == err )
		{
		int counter = 0;
		
		// first try the move exactly as things are
		err = FSMoveObject( &target, &trash, NULL );
		
		// does an even OLDER app -- with the same name -- already exist in the trash?
		while ( dupFNErr == err )
			{
			// if so, rename this one (to "NN.oldName") and try again.
			CFStringRef newName = CFStringCreateWithFormat( kCFAllocatorDefault, NULL,
							CFSTR("%d.%@"), ++counter, oldName );
			if ( newName )
				{
				err = FSGetHFSUniStrFromString( newName, &fname );
				__Check_noErr( err );
				}
			else
				err = memFullErr;
			
			// rename object being deleted; 'target' continues to point to it afterward
			if ( noErr == err )
				{
				err = FSRenameUnicode( &target, fname.length, fname.unicode,
						kTextEncodingUnknown, &target );
				__Check_noErr( err );
				}
			
			// re-attempt the move
			if ( noErr == err )
				err = FSMoveObject( &target, &trash, NULL );
			
			if ( newName )
				CFRelease( newName );
			}
		}
	
	// by now we either have a successful move, or FSMove() had an error _other_ than dupFNErr,
	// or the Rename() failed, or the CFString stuff died.  Not much more we can do to recover.
	
	// housekeeping
	if ( oldName )
		CFRelease( oldName );
	
	// for laffs, compare the foregoing hunk 'o code with this AppleScript one-liner:
	//	"tell app Finder to move 'fsr' to trash"
	
	return err;
}

