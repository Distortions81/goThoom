/*
**	FriendsList_cl.cp		ClanLord
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
**	Entry Functions
*/
/*
DTSError	SetPermFriend( const char * n, int friendType );
DTSError	ReadFriends( const char * myName );
DTSError	WriteFriends( const char * myName );
int			GetPermFriend( const char * n );
DTSError	InitFriends();
*/

const int kDefaultFriendLabel	= kFriendLabel2;	// orange, closest to former "friends" color


/*
**	Internal definitions
*/
class FriendLink : public DTSLinkedList<FriendLink>
{
protected:
	const char *	name_;
	uint			flags_;
	
					FriendLink() : name_(nullptr), flags_(0) {}
	virtual 		~FriendLink() { delete[] name_; }
	
public:
	
	explicit		FriendLink( const char * nom, uint flags = 0 ) :
						name_(),
						flags_( flags )
						{
						char * s = NEW_TAG("FriendLink::name_") char[ std::strlen( nom ) + 1 ];
						if ( s )
							std::strcpy( s, nom );
						name_ = s;
						}
	
	FriendLink *	GetNext()	const	{ return linkNext;	}
	const char *	GetName()	const	{ return name_;		}
	uint			GetFlags()	const	{ return flags_;	}
	
	void			SetFlags( uint f )	{ flags_ = f; }
};


#define kGlobalFriendFile		"*global*"
#define kBlockedPlayersFile		"*Blocked*"
#define kIgnoredPlayersFile		"*Ignored*"


/*
	Friends are stored in 27 separate linked lists, one for each letter of
	the alphabet plus one extra for non-alphas. Each friend's name is linked
	into the list corresponding to the first letter of the name. Within each
	linked list, there's no particular order, nor does it really matter.
	This is probably a lousy choice of data structure -- I'm not sure whether
	players will have tens, hundreds, or thousands of friends.
	
	Each friend is stored as a FriendLink, which holds the actual name, plus
	a long's worth of flags, which determine the degree of friendship (or block-
	edness, or globality) of the name in question.
*/


class FriendArray
{
public:
	static const int kMaxFriendLinks = ('Z' - 'A' + 1 + 1);
/*
	enum {
		kFlagLocal = 1,
		kFlagGlobal = 2,
		kFlagBlocked = 4,
		kFlagIgnored = 8
		};
*/
		
	enum {	
		kFriendArrayGlobalShift		= 0,
		kFriendArrayLocalShift		= 1,
		kFriendArrayLabelShift		= 2,
		kFriendArrayBlockedShift	= 5,
		kFriendArrayIgnoredShift	= 6,
		
		kFriendArrayGlobalMask		= 0x00000001,
		kFriendArrayLocalMask		= 0x00000002,	
			// local & global are separate bits so their dirty status can be tracked separately
		kFriendArrayFileTypeMask	= 0x00000003,
		kFriendArrayLabelMask		= 0x0000001C,
			// next 3 bits are label number 1-5, block (6), or ignore (7)
		kFriendArrayBlockedMask		= 0x00000020,
			// next 2 bits used only during conversion of old-format global files
		kFriendArrayIgnoredMask		= 0x00000040	
		};
	
					FriendArray();
					~FriendArray();
	
	DTSError		Init() const		{ return noErr; }
	
	void			Clear();
	
	DTSError		ReadFile( const char * playerName, uint flags = 0 );
	DTSError		ParseLine( char * line, uint flags, uint * dirty );
	DTSError		WriteFile( const char * playerName, uint flagMask = 0 );
	DTSError		DeleteFile( const char * filename ) const;
	
	DTSError		Add( const char * name, uint flags = 0 );
	DTSError		Remove( const char * name );
	
	bool			Contains( const char * name, uint * flags, uint flagMask = 0 );
	
private:
	FriendLink *	linkHeads_[ kMaxFriendLinks ];
	uint			dirty_;
	
	void			SetDirty( uint isDirty )	{ dirty_ = isDirty; }
	
	FriendLink **	Classify( int initial );
	FriendLink *	Find( const char * name, uint flagMask = 0 );
	
	DTSError		WriteArrayToStream( std::FILE * stream, uint flagMask );
	DTSError		LocateFriendFolder( DTSFileSpec * spec ) const;
};


/*
** global data
*/

FriendArray * gFriendArray;


/*
** FriendsArray::FriendsArray()
**
**	constructor
*/
FriendArray::FriendArray() :
	dirty_() 
{
	for ( int i = 0; i < kMaxFriendLinks; ++i )
		linkHeads_[i] = 0;
}


/*
** FriendsArray::~FriendsArray()
**
**	destructor
*/
FriendArray::~FriendArray()
{
	Clear();
}


/*
**	FriendArray::Clear()
**
**	empty it out
*/
void
FriendArray::Clear()
{
	for ( int i = 0; i < kMaxFriendLinks; ++i )
		FriendLink::DeleteLinkedList( linkHeads_[i] );
	
	SetDirty( 0 );
}


/*
** FriendsArray::Classify()		[private]
**
**	determine the correct linked-list head to search
*/
FriendLink **
FriendArray::Classify( int initial )
{
	initial = std::toupper( initial );
	
	// this test intentionally does NOT use isupper()
	if ( initial >= 'A' && initial <= 'Z' )
		return &linkHeads_[ initial - 'A' ];
	else
		return &linkHeads_[ kMaxFriendLinks - 1 ];
}


/*
** FriendsArray::Find()		[private]
**
**	locate first name with the right flags in the right list, or NULL
**	default flagMask = 0 allows any set of flags
*/
FriendLink *
FriendArray::Find( const char * name, uint flagMask /* = 0 */ )
{
	// just to be safe
	if ( not name[0] )
		return nullptr;
	
	FriendLink ** root = Classify( name[0] );
	for ( FriendLink * link = *root;  link;  link = link->GetNext() )
		{
		if ( flagMask == (link->GetFlags() & flagMask)
		&&	 not std::strcmp( link->GetName(), name ) )
			{
			return link;
			}
		}
	return nullptr;
}


/*
** FriendsArray::Contains()
**
**	is this name, perhaps with these flags, in any list?
*/
bool
FriendArray::Contains( const char * name, uint * flags, uint flagMask /* = 0 */ )
{
	FriendLink * p = Find( name, flagMask );
	if ( p && flags )
		*flags = p->GetFlags();
	
	return p != nullptr;
}


/*
** FriendsArray::Add()
**
**	insert a name in its list, if it's not already present
*/
DTSError
FriendArray::Add( const char * name, uint flags /* =0 */ )
{
	// just to be safe
	if ( *name == 0 )
		return noErr;
	
	uint newFiletype = flags & kFriendArrayFileTypeMask;
	
	// do we already have a node with this file type for this player?
	FriendLink * link = Find( name, newFiletype );
	if ( link )
		{
		// override the earlier value
		// mark this file as dirty to remove hand-written duplicate lines, if any
		link->SetFlags( flags );
		SetDirty( flags | dirty_ );
		return noErr;
		}
	
	// are we adding a local node that has an existing global one?
	if ( kFriendArrayLocalMask == newFiletype )
		{
		link = Find( name, kFriendArrayGlobalMask );
		
		if ( link )
			{
			uint oldFlags = link->GetFlags();
			if ( (oldFlags & kFriendArrayLabelMask) == (flags & kFriendArrayLabelMask) )
				{
				// same label as existing global node: do nothing
				return noErr;
				}
			
			// else fall through and create a new entry
			}
		else
		if ( kFriendNone == ((flags & kFriendArrayLabelMask) >> kFriendArrayLabelShift) )
			{
			// local "none" with no global for it to override: do nothing
			return noErr;
			}
		
		// else fall through and create a new entry
		}
	
	// it's a new one: make an entry
	link = NEW_TAG("FriendLink") FriendLink( name, flags );
	if ( not link )
		return memFullErr;
	
	// install it into the proper link head
	// and mark it as needing to be flushed to file
	FriendLink ** root = Classify( name[0] );
	link->InstallFirst( *root );
	SetDirty( flags | dirty_ );
	
	return noErr;
}


/*
**	FriendsArray::Remove()
**
**	Remove a name from the list, unless it's absent
*/
DTSError
FriendArray::Remove( const char * name )
{
	if ( *name == 0 )
		return -1;
	
	FriendLink * link = Find( name );
	if ( not link )
		return -1;
	
	SetDirty( dirty_ | link->GetFlags() );
	FriendLink ** root = Classify( name[0] );
	link->Remove( *root );
	return noErr;
}


/*
**	FriendArray::LocateFriendFolder()
**
**	return the filespec of the friends folder
*/
DTSError
FriendArray::LocateFriendFolder( DTSFileSpec * spec ) const
{
	// locate the folder - assume cur dir is app dir
	spec->SetFileName( "Friends" );
	DTSError result = spec->SetDir();
	if ( noErr != result )
		{
		result = spec->CreateDir();
		if ( noErr == result )
			result = spec->SetDir();
		}
	
	return result;
}


/*
** FriendsArray::ReadFile()
**
**	read a list of names from a text file, adding each
*/
DTSError
FriendArray::ReadFile( const char * playerName, uint flags /* =0 */ )
{
	DTSFileSpec spec, savedDir;
	savedDir.GetCurDir();
	
	DTSError result = LocateFriendFolder( &spec );
	
	// logic changed by timmer
	// old logic had problems when LocateFriendFolder failed
	
	std::FILE * stream = nullptr;
	if ( noErr == result )
		{
			{
			// some characters just don't belong in filenames
			char buff[ 256 ];
			StringCopySafe( buff, playerName, sizeof buff );
			for ( char * p = buff; *p; ++p )
				{
				if ( *p == ':'			// HFS path separator
				||	 *p == '/' )		// OSX Unix ditto
					{
					*p = '-';			// should be innocuous everywhere
					}
				}
			spec.SetFileName( buff );
			}
		
		stream = spec.fopen( "r" );
		}
	
	savedDir.SetDirNoPath();
	
	// if there's no file, don't try to read it
	// but DO continue trying to read the other files
	if ( stream )
		{
		uint dirty = 0;
		
		// prevent zillions of error messages if they try to read a UTF16 file
		bool bShowedNullWarning = false;
		int fileFlavor = 0;		// 0 = unknown, '\n' = mac, '\r' = unix, anything else = ???
		
		char line[ 256 ];
		char * p = line - 1;
		
		while ( noErr == result )
			{
			// read a line: snagged from Macros_cl.cp
			int ch = std::fgetc( stream );
			
			// end of file?
			if ( EOF == ch )
				{
				*++p = 0;
				if ( p != line )
					result = ParseLine( line, flags, &dirty );
				break;
				}
			else
			if ( 0 == ch )
				{
				// how'd a null get in here? squawk, and ignore it.
				if ( not bShowedNullWarning )
					{
					SafeString msg;
					msg.Clear();
					msg.Format( "File '%s' contains a null character: I'm going to ignore it, "
								" but you should fix it.", spec.GetFileName() );
					ShowInfoText( msg.Get() );
					bShowedNullWarning = true;
					}
				}
			else
			// end of line?
			if ( '\r' == ch
			||	 '\n' == ch )
				{
				if ( 0 == fileFlavor )
					fileFlavor = ch;
				else
				if ( ch != fileFlavor )
					{
					// try to do something not totally stupid with DOS files...
					// namely, if we're at the start of a line and see a 'foreign'
					// terminator, just ignore it: that should keep the line count right.
					// (we ignore the fact that strictly speaking DOS endings are 0d0a
					// not 0a0d -- or is it the other way?. Oh well. Who cares.)
					if ( p == line - 1 )
						continue;
					}
				
				// parse what we have, and prepare for next line
				*++p = 0;
				p = line - 1;
				result = ParseLine( line, flags, &dirty );
				}
			else
				{
				// anything else, save in buffer. Ignore overflows.
				if ( p < line + sizeof line - 1 )
					*++p = ch;
				}
			}
		
		std::fclose( stream );
		SetDirty( dirty | dirty_ );
		}
	else
		{
		// no stream, ergo no file
		return fnfErr;
		}
	
	return noErr;
}


/*
** FriendsArray::ParseLine()		[private]
**
**	parse one line from a friends file
*/
DTSError
FriendArray::ParseLine( char * line, uint flags, uint * dirty )
{
	// zot any trailing newlines
	char * p = line + std::strlen( line );
	while ( *--p == '\n' )
		*p = '\0';
	
	// buffer now holds <name> '\t' <label> '\0'
	// locate the '\t'
	char * labelStr = std::strchr( line, '\t' );
	uint label;
	if ( not labelStr )
		{
		// didn't find a \t; might be importing an earlier version of the file
		if ( flags & kFriendArrayBlockedMask )
			label = kFriendBlock;
		else
		if (flags & kFriendArrayIgnoredMask )
			label = kFriendIgnore;
		else
			label = kDefaultFriendLabel;
		
		// mark new destination file as dirty so conversion happens
		*dirty |= flags;
		}
	else
		{
		// split the buffer; interpret the label
		*labelStr = '\0';
		label = FriendFlagNumberFromName( labelStr + 1, true, false );
		if ( (int) label < 0 )
			label = 0;
		}
	label = (label << kFriendArrayLabelShift) & kFriendArrayLabelMask;
	
	// strip off old flags in conversion
	flags &= (kFriendArrayLocalMask | kFriendArrayGlobalMask);
	
	// glom this new entry
	DTSError result = Add( line, label | flags );
	
	return result;
}


/*
** FriendsArray::WriteArrayToStream()		[private]
**
**	write each name onto the stream, followed by its friend-label name
*/
DTSError
FriendArray::WriteArrayToStream( std::FILE * stream, uint flagMask )
{
	// chase down each link head
	for ( int i = 0; i < kMaxFriendLinks; ++i )
		{
		for ( const FriendLink * link = linkHeads_[i]; link; link = link->GetNext() )
			{
			// does this entry match the flags I wanted?
			uint flags = link->GetFlags();
			if ( flagMask == (flagMask & flags) )
				{
				// print <name> '\t' <label> '\n'
				uint label = (flags & kFriendArrayLabelMask) >> kFriendArrayLabelShift;
				std::fprintf( stream, "%s\t%s\n", link->GetName(), sFriendLabelNames[ label ] );
				}
			}
		}
	return noErr;
}


/*
** FriendsArray::WriteFile()
**
**	write all names that match the mask, into the given filename
*/
DTSError
FriendArray::WriteFile( const char * playerName, uint flagMask /* =0 */ )
{
	if ( not (dirty_ & flagMask) )
		return noErr;
	
	// first find the folder
	DTSFileSpec spec, savedDir;
	savedDir.GetCurDir();
	
	DTSError result = LocateFriendFolder( &spec );
	
	// logic changed by timmer
	// old logic had problems when LocateFriendFolder failed
	
	// install the filename
	if ( noErr == result )
		{
			{
			// whitewash illegal characters -- see ReadFile()
			char buff[ 256 ];
			StringCopySafe( buff, playerName, sizeof buff );
			
			// make sure filename has no ':' or '/' characters
			for ( char * p = buff; *p != '\0'; ++p )
				{
				if ( *p == ':' || *p == '/' )
					*p = '-';
				}
			
			spec.SetFileName( buff );
			}
		
		// open a stream on the file
		std::FILE * stream = spec.fopen( "w" );
		
		// write out the names
		if ( stream )
			{
			result = WriteArrayToStream( stream, flagMask );
			SetDirty( dirty_ & ( ~ flagMask ) );
			std::fclose( stream );
			}
		else
			result = -1;
		}
	
	savedDir.SetDirNoPath();
	return result;
}


/*
**	SetPermFriend()
**
**	interface to class
*/
DTSError
SetPermFriend( const char * n, int friendType )
{
	if ( not gFriendArray )
		return noErr;
	
	uint flag = kFriendNone;
	switch ( friendType )
		{
		case kFriendLabel1:
		case kFriendLabel2:
		case kFriendLabel3:
		case kFriendLabel4:
		case kFriendLabel5:
		case kFriendBlock:
		case kFriendIgnore:
			flag = (friendType << FriendArray::kFriendArrayLabelShift)
				& FriendArray::kFriendArrayLabelMask;
			flag |= FriendArray::kFriendArrayLocalMask;
			break;
		
		case kFriendNone:
			// if there's a different global label for this person, 
			// keep a local "none" node to override it
			if ( gFriendArray->Contains( n, &flag, FriendArray::kFriendArrayGlobalMask ) )
				{
				flag = (friendType << FriendArray::kFriendArrayLabelShift)
					& FriendArray::kFriendArrayLabelMask;
				flag |= FriendArray::kFriendArrayLocalMask;
				}
			break;
		
		default:
			break;
		}
	
	if ( kFriendNone == flag )
		return gFriendArray->Remove( n );
	else
		return gFriendArray->Add( n, flag );
}


/*
** DeleteFile()
**
**	delete friends files no longer needed
*/
DTSError
FriendArray::DeleteFile( const char * filename ) const
{
	// first find the folder
	DTSFileSpec DTSspec, savedDir;
	savedDir.GetCurDir();
	
	DTSError result = LocateFriendFolder( &DTSspec );
	if ( noErr == result )
		{
		// now delete the file
		DTSspec.SetFileName( filename );
		result = DTSspec.Delete();
		}
	
	savedDir.SetDirNoPath();
	return result;
}


/*
** ReadFriends()
**
**	Interface to class function
*/
DTSError
ReadFriends( const char * myName )
{
	// first read the global names, then char-specific, then blocks & ignores
	if ( gFriendArray )
		{
		gFriendArray->Clear();
		gFriendArray->ReadFile( kGlobalFriendFile,	 FriendArray::kFriendArrayGlobalMask );
		// read this second, so it overrides global status
		gFriendArray->ReadFile( myName,				 FriendArray::kFriendArrayLocalMask );
		
		// v252: left in for conversions
		DTSError blockedErr = gFriendArray->ReadFile( kBlockedPlayersFile,
			FriendArray::kFriendArrayBlockedMask | FriendArray::kFriendArrayGlobalMask );
		DTSError ignoredErr = gFriendArray->ReadFile( kIgnoredPlayersFile,
			FriendArray::kFriendArrayIgnoredMask | FriendArray::kFriendArrayGlobalMask );
		
		// delete the now-obsolete block/ignore files if present
		if ( fnfErr != blockedErr )
			gFriendArray->DeleteFile( kBlockedPlayersFile );
		if ( fnfErr != ignoredErr )
			gFriendArray->DeleteFile( kIgnoredPlayersFile );
		}
	return noErr;
}


/*
** WriteFriends()
**
**	interface to class
*/
DTSError
WriteFriends( const char * myName )
{
	if ( gFriendArray )
		{
		gFriendArray->WriteFile( kGlobalFriendFile,	  FriendArray::kFriendArrayGlobalMask );
		gFriendArray->WriteFile( myName,			  FriendArray::kFriendArrayLocalMask );
//		gFriendArray->WriteFile( kBlockedPlayersFile, FriendArray::kFlagBlocked );
//		gFriendArray->WriteFile( kIgnoredPlayersFile, FriendArray::kFlagIgnored );
		}
	return noErr;
}


/*
**	GetPermFriend()
**
**	interface to class
*/
int
GetPermFriend( const char * n )
{
	uint flags = 0;
	int friends = kFriendNone;
	if ( gFriendArray && gFriendArray->Contains( n, &flags ) )
		{
		friends = (flags
			& FriendArray::kFriendArrayLabelMask) >> FriendArray::kFriendArrayLabelShift;
		}

	return friends;
}


/*
** InitFriends()
**
**	interface to class
*/
DTSError
InitFriends()
{
	gFriendArray = NEW_TAG("FriendArray") FriendArray;
	if ( not gFriendArray )
		return memFullErr;
	
	return gFriendArray->Init();
}


