/*
**	Utilities_cl.cp		CLServer, CLEditor, ClanLord
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

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "DatabaseTypes_cl.h"
#include "Public_cl.h"

using std::snprintf;
using std::vsnprintf;
using std::memcpy;


/*
**	Entry Routines
*/
/*
void		SimpleEncrypt( void * data, size_t sz );
void		SpoolFramePicture( DataSpool * spool, DTSKeyID pictID, int horz, int vert );
void		UnspoolFramePicture( DataSpool * s, DTSKeyID& pictID, int& horz, int& vert );
#if BITWISE_IMAGE_SPOOL
void		UnspoolFramePicture( DataSpool * s, DTSKeyID& pictID, int& horz, int& vert,
								 int idBits, int coordBits )
#endif	// BITWISE_IMAGE_SPOOL
DTSError	RemoveCrapFromImagesFile( DTSKeyFile * file );
DTSError	RemoveCrapFromSoundsFile( DTSKeyFile * file );
DTSError	RemoveCrapFromFile( DTSKeyFile * file, const DTSKeyType * goodTypes );
DTSError	Delete1Type( DTSKeyFile * file, DTSKeyType type );
void		CompactName( const char * source, char * dest, size_t destsize );
DTSError	ReadFileVersion( DTSKeyFile * file, int32_t * version );
DTSError	GetUnusedIDNotZero( DTSKeyFile * file, DTSKeyType inType, DTSKeyID& outID );
#ifdef MULTILINGUAL
int			ExtractLangText( const char ** text, int langid, int * gender);
#endif
*/


/*
**	Variables
*/
char	SafeString::gSafeStringZero;


/*
**	DataSpool::DataSpool()
**
**	constructor
*/
DataSpool::DataSpool() :
	dsResult(),
	dsMax(),
	dsLimit(),
	dsData(),
	dsMark(),
	dsBitMark()
{
}


/*
**	DataSpool::~DataSpool()
**
**	deconstructor
*/
DataSpool::~DataSpool()
{
	delete[] dsData;
}


/*
**	DataSpool::Init()
**
**	initialize the data spool
*/
DTSError
DataSpool::Init( size_t maxSize )
{
	DTSError result;
	delete[] dsData;
	
	char * data = NEW_TAG("DataSpool") char[ maxSize ];
	if ( not data )
		{
		result = memFullErr;
		dsMax  = 0;
		}
	else
		{
		result = noErr;
		dsMax  = maxSize;
		}
	dsMark    = 0;
	dsBitMark = 0;
	dsLimit   = 0;
	dsData    = data;
	dsResult  = result;
	
	return result;
}


/*
**	DataSpool::PutData()
**
**	put byte data into the spool at the current mark
*/
void
DataSpool::PutData( const void * buffer, size_t sz )
{
	if ( 0 != dsBitMark )
		dsResult = kSpoolErrNotAligned;
	
	if ( noErr != dsResult )
		return;
	
	size_t oldmark = dsMark;
	size_t newmark = oldmark + sz;
	if ( newmark > dsMax )
		{
		newmark = dsMax;
		sz = newmark - dsMark;
		dsResult = kSpoolErrFull;
		}
	
	dsMark  = newmark;
	dsLimit = newmark;
	memcpy( dsData + oldmark, buffer, sz );
}


/*
**	DataSpool::PutString()
**
**	put a string into the spool at the current mark
*/
void
DataSpool::PutString( const char * str )
{
	size_t len = std::strlen( str ) + 1;
	PutData( str, len );
}


/*
**	DataSpool::PutNumber()
**
**	put a number into the spool at the current mark
*/
void
DataSpool::PutNumber( int number, DataSpoolNumberType numType )
{
	switch ( numType )
		{
		case kSpoolSignedByte:
		case kSpoolUnsignedByte:
			{
			uint8_t number8 = static_cast<uint8_t>( number );	// endian OK
			PutData( &number8, sizeof number8 );
			}
			break;
		
		case kSpoolSignedShort:
		case kSpoolUnsignedShort:
			{
			uint16_t number16 = NativeToBigEndian( static_cast<ushort>( number ) );
			PutData( &number16, sizeof number16 );
			}
			break;
		
		default:
			{
			uint32_t number32 = static_cast<uint32_t>( NativeToBigEndian( number ) );
			PutData( &number32, sizeof number32 );
			}
			break;
		}
}


/*
**	DataSpool::GetData()
**
**	get aligned data from the spool at the current byte mark
**	buffer is filled from the beginning byte, not shifted nor sign-extended
*/
void
DataSpool::GetData( void * buffer, size_t sz )
{
	if ( 0 != dsBitMark )
		dsResult = kSpoolErrNotAligned;
	
	if ( noErr != dsResult )
		return;
	
	size_t mark = dsMark;
	const char * src = dsData + mark;
	memcpy( buffer, src, sz );
	
	mark += sz;
	if ( mark > dsLimit )
		{
		mark = dsLimit;
		dsResult = kSpoolErrEmpty;
		}
	
	dsMark = mark;
}


/*
**	DataSpool::GetString()
**
**	get a null-terminated string from the spool at the current mark
*/
void
DataSpool::GetString( char * dst, size_t maxsize )
{
	if ( 0 != dsBitMark )
		dsResult = kSpoolErrNotAligned;
	
	if ( dsResult != noErr )
		return;
	
	// point to the source string
	size_t mark = dsMark;
	const char * src = dsData + mark;
	
	// find the new mark
	size_t len = std::strlen( src ) + 1;
	mark += len;
	
	// check for over-reading
	if ( mark > dsMax )
		{
		mark = dsMax;
		dsResult = kSpoolErrEmpty;
		*dst = 0;
		}
	// copy the string
	// make sure we don't overflow the destination
	else
		StringCopyPad( dst, src, maxsize );
	
	// update the mark
	dsMark = mark;
}


/*
**	DataSpool::GetNumber()
**
**	get a number from the spool at the current mark
*/
int
DataSpool::GetNumber( DataSpoolNumberType numType )
{
	if ( dsResult != noErr )
		return 0;
	
	uint8_t number8;
	uint16_t number16;
	int32_t number32 = 0;
	
	switch ( numType )
		{
		case kSpoolSignedByte:
			GetData( &number8, 1 );
			if ( noErr == dsResult )
				number32 = (int) (signed char) number8;	// sign-extend
			break;
		
		case kSpoolUnsignedByte:
			GetData( &number8, 1 );
			if ( noErr == dsResult )
				number32 = number8;
			break;
		
		case kSpoolSignedShort:
			GetData( &number16, 2 );
			if ( noErr == dsResult )
				number32 = (int) BigToNativeEndian( (short) number16 );
			break;
		
		case kSpoolUnsignedShort:
			GetData( &number16, 2 );
			if ( noErr == dsResult )
				number32 = (int) BigToNativeEndian( (short) number16 );
			break;
		
		default:
			GetData( &number32, 4 );
			if ( noErr == dsResult )
				number32 = BigToNativeEndian( number32 );
			break;
		}
	
	return number32;
}


/*
**	DataSpool::PutBits()
**
**	put bit data into the spool at the current mark
**	uses 'numbits' lowest-order bits from 'value'
*/
void
DataSpool::PutBits( int inVal, uint numbits )
{
	if ( noErr != dsResult )
		return;
	
	// don't put more bits than exist
//	const uint kBitsPerWord = CHAR_BIT * sizeof(int32_t);
	enum { kBitsPerWord = CHAR_BIT * sizeof(int32_t) };
	if ( numbits > kBitsPerWord )
		numbits = kBitsPerWord;
	
	// left-align incoming bits
//	int32_t val = NativeToBigEndian( value );	// NOPE!
		// if we do that, we'd be spooling
		// bits from the wrong end of the input word!
	int32_t val = inVal << (kBitsPerWord - numbits);
	
	int bitstowrite = int( numbits );
	uint8_t * dest = reinterpret_cast<uint8_t *>( dsData + dsMark );
	
	// fill space in data buffer
	while ( bitstowrite > 0 )
		{
		// fill current byte
		uint8_t oldbyte = *dest;
		// get the high 8 bits
		uint8_t newbyte = uint8_t( uint32_t(val) >> (kBitsPerWord - CHAR_BIT) );
		
		uint bitstoget = CHAR_BIT - uint( dsBitMark );
		uint8_t bitmask = uint8_t( (1U << bitstoget) - 1 );
		
//		newbyte >>= dsBitMark;
		newbyte = uint8_t( newbyte >> dsBitMark );
		
		// strip junk out of old byte, put first bits of new data in
		oldbyte = uint8_t( (oldbyte & ~bitmask) | (newbyte & bitmask) );
		*dest = oldbyte;
		
		// shift out used source bits
		val <<= bitstoget;
		
		// update buffer marks
		bitstowrite -= bitstoget;
		if ( bitstowrite >= 0 )
			{
			dsBitMark = 0;
			++dsMark;
			++dest;
			
			if ( reinterpret_cast<char *>( dest ) >= dsData + dsMax )
				{
				dsResult = kSpoolErrFull;
				break;
				}
			}
		else
			{
			// -bitstowrite is how many bits are still available in current byte
			dsBitMark = CHAR_BIT + bitstowrite;
			}
		}
	
	dsLimit = dsMark;
}


/*
**	DataSpool::ByteAlignWrite()
**
**	pad bit data in the spool to align to next byte
*/
void
DataSpool::ByteAlignWrite()
{
	if ( noErr == dsResult
	&&   dsBitMark != 0 )
		{
		uint bitsneeded = CHAR_BIT - uint( dsBitMark );
		PutBits( 0, bitsneeded );
		}
}


/*
**	DataSpool::ByteAlignRead()
**
**	move marker to start of next byte
*/
void
DataSpool::ByteAlignRead()
{
	if ( noErr == dsResult
	&&   dsBitMark != 0 )
		{
		++dsMark;
		dsBitMark = 0;
		}
}


/*
**	DataSpool::GetBits()
**
**	get bit data from spool
**	result is filled from the low byte, sign-extended if necessary
*/
int
DataSpool::GetBits( uint numbits )
{
	if ( noErr != dsResult )
		return 0;
	
	// don't get more bits than result can hold
	int bitstoget = int( numbits );
//	const int kBitsPerWord = CHAR_BIT * sizeof(int32_t);
	enum { kBitsPerWord = CHAR_BIT * sizeof(int32_t) };
	if ( bitstoget > kBitsPerWord )
		bitstoget = kBitsPerWord;
	
	// this might read off the end...
//	int32_t value = BigToNativeEndian( * (int32_t *)(dsData + dsMark) );
	int32_t value;
	memcpy( &value, dsData + dsMark, sizeof value );
	value = BigToNativeEndian( value );
	
	// left-shift result to trample bits previously used
	value <<= dsBitMark;
	
	// append more bits if needed
	int bitsgotten = kBitsPerWord - dsBitMark;
	if ( bitstoget > bitsgotten )
		{
		// get next byte (the one following the 32 bits of 'value' we just read)
		uint8_t byte = * (uint8_t *)(dsData + dsMark + sizeof value);
		
		uint bitmask = (1U << dsBitMark) - 1;
//		byte >>= dsBitMark;
		byte = uint8_t( byte >> dsBitMark );
		
		// fill it in
		value |= (byte & bitmask);
		}
	
	// right-shift into postion, sign-extending properly
	value >>= (kBitsPerWord - bitstoget);
	
	// adjust marks
	int bitsleft = CHAR_BIT - dsBitMark;
	if ( bitstoget < bitsleft )
		{
		// bits remain in first partial byte
		dsBitMark += bitstoget;
		}
	else
		{
		bitstoget -= bitsleft;
		dsMark += 1U + uint(bitstoget / CHAR_BIT);
		dsBitMark = bitstoget % CHAR_BIT;
		}
	
	// check for errors
	if ( dsMark > dsLimit )
		dsResult = kSpoolErrEmpty;
	
	return value;
}

#pragma mark -- SafeString --


/*
**	SafeString::SafeString()
**
**	constructor
*/
SafeString::SafeString() :
	ssPtr( &gSafeStringZero ),
	ssSize( 1 ),
	ssMax()
{
}


/*
**	SafeString::~SafeString()
**
**	destructor
*/
SafeString::~SafeString()
{
	// don't delete the original constant string!
	if ( ssMax > 0 )
		delete[] ssPtr;
}


/*
**	SafeString::Clear()
**
**	clear the string
*/
void
SafeString::Clear()
{
	if ( ssMax > 0 )
		{
		*ssPtr = '\0';
		ssSize = 1;
		}
}


/*
**	SafeString::ExpandTo()
**
**	grow the backing store
**	[would be easier if we could use vector<char>, or even realloc()]
*/
DTSError
SafeString::ExpandTo( size_t newsize )
{
	// no-op if already large enough
	if ( newsize <= ssMax )
		return noErr;
	
	char * newptr = NEW_TAG("SafeString") char[ newsize ];
	if ( not newptr )
		{
#ifdef DEBUG_VERSION
		ShowMessage( "Utils: SafeString unable to expand." );
#endif
		return memFullErr;
		}
	
	// move old data to new store
	memcpy( newptr, ssPtr, ssSize );
	
	// trash old store
	if ( ssMax > 0 )
		delete[] ssPtr;
	
	// point to new
	ssPtr  = newptr;
	ssMax  = newsize;
	
	return noErr;
}


/*
**	SafeString::Append()
**
**	append a string
*/
void
SafeString::Append( const char * str )
{
	size_t oldsize = ssSize;
	size_t newsize = oldsize + std::strlen( str );
	DTSError result = ExpandTo( newsize );
	if ( noErr == result )
		{
		std::strcpy( ssPtr + oldsize - 1, str );
		ssSize = newsize;
		}
}


/*
**	SafeString::Append()
**
**	append a number
*/
void
SafeString::Append( int value )
{
	char buff[ 16 ];
	snprintf( buff, sizeof buff, "%d", value );
	Append( buff );
}


/*
**	SafeString::Format()
**
**	append a formatted string
*/
void
SafeString::Format( const char * format, ... )
{
	va_list params;
	va_start( params, format );
	VFormat( format, params );
	va_end( params );
}


/*
**	SafeString::VFormat()
**
**	append a formatted string
*/
void
SafeString::VFormat( const char * format, va_list params )
{
	// check the length in a dummy buffer
	// (remember that vsnprintf() returns the number of bytes it _would_ have written
	// into a sufficiently large buffer, NOT the number _actually_ written.)
	char dummybuffer[4];
	va_list saved_params;
	va_copy( saved_params, params );
	int len = vsnprintf( dummybuffer, sizeof dummybuffer, format, params );
	
	// expand the safe string
	size_t oldsize = ssSize;
	size_t newsize = oldsize + uint( len );
	DTSError result = ExpandTo( newsize );
	if ( noErr == result )
		{
		vsnprintf( ssPtr + oldsize - 1, uint( len ) + 1, format, saved_params );
		ssSize = newsize;
		}
	va_end( saved_params );
}


/*
**	SafeString::Read()
**
**	load from a keyfile datum
*/
DTSError
SafeString::Read( DTSKeyFile * file, DTSKeyType aType, DTSKeyID id )
{
	size_t sz;
	DTSError result = file->GetSize( aType, id, &sz );
	if ( noErr == result )
		result = ExpandTo( sz );
	if ( noErr == result )
		result = file->Read( aType, id, ssPtr, sz );
	
	if ( noErr != result )
		Clear();
	else
		ssSize = sz;
	
	return result;
}

#pragma mark -- CLImage --

// FIXME: Rethink endianness issues here

#if ! CL_SERVER
/*
**	DoChecksum()
**
**	perform the adler32 calculation (well, approximately)
*/
static void
DoChecksum( const void * ptr, size_t len, uint32_t& ioS1, uint32_t& ioS2 )
{
	const uint32_t BASE = 65521;	// must be prime
	
	const uchar * p = static_cast<const uchar *>( ptr );
	uint32_t s1 = ioS1;
	uint32_t s2 = ioS2;
	
	for ( const uchar * end = p + len;  p < end;  ++p )
		{
		s1 = (s1 + *p) % BASE;
		s2 = (s2 + s1) % BASE;
		}
	
	ioS1 = s1;
	ioS2 = s2;
}


/*
**	CLImage::CalculateChecksum()
**	
**	for use when bits and colors aren't available (in editor, not in client)
*/
DTSError
CLImage::CalculateChecksum( DTSKeyFile * file, uint32_t * sum )
{
	void * bits = nullptr;
	uchar colors[ 256 ];
	LightingData light;
	LightingData * pLight = nullptr;
	
	// load bits
	DTSError result = TaggedReadAlloc( file, kTypePictureBits,
						cliPictDef.pdBitsID, bits, "LoadImageBits" );
#if 0 && DTS_LITTLE_ENDIAN
	// disabled, cos we always want to calculate the sum of the _unswapped_ BitsDef
	if ( noErr == result )
		{
		// only the first 3 fields need swapped -- cf. LoadImage()
		struct __attribute__((__packed__)) BitsDefTmp
			{
			int16_t bdHt;
			int16_t bdWd;
			int16_t bdPt;	// packtype
			// reserved, data...
			} * bitsTemp = static_cast<BitsDefTmp *>( bits );
		bitsTemp->bdHt = BigToNativeEndian( bitsTemp->bdHt );
		bitsTemp->bdWd = BigToNativeEndian( bitsTemp->bdWd );
		bitsTemp->bdPt = BigToNativeEndian( bitsTemp->bdPt );
		}
#endif  // DTS_LITTLE_ENDIAN
	
	// load colors (endian-neutral)
	if ( noErr == result )
		result = file->Read( kTypePictureColors, cliPictDef.pdColorsID,
					colors, sizeof colors );
	
	// load optional lighting data
	if ( noErr == result
	&&	 cliPictDef.pdLightingID )
		{
		result = file->Read( kTypeLightingData, cliPictDef.pdLightingID,
					&light, sizeof light );
		if ( noErr == result )
			{
			BigToNativeEndian( &light );
			pLight = &light;
			}
		}
	
	// calculate checksum
	if ( noErr == result )
		result = CalculateChecksum( file, bits, colors, pLight, sum );
	
	delete[] reinterpret_cast<char *>( bits );
	
	return result;
}


/*
**	CLImage::CalculateChecksum()
**
**	Calculate slightly encrypted adler32 checksum for image data.
**	The image is expected to have a valid (and native-endian) cliPictDef;
**	likewise the bitsDef header data (6 bytes) should be native, as
**	LoadImage() would have left it.  The color data never needs swapping.
**	The LightingData (which is optional) must also already be native endian,
**	if present.
*/
DTSError
CLImage::CalculateChecksum( DTSKeyFile * file,
			const void * bits,
			const void * colors,
			const void * ilInfo,
			uint32_t * sum )
{
	size_t len;
	*sum = 0;
	
	// adler32 checksum accumulators: s1 is sum of bytes, s2 is sum of sums
	uint32_t s1 = 1;		// ensures that checksum depends on length of sequence
	uint32_t s2 = 0;
	
	if ( not bits
	||	 not colors
//	||	 not ilInfo	// NULL light data just means "don't checksum it"
	||	 not file )
		{
		return -1;
		}
	
	DTSError result = file->GetSize( kTypePictureBits, cliPictDef.pdBitsID, &len );
	if ( noErr == result )
		{
		// bits
		DoChecksum( bits, len, s1, s2 );
		result = file->GetSize( kTypePictureColors, cliPictDef.pdColorsID, &len );
		}
	
	if ( noErr == result )
		{
		// colors
		DoChecksum( colors, len, s1, s2 );
		
		// lighting info
		if ( ilInfo )
			{
#if DTS_BIG_ENDIAN
			DoChecksum( ilInfo, sizeof(LightingData), s1, s2 );
#else
			LightingData ldTmp = * static_cast<const LightingData *>( ilInfo );
			BigToNativeEndian( &ldTmp );
			
			DoChecksum( &ldTmp, sizeof ldTmp, s1, s2 );
#endif
			}
		
		// PictDef
		// but not the checksum itself, nor the irrelevant animation frames.
		//
		// It's kind of regrettable that the sum is defined to include the
		// bitsID, colorsID, and lightingID fields; if those were ignored
		// then the checksum for any arbitrary picture would be constant:
		// it would not vary even if the picture's bits or colors records should
		// ever happen to receive new DTSKeyIDs. For example, in the CLEditor,
		// if you copy and paste/replace an image on top of itself, its
		// colors, bits, & lighting may well be stored under new KeyIDs
		// (thanks to the simple-mindedness of DTSKeyFile::GetUnusedID()),
		// which means that the otherwise identical image will nonetheless be
		// given an entirely new checksum value.
		
		len = offsetof( PictDef, pdAnimFrameTable )
				 + cliPictDef.pdNumAnims * sizeof cliPictDef.pdAnimFrameTable[0];
#if DTS_BIG_ENDIAN
		uint32_t savesum = cliPictDef.pdChecksum;
		cliPictDef.pdChecksum = 0;
		DoChecksum( &cliPictDef, len, s1, s2 );
		cliPictDef.pdChecksum = savesum;
		
		// image ID, so people can't just copy one from elsewhere in the file
		DoChecksum( &cliPictDefID, sizeof cliPictDefID, s1, s2 );
#else
		// must byteswap this PictDef back to BE before summing it
		PictDef tempPD = cliPictDef;
		NativeToBigEndian( &tempPD );
		tempPD.pdChecksum = 0;
		DoChecksum( &tempPD, len, s1, s2 );
		
		// image ID, so people can't just copy one from elsewhere in the file
		DTSKeyID tempID = NativeToBigEndian( cliPictDefID );
		DoChecksum( &tempID, sizeof tempID, s1, s2 );
#endif  // ! DTS_BIG_ENDIAN
		
#if DTS_BIG_ENDIAN
		*sum = static_cast<uint32_t>( (s2 << 16) + (s1 & 0x0FFFF) );
		SimpleEncrypt( sum, sizeof *sum );
#else
		uint32_t tsum = NativeToBigEndian( static_cast<uint32_t>(
							(s2 << 16) + (s1 & 0x0FFFF) ) );
		SimpleEncrypt( &tsum, sizeof tsum );
		*sum = BigToNativeEndian( tsum );
#endif
		}
	
	return result;
}
#endif  // ! CL_SERVER


/*
**	SimpleEncrypt()
**
**	xor the data with a known string
*/
void
SimpleEncrypt( void * data, size_t sz )
{
	static const char XOrStr[] = "\x3C\x5A\x69\x93\xA5\xC6";
	
	char * dataPtr = static_cast<char *>( data );
	const char * xorPtr  = XOrStr;
	for (  ;  sz > 0;  --sz )
		{
		char value = *xorPtr;
		if ( 0 == value )
			{
			xorPtr = XOrStr;
			value = *xorPtr;
			}
		++xorPtr;
		value ^= *dataPtr;
		*dataPtr++ = value;
		}
}


/*
**	SpoolFramePicture()
**
**	pack and spool the picture frame data
*/
void
SpoolFramePicture( DataSpool * spool, DTSKeyID pictID, int horz, int vert )
{
	if ( pictID > kPictFrameIDLimit
	||	 pictID < 0 )
		{
#ifdef DEBUG_VERSION
		ShowMessage( "\xA5" "SharedUtils: picture id out of range." );	// bullet
#endif
		pictID = 0;
		}
	
	if ( horz > kPictFrameCoordLimit )
		horz = kPictFrameCoordLimit;
	if ( horz < - kPictFrameCoordLimit )
		horz = - kPictFrameCoordLimit;
	
	if ( vert > kPictFrameCoordLimit )
		vert = kPictFrameCoordLimit;
	if ( vert < - kPictFrameCoordLimit )
		vert = - kPictFrameCoordLimit;
	
#if BITWISE_IMAGE_SPOOL
	spool->PutBits( pictID, kPictFrameIDNumBits );
	spool->PutBits( horz, kPictFrameCoordNumBits );
	spool->PutBits( vert, kPictFrameCoordNumBits );
#else
	uint32_t packData = 
		  ( static_cast<uint32_t>( pictID ) << (2 * kPictFrameCoordNumBits) )
		| ( horz & kPictFrameCoordMask ) << kPictFrameCoordNumBits
		| ( vert & kPictFrameCoordMask );
	
	spool->PutNumber( packData, kSpoolUnsignedLong );
#endif	// BITWISE_IMAGE_SPOOL
}


/*
**	UnspoolFramePicture()
**
**	read and unpack the picture frame data
*/
void
UnspoolFramePicture( DataSpool * spool, DTSKeyID& pictID, int& horz, int& vert )
{
#if BITWISE_IMAGE_SPOOL
	UnspoolFramePicture( spool, pictID, horz, vert,
		kPictFrameIDNumBits, kPictFrameCoordNumBits );
#else
	uint32_t packData = spool->GetNumber( kSpoolUnsignedLong );
	
	// ID, unsigned
	pictID = packData >> ( 32 - kPictFrameIDNumBits );
	packData <<= kPictFrameIDNumBits;
	
	// H & V, signed
	horz   =  int(packData) >> ( 32 - kPictFrameCoordNumBits );
	packData <<= kPictFrameCoordNumBits;
	vert   =  int(packData) >> ( 32 - kPictFrameCoordNumBits );
#endif
}


#if BITWISE_IMAGE_SPOOL
/*
**	UnspoolFramePicture()
**
**	read and unpack the picture frame data given the bit sizes
*/
void
UnspoolFramePicture( DataSpool * spool, DTSKeyID& pictID, int& horz, int& vert,
					 int idBits, int coordBits )
{
	pictID	= DTSKeyID( spool->GetBits( uint( idBits ) ) );
	horz	= spool->GetBits( uint( coordBits ) );
	vert	= spool->GetBits( uint( coordBits ) );
}
#endif	// BITWISE_IMAGE_SPOOL

#if ! CL_SERVER

#pragma mark -- Misc --


/*
**	GetUnusedIDNotZero()
**	kinda like DTSKeyFile::GetUnusedID()
**	but don't return 0
*/
DTSError
GetUnusedIDNotZero( DTSKeyFile * file, DTSKeyType inType, DTSKeyID& outID )
{
	// GetUnusedID() will return an ID of 0 if there aren't
	// any records of this type yet
	DTSKeyID test;
	DTSError result = file->GetUnusedID( inType, &test );
	if ( noErr == result )
		{
		if ( test != 0 )
			{
			// wow, that was easy
			outID = test;
			return noErr;
			}
		
		// try 1, 2, 3 ...
		for (;;)
			{
			++test;
			result = file->Exists( inType, test );
			if ( -1 == result )
				{
				// -1 means the record doesn't exist, so let's use this ID.
				outID = test;
				return noErr;
				}
			if ( noErr != result )
				break;
			}
		}
	return result;
}


/*
**	RemoveCrapFromImagesFile()
**
**	remove unused record types from the images file
*/
DTSError
RemoveCrapFromImagesFile( DTSKeyFile * file )
{
	// the types -not- to be removed:
	static const DTSKeyType ciTypes[] = {
		kTypePictureBits, kTypePictureColors, kTypeClientItem,
		kTypeLightingData, kTypeLayout, kTypePictureDefinition, kTypeVersion,
		0 };
		
	return RemoveCrapFromFile( file, ciTypes );
}


/*
**	RemoveCrapFromSoundsFile()
**
**	remove unused record types from the sounds file
*/
DTSError
RemoveCrapFromSoundsFile( DTSKeyFile * file )
{
	// keep these
	static const DTSKeyType csTypes[] = { kTypeVersion, kTypeSound, 0 };
	
	return RemoveCrapFromFile( file, csTypes );
}
#endif  // ! CL_SERVER


/*
**	Remove1Type()
**
**	remove one type from the file if it is not in the good list
*/
static DTSError
Remove1Type( DTSKeyFile * file, DTSKeyType aType, const DTSKeyType * goodTypes )
{
	for(;;)
		{
		DTSKeyType test = *goodTypes++;
		if ( 0 == test )
			break;
		if ( test == aType )
			return noErr;
		}
	
	return Delete1Type( file, aType );
}


/*
**	RemoveCrapFromFile()
**
**	remove records from the key file that don't belong there
*/
DTSError
RemoveCrapFromFile( DTSKeyFile * file, const DTSKeyType * goodTypes )
{
	long count;
	DTSError result = file->CountTypes( &count );
	if ( noErr == result )
		{
		for(;;)
			{
			if ( count <= 0 )
				break;
			--count;
			DTSKeyType aType;
			result = file->GetType( count, &aType );
			if ( noErr != result )
				break;
			result = Remove1Type( file, aType, goodTypes );
			}
		}
	
	return result;
}


/*
**	Delete1Type()
**
**	remove one type from the file
*/
DTSError
Delete1Type( DTSKeyFile * file, DTSKeyType aType )
{
	long count;
	DTSError result = file->Count( aType, &count );
	if ( noErr == result )
		{
#ifndef DTS_Unix	// be hyper-cautious on production server: no fast mode
		int oldMode = file->GetWriteMode();
		file->SetWriteMode( kWriteModeFaster );
#endif
		for ( ;  count > 0;  --count )
			{
			DTSKeyID id;
			result = file->GetID( aType, 0, &id );
			if ( noErr != result )
				break;
			result = file->Delete( aType, id );
			if ( noErr != result )
				break;
			}
#ifndef DTS_Unix
		file->SetWriteMode( oldMode );
#endif
		}
	
	return result;
}


/*
**	CompactName()
**
**	remove all non-letters
**	convert to upper case
*/
void
CompactName( const char * src, char * dst, size_t destsize )
{
	char * limit = dst + destsize - 1;
	for(;;)
		{
		int ch = * (const uchar *) src++;
		if ( 0 == ch )
			break;
		if ( isalpha( ch ) )
			{
			*dst++ = char( toupper( ch ) );
			if ( dst >= limit )
				break;
			}
		}
	*dst = '\0';
}


/*
**	ReadFileVersion()
**
**	read and adjust a file version number
*/
DTSError
ReadFileVersion( DTSKeyFile * file, int32_t * version )
{
	int32_t v = 0;
	DTSError result = file->Read( kTypeVersion, 0, &v, sizeof v );
	
	v = BigToNativeEndian( v );
	
	if ( v <= 0xFF )
		v <<= 8;
	
	*version = v;
	
	return result;
}


#if defined( MULTILINGUAL )
/*
**	ExtractLangText()
**
**	extract a single language's escape string, for multilingual servers
**	sets text pointer to beginning of correct language (no null termination)
**	returns length of correct language's text (not including null termination,
**	if any). if genderout is not null, sets it to linguistic gender constant.
**	based on function written by torx@arindal.com
*/
int
ExtractLangText( const char ** text, int langid, int * genderout )
{
	int gender = kLangGenderNeuter;
	if ( genderout )
		*genderout = gender;
	
	if ( not text || not *text )
		return 0;
	
	// search for start of the language we're looking for
	const char * p;
	for ( p = *text ; *p; ++p )
		{
		// speed things up for langid 0 (english), must always be first in string
		if ( 0 == langid )
			break;
		
		// get next %
		const char * escp = std::strchr( p, '%' );
		if ( not escp )
			{
			// no % found
			p = nullptr;
			break;
			}
		
		// what is following the %?
		p = escp + 1;
		if ( 0 == *p )
			break;
		
		if ( '%' == *p )
			{
			/* two %? - special sequence - skip.
			   Note - do not replace the %% by %, as this will
			   be done by the EscapeText() function later */
			continue;
			}
		
		if ( 'l' == *p )
			{
			// looks like a language tag!
			// ok, what's following?
			++p;
			
			// an optional gender description?
			if ( 'm' == *p )
				{
				gender = kLangGenderMasculine;
				++p;
				}
			else
			if ( 'f' == *p )
				{
				gender = kLangGenderFeminine;
				++p;
				}
			else
			if ( 'n' == *p )
				{
				gender = kLangGenderNeuter;
				++p;
				}
			
			// (followed by) a language id?
			if ( isdigit( *p ) )
				{
				int foundlang = *p - '0';
				
				// is it the language we're looking for?
				if ( foundlang == langid )
					{
					// yes, set pointer to following char and quit looking
					++p;
					break;
					}
				}
			// something else?
			else
				{
				// this is an error, so flag p as bad and quit looking
				p = nullptr;
				break;
				}
			}
		}
	
	const char * match;
	if ( p && *p != 0 )
		{
		// point to the start of the string we wanted
		match = p;
		}
	else
		{
		// not found, use first part from text as default
	  	match = *text;
		gender = kLangGenderNeuter;
		}
	
	// now we know where our string starts, but where does it end?
	for ( p = match ; *p; ++p )
		{
		if ( '%' == *p )
			{
			// what's after the %?
			++p;
			
			// ending % is not valid, but we'll let EscapeText() worry about it
			if ( 0 == *p )
				break;
			
			if ( 'l' == *p )
				{
				// ok, another language tag, we are done
				// skip back to the % that ends our language
				--p;
				break;
				}
			}
		}
	
	if ( genderout )
		*genderout = gender;
	
	// if we had neither a language match nor default text, use the whole string
	ptrdiff_t len;
	if ( match == *text && p == match )
		len = strlen( match );
	else
		len = p - match;
	
	*text = match;
	
	return static_cast<int>( len );
}
#endif  // MULTILINGUAL

