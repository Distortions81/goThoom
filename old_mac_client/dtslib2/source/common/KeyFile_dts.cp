/*
**	KeyFile_dts.cp		dtslib2
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

#include "KeyFile_dts.h"
#include "Local_dts.h"
#include "Memory_dts.h"
#include "Platform_dts.h"
#include "Shell_dts.h"


// turn off the stupidity checks
#ifdef DEBUG_VERSION
// # define DEBUG_VERSION_KEYFILES	1
#endif

// when DEBUG_VERSION_KEYFILES is on, we emit lots of messages to this file
#if DEBUG_VERSION_KEYFILES
# define DEBUG_LOG_FILE		"keyfile.log"
#endif  // DEBUG_VERSION_KEYFILES


/*
**	Entry Routines and Classes
**
**	DTSKeyFile
*/


/*
**	Definitions
*/
const int	kKeyVersion			= 2;		// DTSKeyHeader::keyVersion2
// versions 0 and 1 are obsolete. V0 had 16-bit IDs. V1 had 32-bit IDs, but the
// KeyEntry tables weren't kept sorted at all times.

const uint	kMinInitEntries		= 8;	// min # entries in newly-made keyfiles
const int	kTableBumpSize		= 10;	// increment when growing the entry table


	// in-RAM (and on-disk) information about a single record
struct DTSKeyEntry
{
	int32_t		keyPosition;		// position of record in file. TODO: off_t? fpos_t?
	int32_t		keySize;			// size of record
	int32_t		keyType;			// type of record
	int32_t		keyID;				// ID of record
};

	// Doubly-linked list of KeyEntries, in RAM
struct DTSKeyEntryList
{
	DTSKeyEntry			keyEntry;
	DTSKeyEntryList *	keyPrev;
	DTSKeyEntryList *	keyNext;
};

	// File-header data (the in-RAM version;
	// what's on disk is different, and poorly aligned)
struct DTSKeyHeader
{
	int32_t		keyVersion;			// 0 = 16-bit IDs, -1= 32-bit IDs
	int32_t		keyCount;			// number of records in file
	int32_t		keyNotUsed;			// no longer used
	int32_t		keyVersion2;		// version number >0; currently kKeyVersion
};

	// if we ever _do_ invoke a compulsory keyfile upgrade, we might consider
	// skipping forward to something like this, which is also fully aligned
	// (unlike v0 and v1), adds a "magic" identification word, eliminates the
	// previously wasted space, and is slightly more future-proof all around.

struct DTSKeyHeader_v3
{
	int32_t		keySignature;		// magic == '\306Key' (MacRoman delta, opt+j), e.g.
	int32_t		keyVersion;			// version number == 3
	int32_t		keyRevision;		// (compatible) revision number == 0
	int32_t		keyCount;			// number of records in file
};
	
# pragma pack( push, 2)

	// OBSOLETE: for v0 keyfiles
struct DTSKeyEntryOld
{
	int32_t		keyPosition;		// position of record in file
	int32_t		keySize;			// size of record
	int32_t		keyType;			// type of record
	int16_t		keyID;				// id of record
};

	// OBSOLETE: for v0 keyfiles
struct DTSKeyHeaderOld
{
	int16_t		keyVersion;
	int32_t		keyCount;
	int32_t		keyLimit;
};

	// Despite the name, this is what actually exists on everybody's disks right now.
	// We never did get around to enforcing worldwide upgrades to "true" v2 headers.
struct DTSKeyHeaderOld2
{
	int16_t		keyVersion;			// 0 = 16-bit ids, -1 = 32-bit ids
	int32_t		keyCount;			// number of records in file
	int32_t		keyNotUsed;			// no longer used
	int16_t		keyVersion2;		// version number >0
};

# pragma pack( pop )

	// the master keyfile controller object
class DTSKeyFilePriv
{
private:
	// no copying
				DTSKeyFilePriv( const DTSKeyFilePriv& );
	DTSKeyFilePriv&	operator =( const DTSKeyFilePriv& );
	
public:
	DTSKeyHeader		keyHeader;			// header
	DTSKeyEntryList *	keyEntry;			// record table
	DTSKeyEntryList *	keyFirst;			// first entry in file
	int					keyRefNum;			// file reference number
	int					keyWriteMode;		// header write mode
	long				keyMax;				// max that will fit in entry list
	bool				keyWritePerm;		// true if has write permission
	bool				keyHdrDirty;		// true if the header is dirty
	
	// constructor/destructor
				DTSKeyFilePriv();
				~DTSKeyFilePriv();
	
	// interface
	DTSError	Open( DTSFileSpec * spec, uint flags, int fileTypeID, uint initNumEntries );
	void		Close();
	DTSError	Exists( DTSKeyType ttype, DTSKeyID id ) const;
	DTSError	ReadAlloc( DTSKeyType ttype, DTSKeyID id, void *& oBuffer );
	DTSError	GetSize( DTSKeyType ttype, DTSKeyID id, size_t * oSize ) const;
	DTSError	Read( DTSKeyType ttype, DTSKeyID id, void * buffer, size_t bufsz );
	DTSError	Write( DTSKeyType ttype, DTSKeyID id, const void * buffer, size_t size );
	DTSError	Delete( DTSKeyType ttype, DTSKeyID id );
	DTSError	Compress();
	DTSError	Count( DTSKeyType ttype, long * oCount ) const;
	DTSError	GetID( DTSKeyType ttype, long indx, DTSKeyID * oID ) const;
	DTSError	GetIndex( DTSKeyType ttype, DTSKeyID id, long * oIindex ) const;
	DTSError	GetUnusedID( DTSKeyType ttype, DTSKeyID * oID ) const;
	DTSError	GetInfo( DTSKeyInfo * oInfo ) const;
	DTSError	CountTypes( long * oCount ) const;
	DTSError	GetType( long indx, DTSKeyType * oType ) const;
	int			SetWriteMode( int mode );
	int			GetWriteMode() const { return keyWriteMode; }
	
	// private interface
private:
	void		InitFields();
	DTSError	Create( DTSFileSpec * spec, int fileTypeID, uint initNumEntries );
	DTSError	ReadTable();
	DTSError	ReadTableVersion0();
	DTSError	ReadTableVersion1_2();
	void		SortTableNoLinks();
	void		InitPositionList();
	DTSError	WriteHeader();
	DTSError	WriteTable();
	DTSError	WriteTableEntry( DTSKeyEntryList * entry );
	DTSKeyEntryList *	FindEntry( DTSKeyType ttype, DTSKeyID id ) const;
	DTSKeyEntryList *	FindEntryFirst( DTSKeyType ttype ) const;
	DTSKeyEntryList *	FindEntryLast ( DTSKeyType ttype ) const;
	DTSKeyEntryList *	SortTableWithLinks( DTSKeyType ttype, DTSKeyID id, size_t size );
	DTSError	FindCreateEntry( DTSKeyType t, DTSKeyID id, size_t sz, DTSKeyEntryList ** entry );
	DTSError	FindBestPosition( DTSKeyEntryList * e, size_t sz, DTSKeyEntryList **, ulong * p );
	void		RemovePositionList( DTSKeyEntryList * entry );
	
	static DTSError		ReadKeyHeader( int refNum, DTSKeyHeader * header );
	static DTSError		WriteKeyHeader( int refNum, const DTSKeyHeader * header );
	
	// sanity checks
#if DEBUG_VERSION_KEYFILES
	void		CheckConsistency();
	void		CheckKeyCollision( const DTSKeyEntryList *, ulong pos, size_t size );
#endif  // DEBUG_VERSION_KEYFILES
	
	// more definitions
	//  this had better be == sizeof(DTSKeyHeaderOld)
	static const size_t kSizeOfKeyHeader0	= 1 * sizeof( int16_t ) + 2 * sizeof( int32_t );
	
	// and this had better == sizeof(DTSKeyHeaderOld2)
	static const size_t kSizeOfKeyHeader1	= 2 * sizeof( int16_t ) + 2 * sizeof( int32_t );
};


/*
**	Internal Variables
*/
#if DEBUG_VERSION_KEYFILES
static FILE * gStream;
#endif


/*
**	Internal Routines
*/
#if DEBUG_VERSION_KEYFILES
static void		LogToStream( const char * format, ... ) PRINTF_LIKE( 1, 2 );
#endif  // DEBUG_VERSION_KEYFILES


/*
**	DTSKeyFilePriv
*/
DTSDefineImplementFirmNoCopy(DTSKeyFilePriv)


/*
**	DTSKeyFilePriv::DTSKeyFilePriv()
*/
DTSKeyFilePriv::DTSKeyFilePriv() :
	keyEntry(),								// table not loaded
	keyFirst(),
	keyRefNum( -1 ),						// file not open
	keyWriteMode( kWriteModeReliable ),		// assume reliable writing
	keyMax(),								// no entries in table
	keyWritePerm(),
	keyHdrDirty()							// header not dirty
{
	// initialize the fields
	InitFields();
}


/*
**	DTSKeyFilePriv::~DTSKeyFilePriv()
*/
DTSKeyFilePriv::~DTSKeyFilePriv()
{
	// close the key file
	Close();
}


/*
**	DTSKeyFile::Open()
**
**	open the file by name
*/
DTSError
DTSKeyFile::Open( const char * name, uint flags, int typeID /* =0 */, uint numEntries /* =0 */ )
{
	DTSFileSpec spec;
	spec.SetFileName( name );
	
	DTSError result = Open( &spec, flags, typeID, numEntries );
	
	return result;
}


/*
**	DTSKeyFile::Open()
**
**	open the file by DTSFileSpec
*/
DTSError
DTSKeyFile::Open( DTSFileSpec * spec, uint flags, int typeID /* =0 */, uint numEntries /* =0 */ )
{
	DTSKeyFilePriv * p = priv.p;
	return p ? p->Open( spec, flags, typeID, numEntries ) : -1;
}


/*
**	DTSKeyFilePriv::Open()
**
**	open the file by DTSFileSpec
*/
DTSError
DTSKeyFilePriv::Open( DTSFileSpec * spec, uint flags, int fileTypeID, uint initNumEntries )
{
	// write permission
	bool bWritePerm = (flags & kKeyReadWritePerm) != 0;
	keyWritePerm = bWritePerm;
	
	DTSError result = DTS_open( spec, bWritePerm, &keyRefNum );
	if ( fnfErr == result )
		{
		// hm, not there. Maybe they want us to create it.
		if ( not (flags & kKeyDontCreateFile) )
			result = Create( spec, fileTypeID, initNumEntries );
		}
	if ( noErr == result )
		{
		if ( -1 == keyRefNum )
			result = DTS_open( spec, bWritePerm, &keyRefNum );
		}
	
	// read the primary header
	if ( noErr == result )
		result = DTS_seek( keyRefNum, 0 );
	if ( noErr == result )
		result = ReadKeyHeader( keyRefNum, &keyHeader );
	if ( noErr == result )
		{
		if ( -1 == keyHeader.keyVersion
		&&   keyHeader.keyVersion2 > kKeyVersion )
			{
			result = -1;	// on-disk data is too new for this code
			}
		}
	
	// read the key-entry table
	if ( noErr == result )
		result = ReadTable();
	
	// if failing, do so with a modicum of grace & panache
	if ( noErr != result )
		Close();
	
#if DEBUG_VERSION_KEYFILES
	LogToStream( "%p open '%s' result %d\n", this, spec->GetFileName(),
		static_cast<int>( result ) );
#endif
	
	return result;
}


/*
**	DTSKeyFile::Close()
**
**	close the file
**	dispose of all allocated memories
*/
void
DTSKeyFile::Close()
{
	if ( DTSKeyFilePriv * p = priv.p )
		p->Close();
}


/*
**	DTSKeyFilePriv::Close()
**
**	close the file
**	dispose of all allocated memories
*/
void
DTSKeyFilePriv::Close()
{
	// not already closed?
	if ( keyRefNum != -1 )
		{
		// ensure the header & entry table are flushed
		SetWriteMode( kWriteModeReliable );
		
		// close the file
		DTS_close( keyRefNum );
		
		// free memory
		delete[] reinterpret_cast<char *>( keyEntry );
		
		// re-initialize the fields in case someone opens the file again
		InitFields();
		
#if DEBUG_VERSION_KEYFILES
		LogToStream( "%p close\n", this );
#endif
	}
}


/*
**	DTSKeyFilePriv::InitFields()
**
**	initialize the fields to safe values
*/
void
DTSKeyFilePriv::InitFields()
{
	keyHeader.keyVersion2 = -1;					// header not yet read
	keyEntry              = nullptr;			// table not loaded
	keyRefNum             = -1;					// file not open
	keyWriteMode          = kWriteModeReliable;	// assume reliable writing
	keyMax                = 0;					// no entries in table
	keyHdrDirty           = false;				// header not dirty
}


/*
**	DTSKeyFilePriv::ReadTable()
**
**	read and initialize the entry table
**	
**	TODO: stop supporting v0 & v1 tables; those haven't existed in nature since about 1997.
** (well, that's not necessarily strictly true. As the formats have evolved, any keyfile-based
** app that opens old-style files _in R/W mode_ would have silently updated them, as a side
** effect.  But there might still be a few "pockets of resistance", in the form of files
** that have ever only been opened in RO mode, and thus remain in v0 or v1 format...)
**
**	TODO2: also maybe _start_ supporting proposed v3 tables, with decent alignment?
*/
DTSError
DTSKeyFilePriv::ReadTable()
{
	// what version of on-disk file are we dealing with?
	DTSError result;
	int version = keyHeader.keyVersion;
	if ( -1 == version )
		version = keyHeader.keyVersion2;
	switch ( version )
		{
		case 0:
			result = ReadTableVersion0();
			break;
		
		case 1:
		case 2:
			result = ReadTableVersion1_2();
			break;
		
		default:
			result = -1;
			break;
		}
	
	// build the linked list of record positions
	if ( noErr == result )
		InitPositionList();
	
	return result;
}


/*
**	DTSKeyFilePriv::ReadTableVersion0()
**
**	read the old-style entry table
*/
DTSError
DTSKeyFilePriv::ReadTableVersion0()
{
	DTSKeyEntryOld  * oldEntry = nullptr;
	DTSKeyEntryList * newEntry = nullptr;
	
	int count       = keyHeader.keyCount;
	size_t oldSize  = uint( count ) * kSizeOfKeyHeader0;
	size_t newSize  = uint( count ) * sizeof( DTSKeyEntryList );
	size_t newLimit = kSizeOfKeyHeader1 + newSize;
	
	// allocate memory for the old entries
	DTSError result = noErr;
	//if ( noErr == result )
		{
		oldEntry = reinterpret_cast<DTSKeyEntryOld *>( NEW_TAG("DTSKeyEntryTableV0")
						char[ oldSize ] );
		if ( not oldEntry )
			result = memFullErr;
		}
	// allocate memory for the new entries
	if ( noErr == result )
		{
		newEntry = reinterpret_cast<DTSKeyEntryList *>( NEW_TAG("DTSKeyEntryTable")
						char[ newSize ] );
		if ( not newEntry )
			result = memFullErr;
		}
	// read in the old entries
	if ( noErr == result )
		result = DTS_seek( keyRefNum, kSizeOfKeyHeader0 );
	if ( noErr == result )
		result = DTS_read( keyRefNum, oldEntry, oldSize );
	
	// make sure the end of the file is not less than the new limit
	if ( keyWritePerm )
		{
		ulong eof = 0;
		if ( noErr == result )
			result = DTS_geteof( keyRefNum, &eof );
		if ( noErr == result )
			{
			if ( eof < newLimit )
				result = DTS_seteof( keyRefNum, newLimit );
			}
		}
	
	// convert the old entries into new ones
	if ( noErr == result )
		{
		DTSKeyEntryOld  * oe = oldEntry;
		DTSKeyEntryList * ne = newEntry;
		for ( int ix = keyHeader.keyCount;  ix > 0;  --ix, ++ne, ++oe )
			{
			ne->keyEntry.keyPosition = BigToNativeEndian( oe->keyPosition );
			ne->keyEntry.keySize     = BigToNativeEndian( oe->keySize );
			ne->keyEntry.keyType     = BigToNativeEndian( oe->keyType );
			ne->keyEntry.keyID       = (DTSKeyID) BigToNativeEndian( oe->keyID );
			}
		
		// write the new header and table
		keyEntry              = newEntry;
		keyMax                = count;
		keyHeader.keyVersion  = -1;
		keyHeader.keyNotUsed  = -1;
		keyHeader.keyVersion2 = kKeyVersion;
		
		SortTableNoLinks();
		}
	
	// recover in case of error
	if ( result != noErr )
		delete[] reinterpret_cast<char *>( newEntry );
	delete[] reinterpret_cast<char *>( oldEntry );
	
	return result;
}


/*
**	DTSKeyFilePriv::SortTableNoLinks()
**
**	sort the key entry table -- used only when converting v0 or v1 files to v2
*/
void
DTSKeyFilePriv::SortTableNoLinks()
{
	DTSKeyEntryList * tableStart = keyEntry;
	long count = keyHeader.keyCount;
	for ( long startIndex = 0; startIndex < count;  ++startIndex )
		{
		DTSKeyEntryList * testEntry = tableStart + startIndex;
		
		// save the entry to be inserted
		DTSKeyEntryList tempEntry = *testEntry;
		
		// slide the last entries down to make room for the new entry
		DTSKeyType sortType = tempEntry.keyEntry.keyType;
		DTSKeyID sortID = tempEntry.keyEntry.keyID;
		for ( ;  testEntry > tableStart;  --testEntry )
			{
			DTSKeyType testType = testEntry[-1].keyEntry.keyType;
			if ( testType < sortType )
				break;
			
			if ( testType == sortType
			&&   testEntry[-1].keyEntry.keyID < sortID )
				{
				break;
				}
			testEntry[0] = testEntry[-1];
			}
		
		// put the new entry into the correct place
		*testEntry = tempEntry;
		}
}


/*
**	DTSKeyFilePriv::ReadTableVersion1_2()
**
**	read the new-style entry table
*/
DTSError
DTSKeyFilePriv::ReadTableVersion1_2()
{
	// the size of the entry table in RAM is larger than
	// the size of the entry table stored on disk
	int count = keyHeader.keyCount;
	size_t filesize = uint( count ) * sizeof(DTSKeyEntry);
	size_t ramsize  = uint( count ) * sizeof(DTSKeyEntryList);
	
	// allocate memory for the RAM table
	DTSError result = noErr;
	DTSKeyEntryList * entrytable = reinterpret_cast<DTSKeyEntryList *>(
			NEW_TAG("KeyTable1_2") char[ ramsize ] );
	if ( not entrytable )
		result = memFullErr;
	
	// read the file table into the RAM table
	if ( noErr == result )
		result = DTS_seek( keyRefNum, kSizeOfKeyHeader1 );
	if ( noErr == result )
		result = DTS_read( keyRefNum, entrytable, filesize );
	
	// convert the file table into a RAM table
	// TODO: a debug test for whether a v2 table really IS properly sorted
	// (pretty easy to verify while we're scanning every entry)
	// TODO: this could be somewhat simpler if we didn't convert the table in place.
	// Instead, read the file data into a secondary, temp buffer; then unpack into
	// the RAM table, without having to worry about overlap.
	if ( noErr == result )
		{
		// save fields
		keyEntry = entrytable;
		keyMax   = count;
		
		// copy entries starting with the last
		if ( count > 1 )
			{
			// point at last record
			--count;
			DTSKeyEntry * fileentry = reinterpret_cast<DTSKeyEntry *>( entrytable ) + count;
			entrytable += count;
			
			// copy records N-1 down to record 2 (zero-based)
			for ( --count;  count > 0;  --count, --entrytable, --fileentry )
				{
#if DTS_BIG_ENDIAN
				entrytable->keyEntry = *fileentry;
#else
				entrytable->keyEntry.keyPosition = BigToNativeEndian( fileentry->keyPosition );
				entrytable->keyEntry.keySize     = BigToNativeEndian( fileentry->keySize     );
				entrytable->keyEntry.keyType     = BigToNativeEndian( fileentry->keyType     );
				entrytable->keyEntry.keyID       = BigToNativeEndian( fileentry->keyID       );
#endif
				}
			
			// copy record 1
			// handled specially because they overlap
			DTSKeyEntry temp = *fileentry;
//			*fileentry       = entrytable->keyEntry;	// ?? why ??
//	20080123 rer I hated that line so much, I finally removed it altogether
			
#if DTS_BIG_ENDIAN
			entrytable->keyEntry = temp;
#else
			entrytable->keyEntry.keyPosition = BigToNativeEndian( temp.keyPosition );
			entrytable->keyEntry.keySize     = BigToNativeEndian( temp.keySize     );
			entrytable->keyEntry.keyType     = BigToNativeEndian( temp.keyType     );
			entrytable->keyEntry.keyID       = BigToNativeEndian( temp.keyID       );
#endif
			
			// don't need to copy record 0; it's already in the right place
			// ... but not necessarily in the right endianness
#if DTS_LITTLE_ENDIAN
			--entrytable;
			--fileentry;
			
			entrytable->keyEntry.keyPosition = BigToNativeEndian( fileentry->keyPosition );
			entrytable->keyEntry.keySize     = BigToNativeEndian( fileentry->keySize     );
			entrytable->keyEntry.keyType     = BigToNativeEndian( fileentry->keyType     );
			entrytable->keyEntry.keyID       = BigToNativeEndian( fileentry->keyID       );
#endif  // DTS_LITTLE_ENDIAN			
			}
#if DTS_LITTLE_ENDIAN
			// even if there's only one record, it must still be swapped
		else
		if ( 1 == count )
			{
			DTSKeyEntry& entry = entrytable->keyEntry;
			entry.keyPosition = BigToNativeEndian( entry.keyPosition );
			entry.keySize     = BigToNativeEndian( entry.keySize     );
			entry.keyType     = BigToNativeEndian( entry.keyType     );
			entry.keyID       = BigToNativeEndian( entry.keyID       );
			}
#endif  // DTS_LITTLE_ENDIAN
		}
	
	// sort the table if we need to
	if ( noErr == result )
		{
		if ( 1 == keyHeader.keyVersion2 )
			{
			// upgrade to v2
			keyHeader.keyVersion2 = 2;
			SortTableNoLinks();
			}
		}
	
	// recover from an error
	if ( noErr != result )
		delete[] reinterpret_cast<char *>( entrytable );
	
	return result;
}


/*
**	DTSKeyFilePriv::InitPositionList()
**
**	build a linked list of records in order by position
*/
void
DTSKeyFilePriv::InitPositionList()
{
	// special case zero records
	int count = keyHeader.keyCount;
	if ( count <= 0 )
		{
		keyFirst = nullptr;
		return;
		}
	
	DTSKeyEntryList * entry = keyEntry;
	entry->keyPrev = nullptr;
	entry->keyNext = nullptr;
	
	// special case one record
	if ( 1 == count )
		{
		keyFirst = entry;
		return;
		}
	
	// start with the first record
	DTSKeyEntryList * beg = entry;
	DTSKeyEntryList * end = entry;
	
	// add the rest of the records
	++entry;
	--count;
	for ( ;  count > 0;  --count, ++entry )
		{
		// check if the new record goes after the one currently last in the list
		int position = entry->keyEntry.keyPosition;
		if ( position >= end->keyEntry.keyPosition )
			{
			entry->keyNext = nullptr;
			entry->keyPrev = end;
			end->keyNext   = entry;
			end            = entry;
			}
		// check if the new record goes before the one currently first in the list
		else
		if ( position <= beg->keyEntry.keyPosition )
			{
			entry->keyPrev = nullptr;
			entry->keyNext = beg;
			beg->keyPrev   = entry;
			beg            = entry;
			}
		// walk the list
		else
			{
			// we know there are at least two entries already in the table
			// if there were only one, then both beg and end point to it
			// and one of the above conditions must be true
			// and we would have never gotten here
			// so it's safe to use entry[-1]
			
			// the most likely position for this record is right after the previous one
			// test if we should start there
			DTSKeyEntryList * test;
			if ( position >= entry[-1].keyEntry.keyPosition )
				test = entry[-1].keyNext;
			else
				test = beg;
			
			// walk through the list looking for a record after us
			// we know we'll find one eventually
			// otherwise the first test in the count loop would have been true
			// and we would have never gotten here
			for( ;  ;  test = test->keyNext )
				{
				assert( test );
				// when we find that record...
				if ( position <= test->keyEntry.keyPosition )
					{
					// insert ourselves before it
					DTSKeyEntryList * prev = test->keyPrev;
					assert( prev );
					
					entry->keyNext = test;
					entry->keyPrev = prev;
					prev->keyNext  = entry;
					test->keyPrev  = entry;
					break;
					}
				}
			}
		}
	
	// remember the first
	keyFirst = beg;
	
#if DEBUG_VERSION_KEYFILES
	CheckConsistency();
#endif
}


/*
**	DTSKeyFilePriv::Create()
**
**	create the file but don't open it
*/
DTSError
DTSKeyFilePriv::Create( DTSFileSpec * spec, int fileTypeID, uint initNumEntries )
{
	int refNum = -1;
	
	DTSError result;
	// if ( noErr == result )
		{
#if defined( DTS_Mac ) || defined( DTS_XCODE )
		OSType fileType = 0xC64B6579;			// delta, 'Key'
		GetFileType( fileTypeID, &fileType );
#else
		(void) fileTypeID;
		ulong fileType = 0;
#endif
		result = DTS_create( spec, fileType );
		}
	if ( noErr == result )
		result = DTS_open( spec, true, &refNum );		// write permission
	
	// grow the file to the minimum necessary size
	if ( noErr == result )
		{
		if ( initNumEntries < kMinInitEntries )
			initNumEntries = kMinInitEntries;
		
		size_t limit = kSizeOfKeyHeader1 + initNumEntries * sizeof(DTSKeyEntry);
		result = DTS_seteof( refNum, limit );
		}
	
	// initialize & write the primary header
	if ( noErr == result )
		result = DTS_seek( refNum, 0 );
	if ( noErr == result )
		{
		keyHeader.keyVersion  = -1;				// 32-bit IDs
		keyHeader.keyCount    = 0;				// no records yet
		keyHeader.keyNotUsed  = -1;				// unused
		keyHeader.keyVersion2 = kKeyVersion;	// current file version
		
		result = WriteKeyHeader( refNum, &keyHeader );
		}
	
	// recover from errors
	if ( refNum != -1 )
		DTS_close( refNum );
	if ( dupFNErr == result )
		result = noErr;
	
	return result;
}


/*
**	DTSKeyFilePriv::WriteHeader()
*/
DTSError
DTSKeyFilePriv::WriteHeader()
{
	// paranoid checks
	if ( -1 == keyRefNum				// no disk file opened?
	||   keyHeader.keyVersion2 < 0		// absurd version #?
	||   not keyWritePerm				// not writable?
	||   not keyEntry )					// no entry table?
		{
		return -1;
		}
	
	// check for faster writing
	if ( kWriteModeFaster == keyWriteMode )
		{
		keyHdrDirty = true;				// just remember that we're dirty
		return noErr;
		}
	
	DTSError result = DTS_seek( keyRefNum, 0 );
	if ( noErr == result )
		result = WriteKeyHeader( keyRefNum, &keyHeader );
	if ( noErr == result )
		keyHdrDirty = false;			// clean now!
	
	return result;
}


/*
**	DTSKeyFilePriv::WriteTable()
*/
DTSError
DTSKeyFilePriv::WriteTable()
{
	// paranoid checks
	if ( -1 == keyRefNum
	||   keyHeader.keyVersion2 < 0
	||   not keyWritePerm
	||   not keyEntry )
		{
		return -1;
		}
	
	// check for faster writing
	if ( kWriteModeFaster == keyWriteMode )
		{
		keyHdrDirty = true;		// just mark that we're dirty
		return noErr;
		}
	
#if DEBUG_VERSION_KEYFILES
	LogToStream( "%p begin WriteTable count %d\n", this, static_cast<int>( keyHeader.keyCount ) );
#endif
	
	int count = keyHeader.keyCount;
	size_t size = uint( count ) * sizeof(DTSKeyEntry);
	DTSKeyEntry * filetable = nullptr;
	
	DTSError result = noErr;
	
	// relocate any existing on-disk entries that overlap with the table
	// (the table might have grown since last written out)
	if ( DTSKeyEntryList * first = keyFirst )
		{
		size_t limit = kSizeOfKeyHeader1 + size;
		for(;;)
			{
			// stop when we're out of the danger zone
			if ( first->keyEntry.keyPosition >= long(limit) )
				break;
			
			// make a temp copy of the overlapped data
			void * temp = nullptr;
			DTSKeyType ttype = first->keyEntry.keyType;
			DTSKeyID id      = first->keyEntry.keyID;
			result = ReadAlloc( ttype, id, temp );
			if ( noErr == result )
				{
				// temporarily switch to the faster write mode
				// we're going to write out the whole table real soon
				// we don't want this Write() to write the table
				// because it might clobber some records
				keyWriteMode = kWriteModeFaster;
				
				result = Write( ttype, id, temp, static_cast<size_t>( first->keyEntry.keySize ) );
				
				// restore the slower safer write mode
				keyWriteMode = kWriteModeReliable;
				}
			delete[] static_cast<char *>( temp );
			if ( noErr != result )
				break;
			first = keyFirst;
			}
		}
	
	// build a new on-disk entry table
	if ( noErr == result )
		{
		filetable = reinterpret_cast<DTSKeyEntry *>( NEW_TAG("DTSKeyWriteTableTemp")
						char[ size ] );
		if ( not filetable )
			result = memFullErr;
		}
	if ( noErr == result )
		{
		DTSKeyEntryList * list = keyEntry;
		for ( DTSKeyEntry * walk = filetable;  count > 0;  --count, ++walk, ++list )
			{
#if DTS_BIG_ENDIAN
			*walk = list->keyEntry;
#else
			// the on-disk table will have network endianness
			walk->keyPosition = NativeToBigEndian( list->keyEntry.keyPosition );
			walk->keySize     = NativeToBigEndian( list->keyEntry.keySize     );
			walk->keyType     = NativeToBigEndian( list->keyEntry.keyType     );
			walk->keyID       = NativeToBigEndian( list->keyEntry.keyID       );
#endif
			}
		
		result = DTS_seek( keyRefNum, kSizeOfKeyHeader1 );
		}
	if ( noErr == result )
		result = DTS_write( keyRefNum, filetable, size );
	
	// if there were no errors, the header is now clean
	keyHdrDirty = (noErr != result);
	
	delete[] reinterpret_cast<char *>( filetable );
	
#if DEBUG_VERSION_KEYFILES
	LogToStream( "%p end WriteTable\n", this );
#endif
	
	return result;
}


/*
**	DTSKeyFilePriv::WriteTableEntry()
*/
DTSError
DTSKeyFilePriv::WriteTableEntry( DTSKeyEntryList * entry )
{
	// paranoid checks
	if ( -1 == keyRefNum
	||   keyHeader.keyVersion2 < 0
	||   not keyWritePerm
	||   not keyEntry )
		{
		return -1;
		}
	
	// check for faster writing
	if ( kWriteModeFaster == keyWriteMode )
		{
		keyHdrDirty = true;
		return noErr;
		}
	
	// check for a dirty header
	DTSError result = noErr;
	if ( keyHdrDirty )
		{
		// write the entire table, not just this one entry
		result = WriteTable();
		return result;
		}
	
#if DEBUG_VERSION_KEYFILES
	LogToStream( "%p WriteTableEntry t/i %4.4s %d p/s %.8X %.8X\n", this,
		reinterpret_cast<const char *>( &entry->keyEntry.keyType ),
		static_cast<int>( entry->keyEntry.keyID ),
		entry->keyEntry.keyPosition,
		entry->keyEntry.keySize );
#endif
	
	//if ( noErr == result )
		{
		size_t offset = static_cast<size_t>(entry - keyEntry) * sizeof(DTSKeyEntry) + kSizeOfKeyHeader1;
		result = DTS_seek( keyRefNum, offset );
		}
	if ( noErr == result )
		{
#if DTS_BIG_ENDIAN
		result = DTS_write( keyRefNum, &entry->keyEntry, sizeof entry->keyEntry );
#else
		// convert to network-endianness just before flushing to disk
		DTSKeyEntry temp;
		temp.keyPosition = NativeToBigEndian( entry->keyEntry.keyPosition );
		temp.keySize     = NativeToBigEndian( entry->keyEntry.keySize );
		temp.keyType     = NativeToBigEndian( entry->keyEntry.keyType );
		temp.keyID       = NativeToBigEndian( entry->keyEntry.keyID );
		
		result = DTS_write( keyRefNum, &temp, sizeof temp );
#endif
		}
	
	if ( noErr != result )
		keyHdrDirty = true;
	
	return result;
}


/*
**	DTSKeyFile::SetWriteMode()
**
**	set the write mode
*/
int
DTSKeyFile::SetWriteMode( int newmode )
{
	DTSKeyFilePriv * p = priv.p;
	return p ? p->SetWriteMode( newmode ) : -1;
}


/*
**	DTSKeyFilePriv::SetWriteMode()
**
**	set the write mode
*/
int
DTSKeyFilePriv::SetWriteMode( int newmode )
{
	int oldMode = keyWriteMode;
	keyWriteMode = newmode;
	
	// switching to reliable mode flushes all pending writes
	if ( kWriteModeFaster != newmode
	&&   keyHdrDirty )
		{
		WriteHeader();
		WriteTable();
		}
	
	return oldMode;
}


/*
**	DTSKeyFile::GetWriteMode()
**
**	return the write mode
*/
int
DTSKeyFile::GetWriteMode() const
{
	const DTSKeyFilePriv * p = priv.p;
	return p ? p->GetWriteMode() : -1;
}


/*
**	DTSKeyFile::CountTypes()
**
**	return the number of types in the file
*/
DTSError
DTSKeyFile::CountTypes( long * count ) const
{
	const DTSKeyFilePriv * p = priv.p;
	return p ? p->CountTypes( count ) : -1;
}


/*
**	DTSKeyFilePriv::CountTypes()
**
**	return the number of types in the file
*/
DTSError
DTSKeyFilePriv::CountTypes( long * count ) const
{
	// paranoid checks
	if ( -1 == keyRefNum
	||   not keyEntry )
		{
		return fnOpnErr;
		}
	
	// count the types, by iterating all of them
	DTSKeyType lastType = 0;
	const DTSKeyEntryList * entry = keyEntry;
	int numTypes = 0;
	for ( int ix = keyHeader.keyCount;  ix > 0;  --ix )
		{
		DTSKeyType aType = entry->keyEntry.keyType;
		if ( lastType != aType )
			{
			lastType = aType;
			++numTypes;
			}
		++entry;
		}
	
	if ( count )
		*count = numTypes;
	
	return noErr;
}


/*
**	DTSKeyFile::Count()
**
**	return the number of entries of this type
*/
DTSError
DTSKeyFile::Count( DTSKeyType ttype, long * outCount ) const
{
	const DTSKeyFilePriv * p = priv.p;
	return p ? p->Count( ttype, outCount ) : -1;
}


/*
**	DTSKeyFilePriv::Count()
**
**	return the number of entries of this type
*/
DTSError
DTSKeyFilePriv::Count( DTSKeyType ttype, long * outCount ) const
{
	long count = 0;
	
	DTSError result = noErr;
	if ( -1 == keyRefNum
	||   not keyEntry )
		{
		result = fnOpnErr;
		}
	
	if ( noErr == result )
		{
		if ( const DTSKeyEntryList * first = FindEntryFirst( ttype ) )
			{
			const DTSKeyEntryList * last = FindEntryLast( ttype );
			count = ( last - first ) + 1;	// oh, the magic of pointer math!
			}
		}
	
	if ( outCount )
		*outCount = count;
	
	return result;
}


/*
**	DTSKeyFilePriv::FindEntryFirst()
**
**	return a pointer to the first entry of this type
*/
DTSKeyEntryList *
DTSKeyFilePriv::FindEntryFirst( DTSKeyType ttype ) const
{
	long step = keyHeader.keyCount - 1;
	if ( step < 0 )
		return nullptr;
	
	const DTSKeyEntryList * lentry = keyEntry;
	
	// we might be lucky; it could be the very first type
	if ( lentry->keyEntry.keyType == ttype )
		return const_cast<DTSKeyEntryList *>( lentry );
	
	// not so lucky: do a binary search
	const DTSKeyEntryList * rentry = lentry + step;
	for(;;)
		{
		if ( step <= 1 )
			{
			if ( lentry->keyEntry.keyType == ttype )
				return const_cast<DTSKeyEntryList *>( lentry );
			
			if ( 1 == step
			&&   rentry->keyEntry.keyType == ttype )
				{
				return const_cast<DTSKeyEntryList *>( rentry );
				}
			return nullptr;
			}
		long lstep = step >> 1;
		long rstep = step - lstep;
		const DTSKeyEntryList * mentry = lentry + lstep;
		if ( mentry->keyEntry.keyType < ttype )
			{
			lentry = mentry;
			step   = rstep;
			}
		else
			{
			rentry = mentry;
			step   = lstep;
			}
		}
}


/*
**	DTSKeyFilePriv::FindEntryLast()
**
**	return a pointer to the last entry of this type
*/
DTSKeyEntryList *
DTSKeyFilePriv::FindEntryLast( DTSKeyType ttype ) const
{
	long step = keyHeader.keyCount - 1;
	if ( step < 0 )
		return nullptr;
	
	const DTSKeyEntryList * lentry = keyEntry;
	const DTSKeyEntryList * rentry = lentry + step;
	
	// we might be lucky: it could be the very last type
	if ( rentry->keyEntry.keyType == ttype )
		return const_cast<DTSKeyEntryList *>( rentry );
	
	// nope; binary search
	for(;;)
		{
		if ( step <= 1 )
			{
			if ( lentry->keyEntry.keyType == ttype )
				return const_cast<DTSKeyEntryList *>( lentry );
			
			if ( 1 == step
			&&   rentry->keyEntry.keyType == ttype )
				{
				return const_cast<DTSKeyEntryList *>( rentry );
				}
			return nullptr;
			}
		long lstep = step >> 1;
		long rstep = step - lstep;
		const DTSKeyEntryList * mentry = lentry + lstep;
		if ( mentry->keyEntry.keyType <= ttype )
			{
			lentry = mentry;
			step   = rstep;
			}
		else
			{
			rentry = mentry;
			step   = lstep;
			}
		}
}


/*
**	DTSKeyFile::GetType()
**
**	return the index'th type present in the file
*/
DTSError
DTSKeyFile::GetType( long inIndex, DTSKeyType * typeOut ) const
{
	const DTSKeyFilePriv * p = priv.p;
	return p ? p->GetType( inIndex, typeOut ) : -1;
}


/*
**	DTSKeyFilePriv::GetType()
**
**	return the indexed type
**	does linear scan of all entries until found
*/
DTSError
DTSKeyFilePriv::GetType( long inIndex, DTSKeyType * typeOut ) const
{
	// paranoid checks
	if ( -1 == keyRefNum
	||   not keyEntry )
		{
		return fnOpnErr;
		}
	
	// iterate the types
	DTSKeyType lastType = 0;
	const DTSKeyEntryList * entry = keyEntry;
	for ( int step = keyHeader.keyCount;  step > 0;  --step, ++entry )
		{
		DTSKeyType ttype = entry->keyEntry.keyType;
		if ( lastType != ttype )
			{
			lastType = ttype;
			if ( 0 == inIndex )
				{
				if ( typeOut )
					*typeOut = ttype;
				return noErr;
				}
			--inIndex;
			}
		}
	
	return -1;
}


/*
**	DTSKeyFile::Exists()
**
**	return noErr if the record exists
*/
DTSError
DTSKeyFile::Exists( DTSKeyType ttype, DTSKeyID id ) const
{
	const DTSKeyFilePriv * p = priv.p;
	return p ? p->Exists( ttype, id ) : -1;
}


/*
**	DTSKeyFilePriv::Exists()
**
**	return noErr if a record with the given type & ID exists in the file;
**	otherwise various error codes
*/
DTSError
DTSKeyFilePriv::Exists( DTSKeyType ttype, DTSKeyID id ) const
{
	// paranoid checks
	if ( -1 == keyRefNum
	||   not keyEntry )
		{
		return fnOpnErr;
		}
	
	// do we have it?
	if ( FindEntry( ttype, id ) )
		return noErr;
	
	return -1;		// not found
}


/*
**	DTSKeyFilePriv::FindEntry()
**
**	find the record in the entry table, via binary search
*/
DTSKeyEntryList *
DTSKeyFilePriv::FindEntry( DTSKeyType aType, DTSKeyID anID ) const
{
	const DTSKeyEntryList * entry = keyEntry;
	int count = keyHeader.keyCount;
	for(;;)
		{
		// bail when we run out of key entries to test
		if ( count <= 0 )
			return nullptr;
		
		// find the entry in the middle of the search range
		int indx = count >> 1;
		const DTSKeyEntryList * test = entry + indx;
		
		// is this our type?
		DTSKeyType testType = test->keyEntry.keyType;
		bool bIsLess;
		if ( testType == aType )
			{
			// is this our ID?
			DTSKeyID testID = test->keyEntry.keyID;
			if ( testID == anID )
				return const_cast<DTSKeyEntryList *>( test );
			
			// no, use the ID for the comparison
			bIsLess = (anID < testID);
			}
		// no, use the type for the comparison
		else
			bIsLess = (aType < testType);
		
		// do we look in the first half?
		if ( bIsLess )
			count = indx;
		// or in the second half?
		else
			{
			count -= indx + 1;
			entry  = test  + 1;
			}
		}
}


/*
**	DTSKeyFile::GetID()
**
**	return the ID of the indexed record
*/
DTSError
DTSKeyFile::GetID( DTSKeyType ttype, long indx, DTSKeyID * pid ) const
{
	const DTSKeyFilePriv * p = priv.p;
	return p ? p->GetID( ttype, indx, pid ) : -1;
}


/*
**	DTSKeyFilePriv::GetID()
**
**	return the ID of the indexed record
*/
DTSError
DTSKeyFilePriv::GetID( DTSKeyType aType, long indx, DTSKeyID * pid ) const
{
	long count = 0;
	DTSKeyID id = 0;
	
	// paranoid checks
	DTSError result = noErr;
	if ( -1 == keyRefNum
	||	 not keyEntry )
		{
		result = fnOpnErr;
		}
	if ( noErr == result )
		{
		if ( indx < 0 )
			result = -1;
		}
	
	// don't overreach
	if ( noErr == result )
		result = Count( aType, &count );
	if ( noErr == result )
		{
		if ( indx >= count )
			result = -1;
		}
	
	// offset from first entry of this type
	if ( noErr == result )
		{
		if ( const DTSKeyEntryList * entry = FindEntryFirst( aType ) )
			id = entry[indx].keyEntry.keyID;
		else
			result = -1;
		}
	
	if ( pid )
		*pid = id;
	
	return result;
}


/*
**	DTSKeyFile::GetIndex()
**
**	Get the index of an entry, given its ID.
**	(This is the inverse of GetID() above)
*/
DTSError
DTSKeyFile::GetIndex( DTSKeyType aType, DTSKeyID id, long * indexout ) const
{
	const DTSKeyFilePriv * p = priv.p;
	return p ? p->GetIndex( aType, id, indexout ) : -1;
}


/*
**	DTSKeyFilePriv::GetIndex()
**
**	return the index of the entry
*/
DTSError
DTSKeyFilePriv::GetIndex( DTSKeyType ttype, DTSKeyID id, long * indexout ) const
{
	long indx = 0;
	
	// paranoid checks
	DTSError result = noErr;
	if ( -1 == keyRefNum
	||   not keyEntry )
		{
		result = fnOpnErr;
		}
	
	if ( noErr == result )
		{
		// subtract N'th entry pointer from 0'th
		if ( const DTSKeyEntryList * entry = FindEntry( ttype, id ) )
			{
			const DTSKeyEntryList * first = FindEntryFirst( ttype );
			indx = entry - first;
			}
		else
			result = -1;
		}
	
	if ( indexout )
		*indexout = indx;
	
	return result;
}


/*
**	DTSKeyFile::GetUnusedID()
**
**	return an unused ID for the given KeyType within the file
**	(this doesn't attempt to fill up gaps in the used IDs; it just
**	looks at the current highest ID # for the type, and adds 1 to it.)
*/
DTSError
DTSKeyFile::GetUnusedID( DTSKeyType ttype, DTSKeyID * idout ) const
{
	const DTSKeyFilePriv * p = priv.p;
	return p ? p->GetUnusedID( ttype, idout ) : -1;
}


/*
**	DTSKeyFilePriv::GetUnusedID()
**
**	return an unused ID for the type
*/
DTSError
DTSKeyFilePriv::GetUnusedID( DTSKeyType ttype, DTSKeyID * idout ) const
{
	DTSKeyID id = 0;
	DTSError result = noErr;
	if ( -1 == keyRefNum || not keyEntry )
		result = fnOpnErr;
	
	if ( noErr == result )
		if ( const DTSKeyEntryList * entry = FindEntryLast( ttype ) )
			id = entry->keyEntry.keyID + 1;
	
	if ( idout )
		*idout = id;
	
	return result;
}


/*
**	DTSKeyFile::GetSize()
**
**	return the size (in bytes) of the designated record
*/
DTSError
DTSKeyFile::GetSize( DTSKeyType ttype, DTSKeyID id, size_t * psize ) const
{
	const DTSKeyFilePriv * p = priv.p;
	return p ? p->GetSize( ttype, id, psize ) : -1;
}


/*
**	DTSKeyFilePriv::GetSize()
**
**	return the size of the record
*/
DTSError
DTSKeyFilePriv::GetSize( DTSKeyType ttype, DTSKeyID id, size_t * psize ) const
{
	if ( -1 == keyRefNum || not keyEntry )
		return fnOpnErr;
	
	if ( const DTSKeyEntryList * entry = FindEntry( ttype, id ) )
		{
		if ( psize )
			*psize = static_cast<size_t>( entry->keyEntry.keySize );
		
		return noErr;
		}
	
	return -1;
}


/*
**	DTSKeyFile::Read()
**
**	read the record into the buffer.
**	assume the buffer is big enough, unless explicit max-size given
*/
DTSError
DTSKeyFile::Read( DTSKeyType ttype, DTSKeyID id, void * buffer, size_t bufsz /* =ULONG_MAX */ )
{
	DTSKeyFilePriv * p = priv.p;
	return p ? p->Read( ttype, id, buffer, bufsz ) : -1;
}


/*
**	DTSKeyFilePriv::Read()
**
**	read the record into the buffer, but not more than 'bufsz' bytes.
*/
DTSError
DTSKeyFilePriv::Read( DTSKeyType ttype, DTSKeyID id, void * buffer, size_t bufsz )
{
	if ( -1 == keyRefNum || not keyEntry )
		return fnOpnErr;
	
#if DEBUG_VERSION_KEYFILES
	CheckConsistency();
#endif
	
	// locate table entry for this record
	const DTSKeyEntryList * entry = FindEntry( ttype, id );
	if ( not entry )
		return -1;
	
	// enforce buffer size limit
	if ( static_cast<size_t>( entry->keyEntry.keySize ) < bufsz )
		bufsz = static_cast<size_t>( entry->keyEntry.keySize );
	
	// read data from file
	DTSError result = DTS_seek( keyRefNum, static_cast<size_t>( entry->keyEntry.keyPosition ) );
	if ( noErr == result && buffer )
		result = DTS_read( keyRefNum, buffer, bufsz );
	
#if DEBUG_VERSION_KEYFILES
	CheckConsistency();
#endif
	
	return result;
}


/*
**	DTSKeyFile::ReadAlloc()
**
**	allocate memory for a record and read it.
**	If successful, a pointer to that memory is returned in *buffer; the caller
**	must eventually deallocate that storage via operator delete[]( char * ).
**	Otherwise returns a non-zero error code.
*/
DTSError
DTSKeyFile::ReadAlloc( DTSKeyType ttype, DTSKeyID id, void *& buffer )
{
	DTSKeyFilePriv * p = priv.p;
	return p ? p->ReadAlloc( ttype, id, buffer ) : -1;
}


/*
**	DTSKeyFilePriv::ReadAlloc()
**
**	allocate memory for a record and read it
*/
DTSError
DTSKeyFilePriv::ReadAlloc( DTSKeyType ttype, DTSKeyID id, void *& buffer )
{
	size_t size;
	char * temp = nullptr;
	
	// TODO: it would be convenient to report this size back to our callers,
	// somehow, to save them from always having to call GetSize() themselves.
	DTSError result = GetSize( ttype, id, &size );
	if ( noErr == result )
		{
		temp = NEW_TAG("DTSKeyReadAlloc") char[ size ];
		if ( not temp )
			result = memFullErr;
		}
	if ( noErr == result )
		result = Read( ttype, id, temp, size );
	
	// fail cleanly
	if ( result != noErr )
		{
		delete[] temp;
		temp = nullptr;
		}
	
	buffer = temp;
	
	return result;
}


/*
**	DTSKeyFile::Write()
**
**	write the record.
**	if the file already contains a record having the given type and ID,
**	replace it; otherwise add it.
*/
DTSError
DTSKeyFile::Write( DTSKeyType ttype, DTSKeyID id, const void * buffer, size_t size )
{
	DTSKeyFilePriv * p = priv.p;
	return p ? p->Write( ttype, id, buffer, size ) : -1;
}


/*
**	DTSKeyFilePriv::Write()
**
**	write the record.
*/
DTSError
DTSKeyFilePriv::Write( DTSKeyType ttype, DTSKeyID id, const void * buffer, size_t size )
{
	// paranoid checks
	if ( -1 == keyRefNum )
		return fnOpnErr;
	if ( not keyEntry )
		return memFullErr;
	if ( not keyWritePerm )
		return wrPermErr;
	if ( size <= 0 )
		return -1;
	
#if DEBUG_VERSION_KEYFILES
	CheckConsistency();
#endif
	
	long orgcount = keyHeader.keyCount;
	
	// find an entry in the table to use; create one if necessary
	DTSKeyEntryList * entry;
	DTSError result = FindCreateEntry( ttype, id, size, &entry );
	
#if DEBUG_VERSION_KEYFILES
	CheckConsistency();
#endif
	
	// find a place to write the record
	ulong position = 0;
	DTSKeyEntryList * after = nullptr;
	if ( noErr == result )
		{
		result = FindBestPosition( entry, size, &after, &position );
		
#if DEBUG_VERSION_KEYFILES
		CheckConsistency();
#endif
		
		// brain damage
		// look for a collision
#if DEBUG_VERSION_KEYFILES
		CheckKeyCollision( entry, position, size );
#endif
		}
	// write the record
	if ( noErr == result )
		result = DTS_seek( keyRefNum, position );
	if ( noErr == result )
		result = DTS_write( keyRefNum, buffer, size );
	
	// update the table
	if ( noErr == result )
		{
		// update the entry
		size_t orgsize  = static_cast<size_t>( entry->keyEntry.keySize );
		int orgposition = entry->keyEntry.keyPosition;
		entry->keyEntry.keySize     = static_cast<int32_t>( size );
		entry->keyEntry.keyPosition = static_cast<int32_t>( position );
		
		// handle an oddball case
		if ( after == entry )
			after = entry->keyPrev;
		
		// remove ourselves from the list
		// and re-install ourselves into it
		// need to do this before the call to WriteTable()
		// because it might change the position list
		// which would invalidate 'after'
		RemovePositionList( entry );
		DTSKeyEntryList * test;
		if ( not after )
			{
			// install first
			test = keyFirst;
			keyFirst = entry;
			}
		else
			{
			// install after
			test = after->keyNext;
			after->keyNext = entry;
			}
		entry->keyPrev = after;
		entry->keyNext = test;
		if ( test )
			test->keyPrev = entry;
		
#if DEBUG_VERSION_KEYFILES
		CheckConsistency();
#endif
		
		// write the entire table if we added a new entry
		if ( orgcount != keyHeader.keyCount )
			{
			result = WriteHeader();
			if ( noErr == result )
				result = WriteTable();
			}
		// write this entry if the position or size has changed 
		else
		if ( orgposition != entry->keyEntry.keyPosition
		||   orgsize     != size )
			{
			result = WriteTableEntry( entry );
			}
		}
	
#if DEBUG_VERSION_KEYFILES
	LogToStream( "%p Write t/i %4.4s %d p/s %.8X %.8X\n", this,
		reinterpret_cast<const char *>( &entry->keyEntry.keyType ),
		static_cast<int>( entry->keyEntry.keyID ),
		entry->keyEntry.keyPosition,
		entry->keyEntry.keySize );
#endif
	
#if DEBUG_VERSION_KEYFILES
	CheckConsistency();
#endif
	
	return result;
}


/*
**	DTSKeyFilePriv::FindCreateEntry()
**
**	find the entry
**	create it if it doesn't exist
*/
DTSError
DTSKeyFilePriv::FindCreateEntry( DTSKeyType ttype, DTSKeyID id, size_t size,
	DTSKeyEntryList ** pentry )
{
	// find it
	DTSKeyEntryList * entry = FindEntry( ttype, id );
	if ( entry )
		{
		*pentry = entry;
		return noErr;
		}
	
	// is there room for another?
	long count = keyHeader.keyCount;
	long max   = keyMax;
	if ( count >= max )
		{
		// nope; allocate a new larger table
#if DEBUG_VERSION_KEYFILES
		LogToStream( "%p expanding table %d\n", this, static_cast<int>( keyHeader.keyCount + 1 ) );
#endif
		
		long newcount = count + kTableBumpSize;
		size_t newsize = ulong( newcount ) * sizeof(DTSKeyEntryList);
		DTSKeyEntryList * newtable = reinterpret_cast<DTSKeyEntryList *>(
				NEW_TAG("DTSKeyExpandTable") char[ newsize ] );
		if ( not newtable )
			return memFullErr;
		
		// copy and delete the old table
		DTSKeyEntryList * oldtable = keyEntry;
		if ( oldtable )
			{
			DTSKeyEntryList * test = keyFirst;
			if ( test )
				keyFirst = newtable + (test - oldtable);
			DTSKeyEntryList * src = oldtable + count;
			DTSKeyEntryList * dst = newtable + count;
			while ( dst > newtable )
				{
				--src;
				--dst;
				
				dst->keyEntry = src->keyEntry;
				test = src->keyPrev;
				if ( test )
					test = newtable + (test - oldtable);
				dst->keyPrev = test;
				test = src->keyNext;
				if ( test )
					test = newtable + (test - oldtable);
				dst->keyNext = test;
				}
			delete[] reinterpret_cast<char *>( oldtable );
			}
		
		// update the new table
		keyMax   = newcount;
		keyEntry = newtable;
		}
	
	// update the count
	keyHeader.keyCount = static_cast<int32_t>( count + 1 );
	
	// sort the table
	entry = SortTableWithLinks( ttype, id, size );
	*pentry = entry;
	
	return noErr;
}


/*
**	DTSKeyFilePriv::SortTableWithLinks()
**
**	sort the key entry table
**	assume the new entry is last
**	and is the only one out of order
*/
DTSKeyEntryList *
DTSKeyFilePriv::SortTableWithLinks( DTSKeyType ttype, DTSKeyID id, size_t size )
{
	long count = keyHeader.keyCount - 1;
	DTSKeyEntryList * table = keyEntry;
	DTSKeyEntryList * entry = table;
	DTSKeyEntryList * insert = entry + count;
	DTSKeyEntryList * test;
	
	// bail when we run out of key entries to test
	while ( count > 0 )
		{
		// find the entry in the middle
		long indx = count >> 1;
		test = entry + indx;
		
		// is this our type?
		DTSKeyType testType = test->keyEntry.keyType;
		bool bIsLess;
		if ( testType == ttype )
			bIsLess = id < test->keyEntry.keyID;	// compare by IDs
		else
			bIsLess = ttype < testType;				// compare by types
		
		// do we look in the first half?
		if ( bIsLess )
			count = indx;
		// or in the second half?
		else
			{
			count -= indx + 1;
			entry  = test  + 1;
			}
		}
	
	// adjust the first link
	test = keyFirst;
	if ( test >= entry
	&&   test <  insert )
		{
		keyFirst = test + 1;
		}
	
	// adjust all links in the table
	for ( ;  table < insert;  ++table )
		{
		test = table->keyPrev;
		if ( test >= entry
		&&   test <  insert )
			{
			table->keyPrev = test + 1;
			}
		test = table->keyNext;
		if ( test >= entry
		&&   test <  insert )
			{
			table->keyNext = test + 1;
			}
		}
	
	// move the entries
	while ( entry < insert )
		{
		--insert;
		insert[1] = insert[0];
		}
	
	// set our entry
	entry->keyEntry.keyPosition = 0;		// means "file offset not yet assigned"
	entry->keyEntry.keySize     = static_cast<int32_t>( size );
	entry->keyEntry.keyType     = ttype;
	entry->keyEntry.keyID       = id;
	
	entry->keyPrev              = nullptr;
	entry->keyNext              = nullptr;
	
	return entry;
}


/*
**	DTSKeyFilePriv::FindBestPosition()
**
**	find the best position to write the record
**	prefer to reuse empty holes, if any, before forcing the file to grow
**	first, try its current location
**	second, try before the first record
**	third, try somewhere in the middle of the file
**	lastly, append to the end of the file
*/
DTSError
DTSKeyFilePriv::FindBestPosition( DTSKeyEntryList * entry, size_t size,
	DTSKeyEntryList ** after, ulong * pposition )
{
	DTSError result;
	
	// first, try its current location, if not fictitious
	// (newly-made, never-written entries have position == 0)
	ulong position = static_cast<ulong>( entry->keyEntry.keyPosition );
	ulong tablelimit = kSizeOfKeyHeader1 + static_cast<ulong>( keyHeader.keyCount ) * sizeof(DTSKeyEntry);
	if ( position >= tablelimit )
		{
		// we can use the current location if...
		// we are the last record in the file (ergo can grow as large as we wish)
		// or we won't overwrite the next record
		const DTSKeyEntryList * test = entry->keyNext;
		if ( not test		// i.e. we are last
		||   position + size <= ulong( test->keyEntry.keyPosition ) ) 
			{
#if DEBUG_VERSION_KEYFILES
			LogToStream( "%p best position current %.8lX %.8lX\n", this, position, size );
#endif
			
			*after = entry->keyPrev;
			*pposition = position;
			return noErr;
			}
		}
	
	// use the end of the file if there are no records
	DTSKeyEntryList * test = keyFirst;
	if ( not test )
		{
		*after = nullptr;
		result = DTS_geteof( keyRefNum, pposition );
		
#if DEBUG_VERSION_KEYFILES
		LogToStream( "%p best position eof %.8lX %.8lX\n", this, *pposition, size );
#endif
		
		return result;
		}
	
	// second, try before the first record
	// leave room for the current table
	if ( ulong(test->keyEntry.keyPosition) >= tablelimit + size )
		{
		position = static_cast<size_t>( test->keyEntry.keyPosition ) - size;
		*after   = nullptr;
		*pposition = position;
		
#if DEBUG_VERSION_KEYFILES
		LogToStream( "%p best position before first %.8lX %.8lX\n", this, *pposition, size );
#endif
		
		return noErr;
		}
	
	// third, try somewhere in the middle of the file
	ulong bestpos = 0;
	int bestwaste = INT_MAX;
	DTSKeyEntryList * bestentry = nullptr;
	for(;;)
		{
		// any more entries to test?
		DTSKeyEntryList * next = test->keyNext;
		if ( not next )
			{
			// bail if there was nowhere to put it. 'test' will be the last existing entry.
			if ( not bestentry )
				break;
			
			// success
			*after     = bestentry;
			*pposition = bestpos;
			
#if DEBUG_VERSION_KEYFILES
			LogToStream( "%p best position good fit %.8lX %.8lX\n", this, bestpos, size );
#endif
			
			return noErr;
			}
		
		// make sure we're beyond the table
		position = static_cast<ulong>( test->keyEntry.keyPosition + test->keyEntry.keySize );
		if ( position < tablelimit )		// can this even happen?
			position = tablelimit;
		
		// compare the size of the gap between 'test' and 'next' against our own size
		int waste = int( next->keyEntry.keyPosition - long( position ) - long( size ) );	// can be negative
		
		// if we find a perfect fit, claim it immediately and stop looking.
		// possible TO-DO: consider whether there'd be any advantages in also allowing
		// "nearly perfect" fit, such that (waste >= 0 && waste < kMaxWaste), where
		// kMaxWaste is on the order of, say, 4 to 16 bytes. This might lead to _slightly_
		// quicker allocations, at the cost of _slightly_ greater disk usage.
		if ( 0 == waste )
			{
			*after     = test;
			*pposition = position;
			
#if DEBUG_VERSION_KEYFILES
			LogToStream( "%p best position perfect fit %.8lX %.8lX\n", this, position, size );
#endif
			
			return noErr;
			}
		
		// check if this hole is a better fit than any we've yet seen
		if ( waste > 0
		&&   waste < bestwaste )
			{
			bestwaste = waste;
			bestpos   = position;
			bestentry = test;
			}
		
		// try after the next entry
		test = next;
		}
	
	// if all else fails, simply append to the end of the file
	*after     = test;
	*pposition = static_cast<ulong>( test->keyEntry.keyPosition + test->keyEntry.keySize );
	
#if DEBUG_VERSION_KEYFILES
	LogToStream( "%p best position after last %.8lX %.8lX\n", this, *pposition, size );
#endif
	
	return noErr;
}


/*
**	DTSKeyFile::Delete()
**
**	remove a record from the keyfile
*/
DTSError
DTSKeyFile::Delete( DTSKeyType ttype, DTSKeyID id )
{
	DTSKeyFilePriv * p = priv.p;
	return p ? p->Delete( ttype, id ) : -1;
}


/*
**	DTSKeyFilePriv::Delete()
**
**	delete the record
*/
DTSError
DTSKeyFilePriv::Delete( DTSKeyType ttype, DTSKeyID id )
{
	// paranoid checks
	if ( -1 == keyRefNum )
		return fnOpnErr;
	if ( not keyEntry )
		return memFullErr;
	if ( not keyWritePerm )
		return wrPermErr;
	
	// find the entry
	DTSKeyEntryList * entry = FindEntry( ttype, id );
	if ( not entry )
		return -1;
	
#if DEBUG_VERSION_KEYFILES
	CheckConsistency();
#endif
	
	// decrement the number of entries, in the header
	long count = --keyHeader.keyCount;
	
	// remove it from the position list
	RemovePositionList( entry );
	
	// adjust the first link
	DTSKeyEntryList * test = keyFirst;
	DTSKeyEntryList * limit = keyEntry + count;
	if ( test >  entry
	&&   test <= limit )
		{
		keyFirst = test - 1;
		}
	
	// adjust all links in the table
	for ( DTSKeyEntryList * table = keyEntry;  table <= limit;  ++table )
		{
		test = table->keyPrev;
		if ( test >  entry
		&&   test <= limit )
			{
			table->keyPrev = test - 1;
			}
		test = table->keyNext;
		if ( test >  entry
		&&   test <= limit )
			{
			table->keyNext = test - 1;
			}
		}
	
	// move all the following entries
	for ( ;  entry < limit;  ++entry )
		entry[0] = entry[1];
	
	// write header and data
	DTSError result = WriteHeader();
	if ( noErr == result )
		result = WriteTable();
	
#if DEBUG_VERSION_KEYFILES
	CheckConsistency();
#endif
	
	return result;
}


/*
**	DTSKeyFilePriv::RemovePositionList()
**
**	remove the entry from the position list
*/
void
DTSKeyFilePriv::RemovePositionList( DTSKeyEntryList * entry )
{
	DTSKeyEntryList * test = entry->keyPrev;
	if ( test )
		test->keyNext = entry->keyNext;
	else
	if ( entry == keyFirst )
		keyFirst = entry->keyNext;
	
	test = entry->keyNext;
	if ( test )
		test->keyPrev = entry->keyPrev;
}


/*
**	DTSKeyFile::GetInfo()
**
**	return info about the file
*/
DTSError
DTSKeyFile::GetInfo( DTSKeyInfo * info ) const
{
	const DTSKeyFilePriv * p = priv.p;
	return p ? p->GetInfo( info ) : -1;
}


/*
**	DTSKeyFilePriv::GetInfo()
**
**	get assorted info about the file:
**		number of types and records
**		actual size (on disk)
**		compressed size (total of all data records, plus overhead --
**		i.e., what the actual size would be, after calling Compress())
*/
DTSError
DTSKeyFilePriv::GetInfo( DTSKeyInfo * info ) const
{
	// paranoid checks
	if ( -1 == keyRefNum || not keyEntry )
		return fnOpnErr;
	if ( not info )
		return -1;
	
	// get the size of the file
	DTSError result;
	{
	ulong fsize;
	result = DTS_geteof( keyRefNum, &fsize );
	info->keyFileSize = static_cast<long>( fsize );
	}
	
	// fill in the other fields
	if ( noErr == result )
		{
		// copy the number of records
		int numRecords = keyHeader.keyCount;
		info->keyNumRecords = numRecords;
		
		// tot up the number of record types
		uint numTypes = 0;
		const DTSKeyEntryList * entry = keyEntry;
		DTSKeyType lastType = ~ entry->keyEntry.keyType; // anything that's != thisType
		
		// and the total compressed size, starting with the header and the table entries
		ulong compressedSize = kSizeOfKeyHeader1 + uint(numRecords) * sizeof(DTSKeyEntry);
		
		for ( uint count = uint(numRecords);  count > 0;  --count )
			{
			// bump type-count if there's a change
			DTSKeyType thisType = entry->keyEntry.keyType;
			if ( lastType != thisType )
				{
				lastType = thisType;
				++numTypes;
				}
			
			// accumulate record sizes
			compressedSize += ulong(entry->keyEntry.keySize);
			
			++entry;
			}
		
		info->keyNumTypes = static_cast<long>( numTypes );
		info->keyCompressedSize = static_cast<long>( compressedSize );
		}
	
	return result;
}


/*
**	DTSKeyFile::Compress()
**
**	compress the file to its smallest size
*/
DTSError
DTSKeyFile::Compress()
{
	DTSKeyFilePriv * p = priv.p;
	return p ? p->Compress() : -1;
}


/*
**	DTSKeyFilePriv::Compress()
**
**	compress the file to its smallest size
**	by eliminating unused space in the file
*/
DTSError
DTSKeyFilePriv::Compress()
{
	if ( -1 == keyRefNum )
		return fnOpnErr;
	if ( not keyEntry )
		return memFullErr;
	if ( not keyWritePerm )
		return wrPermErr;
	
	// read all of the records and append them to the end of the file
	DTSKeyEntryList * entry;
	long nnn;
	void * temp;
	ulong position;
	ulong tableend = 0;
	size_t size = 0;
	long count = keyHeader.keyCount;
	DTSError result = DTS_geteof( keyRefNum, &position );
	if ( noErr == result )
		{
		// make sure there's room on disk for the entire entry table
		tableend = kSizeOfKeyHeader1 + uint(count) * sizeof(DTSKeyEntry);
		if ( position < tableend )
			{
			position = tableend;
			result = DTS_seteof( keyRefNum, position );
			}
		}
	if ( noErr == result )
		{
		entry = keyEntry;
		for ( nnn = count;  nnn > 0;  --nnn, ++entry )
			{
			result = ReadAlloc( entry->keyEntry.keyType, entry->keyEntry.keyID, temp );
			if ( result != noErr )
				break;
			result = DTS_seek( keyRefNum, position );
			if ( noErr == result )
				{
				size = static_cast<size_t>( entry->keyEntry.keySize );
				result = DTS_write( keyRefNum, temp, size );
				}
			delete[] static_cast<char *>( temp );
			if ( result != noErr )
				break;
			entry->keyEntry.keyPosition = static_cast<int32_t>( position );
			position += size;
			}
		}
	
	// okay, we haven't blown away any data in the file yet
	// (even though we've duplicated every record, after the file's logical end).
	// let's sync up the header table and the newly written records.
	// so we can safely blow away the old records
	// just in case one of those darn users turns the power off.
	if ( noErr == result )
		result = WriteTable();
	
	// read all of the records which are at the end of the file
	// and write them to their new locations
	if ( noErr == result )
		{
		entry = keyEntry;
		position = tableend;
		for ( nnn = count;  nnn > 0;  --nnn, ++entry )
			{
			result = ReadAlloc( entry->keyEntry.keyType, entry->keyEntry.keyID, temp );
			if ( result != noErr )
				break;
			entry->keyEntry.keyPosition = static_cast<int32_t>( position );
			size = static_cast<size_t>( entry->keyEntry.keySize );
			result = DTS_seek( keyRefNum, position );
			if ( noErr == result )
				result = DTS_write( keyRefNum, temp, size );
			delete[] static_cast<char *>( temp );
			if ( result != noErr )
				break;
			position += size;
			}
		}
	
	// write out the new table (again)
	if ( noErr == result )
		{
		InitPositionList();
		result = WriteTable();
		}
	// set the new end of the file
	if ( noErr == result )
		result = DTS_seteof( keyRefNum, position );
	
	return result;
}


/*
**	DTSKeyFilePriv::ReadKeyHeader()
**
**	read the header
**	pay attention to possible alignment problems
*/
DTSError
DTSKeyFilePriv::ReadKeyHeader( int refNum, DTSKeyHeader * header )
{
	DTSError result;
	
	// do we have alignment problems?
	if ( sizeof(DTSKeyHeader) != kSizeOfKeyHeader1 )
		{
		// the data in the header in the file is misaligned
		// we need to align it properly
		// in other words...
		// what's on disk is really a DTSKeyHeaderOld2 (12 bytes)
		char temp[ kSizeOfKeyHeader1 ];
		result = DTS_read( refNum, temp, kSizeOfKeyHeader1 );
		if ( noErr == result )
			{
# if defined( DTS_XCODE ) || TARGET_API_MAC_OSX
			// we can use the fancy Darwin endian macros
			header->keyVersion	= static_cast<int16_t>( OSReadBigInt16( temp, 0 ) );
			header->keyCount	= static_cast<int32_t>( OSReadBigInt32( temp, 2 ) );
			header->keyNotUsed	= static_cast<int32_t>( OSReadBigInt32( temp, 6 ) );
			header->keyVersion2	= static_cast<int16_t>( OSReadBigInt16( temp, 10 ) );
# else
			int16_t tempversion;
			int32_t tempcount;
			int32_t tempunused;
			int16_t tempversion2;
			
			std::memcpy( &tempversion,  &temp[ 0], sizeof tempversion );
			std::memcpy( &tempcount,    &temp[ 2], sizeof tempcount );
			std::memcpy( &tempunused,   &temp[ 6], sizeof tempunused );
			std::memcpy( &tempversion2, &temp[10], sizeof tempversion2 );
			
			header->keyVersion  = BigToNativeEndian( tempversion );
			header->keyCount    = BigToNativeEndian( tempcount );
			header->keyNotUsed  = BigToNativeEndian( tempunused );
			header->keyVersion2 = BigToNativeEndian( tempversion2 );
# endif  // DTS_XCODE
			}
		}
	else
		{
		// no alignment problems
		// fire away
		// what's on disk is a true DTSKeyHeader (16 bytes)
		// alas, this doesn't ever happen in practice.
		
#if DTS_BIG_ENDIAN
		result = DTS_read( refNum, &header, kSizeOfKeyHeader1 );
#else
		// convert to native endianness just after reading from disk
		DTSKeyHeader temp;
		result = DTS_read( refNum, &temp, sizeof temp );
		if ( noErr == result )
			{
			header->keyVersion = BigToNativeEndian( temp.keyVersion );
			header->keyCount   = BigToNativeEndian( temp.keyCount );
			header->keyNotUsed = BigToNativeEndian( temp.keyNotUsed );
			header->keyVersion2 = BigToNativeEndian( temp.keyVersion2 );
			}
#endif
		}
	
	return result;
}


/*
**	DTSKeyFilePriv::WriteKeyHeader()
**
**	write the header
**	pay attention to possible alignment problems
*/
DTSError
DTSKeyFilePriv::WriteKeyHeader( int refNum, const DTSKeyHeader * header )
{
	DTSError result;
	
	// do we have alignment problems?
	if ( sizeof(DTSKeyHeader) != kSizeOfKeyHeader1 )
		{
		// the data in the header in the file is misaligned
		// we need to align it properly
		char temp[ kSizeOfKeyHeader1 ];
#if defined( DTS_XCODE ) || TARGET_API_MAC_OSX
		// again with the snazzy macros
		OSWriteBigInt16( temp, 0, header->keyVersion );
		OSWriteBigInt32( temp, 2, header->keyCount );
		OSWriteBigInt32( temp, 6, header->keyNotUsed );
		OSWriteBigInt16( temp, 10, header->keyVersion2 );
#else
		int16_t tempversion  = NativeToBigEndian( (int16_t) header->keyVersion );
		int32_t tempcount    = NativeToBigEndian( header->keyCount );
		int32_t tempunused   = NativeToBigEndian( header->keyNotUsed );
		int16_t tempversion2 = NativeToBigEndian( (int16_t) header->keyVersion2 );
		
		std::memcpy( &temp[ 0], &tempversion,  sizeof tempversion );
		std::memcpy( &temp[ 2], &tempcount,    sizeof tempcount );
		std::memcpy( &temp[ 6], &tempunused,   sizeof tempunused );
		std::memcpy( &temp[10], &tempversion2, sizeof tempversion2 );
#endif  // DTS_XCODE
		
		result = DTS_write( refNum, temp, kSizeOfKeyHeader1 );
		}
	else
		{
		// no alignment problems -- it must be a true 16-byte DTSKeyHeader
		// fire away
#if DTS_BIG_ENDIAN
		result = DTS_write( refNum, header, sizeof header );
#else
		// convert to network endianness just before flushing to disk
		DTSKeyHeader temp;
		temp.keyVersion  = NativeToBigEndian( header->keyVersion );
		temp.keyCount    = NativeToBigEndian( header->keyCount );
		temp.keyNotUsed  = NativeToBigEndian( header->keyNotUsed );
		temp.keyVersion2 = NativeToBigEndian( header->keyVersion2 );
		result = DTS_write( refNum, &temp, sizeof temp );
#endif
		}
	
	return result;
}


#if DEBUG_VERSION_KEYFILES
/*
**	LogToStream
**
**	log debugging text to the file
*/
void
LogToStream( const char * format, ... )
{
	FILE * stream = gStream;
	if ( not stream )
		{
		stream = fopen( DEBUG_LOG_FILE, "w" );
		if ( not stream )
			return;
		setvbuf( stream, nullptr, _IOLBF, 0 );		// make line-buffered
		gStream = stream;
		}
	
	va_list params;
	va_start( params,format );
	
	vfprintf( stream, format, params );
	
	va_end( params );
}


/*
**	CheckConsistency()
**
**	check the consistency of the file
*/	
void
DTSKeyFilePriv::CheckConsistency()
{
	// for debugging
	if ( keyHeader.keyCount <= 1 )
		return;
	if ( not keyFirst )
		{
		VDebugStr( "uh oh: no keyFirst" );
		return;
		}
	
	int32_t bd_pos = keyFirst->keyEntry.keyPosition;
	const DTSKeyEntryList * bd_walk = keyEntry;
	
	// verify that keyFirst really IS the first
	long bd_count;
	for ( bd_count = keyHeader.keyCount;  bd_count > 0;  --bd_count, ++bd_walk )
		{
		int32_t walkPos = bd_walk->keyEntry.keyPosition;
		if ( walkPos >= 0 && walkPos < bd_pos )
			VDebugStr( "uh oh: key entry before first" );
		}
	
	// verify that the header's keyCount matches the size of the linked list
	bd_count = keyHeader.keyCount;
	bd_pos = 0;
	for ( bd_walk = keyFirst;  bd_walk;  bd_walk = bd_walk->keyNext )
		++bd_pos;
	
	long bd_diff = bd_count - bd_pos;
	if ( bd_diff != 0
	&&   bd_diff != 1 )
		{
		VDebugStr( "uh oh: key count incorrect" );
		}
	
	// TODO: maybe call CheckKeyCollision() here on every entry in the file
	// to verify that there are no overlaps in the keys' data
}


/*
**	CheckKeyCollision()
**
** sanity check for a newly-placed entry:
**	is it reasonably located? Does it collide with an existing entry?
*/
void
DTSKeyFilePriv::CheckKeyCollision( const DTSKeyEntryList * entry, ulong position, size_t size )
{
	long bd_count = keyHeader.keyCount;
	if ( position < kSizeOfKeyHeader1 + bd_count * sizeof(DTSKeyEntry) )
		{
		VDebugStr( "uh oh: position inside table" );
		return;
		}
	
	ulong bd_end = position + size;
	const DTSKeyEntryList * bd_walk = keyEntry;
	for ( ;  bd_count > 0;  --bd_count, ++bd_walk )
		{
		if ( bd_walk != entry )
			{
			int32_t walkPos = bd_walk->keyEntry.keyPosition;
			if ( walkPos < bd_end
			&&   walkPos + bd_walk->keyEntry.keySize > int32_t(position) )
				{
				VDebugStr( "uh oh: entry collision" );
				}
			}
		}
}
#endif  // DEBUG_VERSION_KEYFILES


