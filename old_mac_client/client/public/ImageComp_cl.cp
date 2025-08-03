/*
**	ImageComp_cl.cp		ClanLord Shared
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

#include "DatabaseTypes_cl.h"
#include "Public_cl.h"


/*
**	Entry Routines
*/
/*
DTSError	LoadImage( CLImage * image, DTSKeyFile * file, int nColr, const uchar * custColrs );
DTSError	TaggedReadAlloc( DTSKeyFile *, DTSKeyType, DTSKeyID, void *&, const char * );
 
DTSColor	GetCLColor( uint8_t idx );
const RGBColor8 *	GetCLColorTable();
CGImageRef	CGImageCreateFromDTSImage( const DTSImage * img, uint32_t pdFlags );
DTSError	LoadImage( const PictDef&, CGImageRef *, DTSKeyFile *, int = 0, const uchar * = nullptr );
DTSError	LoadImageX( DTSKeyID, CGImageRef *, DTSKeyFile *, bool, int, const uchar * );

// editor only:
DTSError	StoreImage( CLImage * image, DTSKeyFile * file );
DTSError	RemoveImage( DTSKeyFile * file, DTSKeyID id );
DTSError	GetImageSize( DTSKeyFile * file, DTSKeyID id, int * width, int * height );
DTSError	GetImageNumColors( DTSKeyFile * file, DTSKeyID bitsid, int * numColors );
*/

/**************************************************************************
we need to compress images pretty much as small as possible. we're going to
make the assumption that some of the images will only differ by transforming
the colors. so when compressing a picture we will store the bits and the
color table separately. the color table is sorted in the order that the
colors appear in the image. we can also reuse the color tables. when we
remove an image from the file we must be careful to only remove the bits and
the table when it is no longer in use. we are not currently going to worry
too much about finding other transformations like rotations and reflections.
**************************************************************************/

/*
**	brain damage
**	there must be a better way to do this
**
**	... which is timmerese for:
**
**		This file is shared by the client and editor projects. But only the editor ever
**		needs to modify stuff on disk, via these bottlenecks (which form a part of the
**		editor's Undo machinery).
**		So: the editor will provide actual working definitions to match these declarations;
**		client will not. But since those latter apps never call StoreImage() or RemoveImage(),
**		the linker will dead-strip that function, and therefore won't need to
**		have definitions for WriteData() / DeleteData() available.
**
**		The alternative would have been to #ifdef out StoreImage(), RemoveImage(), and all
**		their dependencies for the client app.  And indeed, we're moving in that
**		direction already; viz. the use of the "CL_SERVER" symbol in Utilities_cl.cp.
**
**	Since the gcc linker isn't as good as the Metrowerks one when it comes to dead-
**	stripping unused code, let's implement that #ifdef solution.
*/
#if 'CLEd' == COMPONENT_SIGNATURE	// editor
# define BUILDING_CL_EDITOR		1
#else
# undef BUILDING_CL_EDITOR
#endif

#if BUILDING_CL_EDITOR
DTSError	WriteData( int file, DTSKeyType ttype, DTSKeyID id, size_t sz, const void * data );
DTSError	DeleteData( int file, DTSKeyType ttype, DTSKeyID id );
#endif


/*
**	Internal Definitions
*/


// these are the bit-packing schemes we support
enum
{
	kPackType1			// original: homegrown run-length encoding
};


#pragma pack( push, 2 )
struct BitsDef
{
	int16_t		bdHeight;		// pixels
	int16_t		bdWidth;		// pixels
	int16_t		bdPackType;		// one of the above PackTypes
	int16_t		bdReserved;		// this aligns the data to 32 bits
	uchar		bdData[2];		// variably-sized
};
#pragma pack( pop )


// for kPackType1:
// the first byte is log2( number of bits per pixel)
// the second byte is log2( run length )
// the remainder is the run-length-encoded data (see CompressBits1())
// within which we have two "run types":
enum
{
	kPackRepetition = 0,	// run of repeated values
	kPackRun		= 1		// run of non-repeating values
};


/*
**	Internal Routines
*/
static DTSError	DecompressImage( uchar *, const BitsDef *, size_t, const uchar * );

#if BUILDING_CL_EDITOR
static uint		BuildColors( const DTSImage *, uchar[256] );
static DTSError	StoreColors( DTSKeyFile *, const uchar[256], size_t, DTSKeyID * );
static DTSError	CompressBits( const DTSImage *, BitsDef **, size_t *, const uchar[256], int );
static DTSError	StoreBits( DTSKeyFile *, const BitsDef *, size_t, DTSKeyID * );
static DTSError	StoreLightInfo( DTSKeyFile *, const LightingData *, DTSKeyID * );
#endif  // BUILDING_CL_EDITOR


/*
**	Internal Variables
*/
static const uint32_t *	gD2CPixelTable;


/*
**	LoadImage()
**
**	load a compressed image from the key file
*/
DTSError
LoadImage( CLImage * image, DTSKeyFile * file,
			int numColors /* = 0 */, const uchar * customColors /* = nullptr */ )
{
	BitsDef * bits = nullptr;
	uchar colors[ 256 ];
	bool checksumErr = false;
	
	// load the picture definition
	DTSError result = file->Read( kTypePictureDefinition, image->cliPictDefID,
		&image->cliPictDef, sizeof image->cliPictDef );
	
	// swap the PDef; load the bits
	if ( noErr == result )
		{
		BigToNativeEndian( &image->cliPictDef );
		result = TaggedReadAlloc( file, kTypePictureBits, image->cliPictDef.pdBitsID,
			(void *&) bits, "LoadImageBits" );
		}
	
	// load the colors
	if ( noErr == result )
		{
		memset( colors, 0, sizeof colors );
		result = file->Read( kTypePictureColors, image->cliPictDef.pdColorsID,
			colors, sizeof colors );
		}
	
	// assume there's no lighting data
	bool bHadLights = false;
	memset( &image->cliLightInfo, 0, sizeof image->cliLightInfo );
	
	// but if there is, load it
	if ( noErr == result
	&&   image->cliPictDef.pdLightingID != 0 )
		{
		result = file->Read( kTypeLightingData, image->cliPictDef.pdLightingID,
			&image->cliLightInfo, sizeof image->cliLightInfo );
		if ( noErr == result )
			{
			bHadLights = true;
			BigToNativeEndian( &image->cliLightInfo );
			}
		}
	
	// verify the checksum
	if ( not (image->cliPictDef.pdFlags & kPictDefFlagNoChecksum) )
		{
		uint32_t sum;
		if ( noErr == result )
			result = image->CalculateChecksum( file, bits, colors,
							bHadLights ? &image->cliLightInfo : nullptr,
							&sum );
		
		if ( noErr == result
		&&	 sum != image->cliPictDef.pdChecksum )
			{
			checksumErr = true;
			}
		}
	
	// initialize the image
	if ( noErr == result )
		{
#if DTS_LITTLE_ENDIAN
		bits->bdHeight   = EndianS16_BtoN( bits->bdHeight );
		bits->bdWidth    = EndianS16_BtoN( bits->bdWidth );
		bits->bdPackType = EndianS16_BtoN( bits->bdPackType );
		// bdReserved: unused, don't bother to swap
		// bdData[] -- will be handled elsewhere
#endif  // DTS_LITTLE_ENDIAN
		
		result = image->cliImage.Init( nullptr, bits->bdWidth, bits->bdHeight, 8 );
		}
	
	// allocate memory for the image
	if ( noErr == result )
		result = image->cliImage.AllocateBits();
	
	// overwrite the default colors with the custom colors
	// decompress the bits into the image
	if ( noErr == result )
		{
		if ( image->cliPictDef.pdFlags & kPictDefCustomColors )
			memcpy( colors, customColors, numColors );
		result = DecompressImage( static_cast<uchar *>( image->cliImage.GetBits() ),
					bits, image->cliImage.GetRowBytes(), colors );
		}
	
	delete[] reinterpret_cast<char *>( bits );
	
	if ( checksumErr && noErr == result )
		result = kImageChecksumError;
	
	return result;
}


#if BUILDING_CL_EDITOR
/*
**	StoreImage()
**
**	compress and store an image in the key file
*/
DTSError
StoreImage( CLImage * image, DTSKeyFile * file )
{
	// build the color table
	uchar colors[ 256 ];
	memset( colors, 0, sizeof colors );
	uint nColors = BuildColors( &image->cliImage, colors );
	
	// store the color table (looking for duplicates)
	DTSKeyID tmpID( 0 );
	DTSError result = StoreColors( file, colors, nColors * sizeof(colors[0]), &tmpID );
	if ( noErr == result )
		image->cliPictDef.pdColorsID = tmpID;
	
	// then compress the bits
	size_t sz;
	BitsDef * bits = nullptr;
	if ( noErr == result )
		{
		result = CompressBits( &image->cliImage, &bits, &sz, colors, nColors );
		
#if defined( DEBUG_VERSION )
		DTSRect imageBounds;
		image->cliImage.GetBounds( &imageBounds );
		if ( sz >= (uint)( imageBounds.rectBottom * imageBounds.rectRight ) )
			VDebugStr( "Huh?" );
#endif
		}
	
	// store the bits (again, look for duplicates)
	if ( noErr == result )
		{
		tmpID = 0;
		result = StoreBits( file, bits, sz, &tmpID );
		if ( noErr == result )
			image->cliPictDef.pdBitsID = tmpID;
		}
	
	// then store the light info, if any
	bool bHasLights = image->HasLightingData();
	if ( noErr == result  &&  bHasLights )
		{
		tmpID = image->cliPictDef.pdLightingID;
		result = StoreLightInfo( file, &image->cliLightInfo, &tmpID );
		image->cliPictDef.pdLightingID = tmpID;
		}
	
	// at this point we need to replace the 'colors' table with the real one, from the disk.
	// Here's why: suppose the image we're adding has only a few colors, and that StoreColors()
	// is able to locate a (larger) pre-existing color table with a matching prefix: that is,
	// a shared table. The PDef's colors-ID field will thus point to that record. When we
	// call CalculateChecksum(), below, it will only examine the _used_ portion of the
	// 'colors' array, plus whatever garbage follows it.  However, later, when we re-load this
	// image, the checksum code will inspect the _entire_ contents of the 'Clrs' record, not
	// just the relevant prefix. Thus the sums won't match: whoops.
	
	// The other, more correct solution would be to take the checksum over only the _used_
	// portion of the color table. But while that's easy to do here (after calling BuildColors(), 
	// the 'nColors' variable tells exactly what to look at), it's very much harder in LoadImage().
	// In fact the only way I can think of is to, first, decompress the image; second, scan the
	// entire bits table to find the highest color-index value therein; and only then, third,
	// verify the checksum.  Ouch!
	
	if ( noErr == result )
		result = file->Read( kTypePictureColors, image->cliPictDef.pdColorsID,
					colors, sizeof colors );
	
	// fill the checksum
	if ( noErr == result )
		{
		result = image->CalculateChecksum( file, bits, colors,
			bHasLights ? &image->cliLightInfo : nullptr,
			&image->cliPictDef.pdChecksum );
		}
	
	// store the picture definition
	if ( noErr == result )
		{
		sz = offsetof( PictDef, pdAnimFrameTable )
			+ image->cliPictDef.pdNumAnims * sizeof image->cliPictDef.pdAnimFrameTable[0];
		
#if DTS_BIG_ENDIAN
		// brain damage -- there must be a better way to do this
		result = WriteData( rClientImagesFREF, kTypePictureDefinition, image->cliPictDefID,
					sz, &image->cliPictDef );
#else
		PictDef tempPD = image->cliPictDef;
		NativeToBigEndian( &tempPD );
		result = WriteData( rClientImagesFREF, kTypePictureDefinition, image->cliPictDefID,
					sz, &tempPD );
#endif
		}
	
	// dispose of the memory used for the compressed bits
	delete[] reinterpret_cast<char *>( bits );
	
	return result;
}


/*
**	RemoveImage()
**
**	remove an image from the key file
*/
DTSError
RemoveImage( DTSKeyFile * file, DTSKeyID id )
{
	long nRecs;
	PictDef pd;
	DTSKeyID bitsID;
	DTSKeyID colsID = 0;
	DTSKeyID lightsID = 0;
	bool killBits = true;
	bool killCols = true;
	bool killLights = true;
	
	// check if the darn thing even exists
	DTSError result = file->Exists( kTypePictureDefinition, id );
	if ( noErr != result )
		return noErr;
	
	// load it to find the IDs of the bits, colors, and lighting
	memset( &pd, 0, sizeof pd );	// clean slate
	if ( noErr == result )
		{
		result = file->Read( kTypePictureDefinition, id, &pd, sizeof pd );
		if ( noErr == result )
			BigToNativeEndian( &pd );
		}
	
	// delete it from the key file
	if ( noErr == result )
		{
		// brain damage -- there must be a better way to do this
		result = DeleteData( rClientImagesFREF, kTypePictureDefinition, id );
		// old way
//		result = file->Delete( kTypePictureDefinition, id );
		}
	
	// count how many picture definitions there are remaining
	if ( noErr == result )
		result = file->Count( kTypePictureDefinition, &nRecs );
	
	// search all other pictdefs to see if any of them use the same bits, colors, or lights
	if ( noErr == result )
		{
		bitsID = pd.pdBitsID;
		colsID = pd.pdColorsID;
		lightsID = pd.pdLightingID;
		if ( 0 == lightsID )
			killLights = false;
		
		while ( --nRecs >= 0 )
			{
			// get the ID of the candidate picture def
			// if ( noErr == result )
				result = file->GetID( kTypePictureDefinition, nRecs, &id );
			
			// load the picture def
			if ( noErr == result )
				result = file->Read( kTypePictureDefinition, id, &pd, sizeof pd );
			
			if ( noErr == result )
				{
				// check the bits ID
				if ( EndianS32_BtoN( pd.pdBitsID ) == bitsID )
					killBits = false;
				
				// check the colors ID
				if ( EndianS32_BtoN( pd.pdColorsID ) == colsID )
					killCols = false;
				
				// check the lights ID
				if ( EndianS32_BtoN( pd.pdLightingID ) == lightsID )
					killLights = false;
				}
			else	// bail if there was an error
				break;
			
			// bail if we need to keep all shared data
			if ( not killBits && not killCols && not killLights )
				break;
			}
		}
	
	// delete the bits if it exists
	if ( noErr == result )
		{
		if ( killBits
		&&	 noErr == file->Exists( kTypePictureBits, bitsID ) )
			{
			// brain damage -- there must be a better way to do this
			result = DeleteData( rClientImagesFREF, kTypePictureBits, bitsID );
			// old way
//			result = file->Delete( kTypePictureBits, bitsID );
			}
		}
	
	// delete the colors if it exists
	if ( noErr == result )
		{
		if ( killCols
		 &&	 noErr == file->Exists( kTypePictureColors, colsID ) )
			{
			// brain damage -- there must be a better way to do this
			result = DeleteData( rClientImagesFREF, kTypePictureColors, colsID );
			// old way
//			result = file->Delete( kTypePictureColors, colsID );
			}
		}
	
	// ditto for lights
	if ( noErr == result )
		{
		if ( killLights
		&&	 lightsID != 0
		&&	 noErr == file->Exists( kTypeLightingData, lightsID ) )
			{
			result = DeleteData( rClientImagesFREF, kTypeLightingData, lightsID );
			}
		}
	
	return result;
}
#endif  // BUILDING_CL_EDITOR


#pragma mark -
#pragma mark Internal Routines

#if 0
/*
**	GetBits()
**
**	inline routine
**	fetch variably-sized chunks of bits from a stream of 32-bit words.
**	WARNING: the input 'Bits' data may not be an exact multiple of 4 bytes in length;
**	in that case, we will read 1 to 3 bytes beyond the official end of the data.
**	The algorithm ensures that those spurious bytes never contaminate the output,
**	so no harm is done. However, it's not inconceivable that someday the 'Bits' data
**	might be mmap(2)'ed instead of ReadAlloc()'ed; if so, that very last read may
**	provoke [the moral equivalent of] a bus error.
**	The simplest solution would be, when mmap()'ing a 'Bits' record, to enlarge the
**	mapped region to a multiple of 4 bytes in length. That could still fail, in the
**	highly unlikely scenario where the 'Bits' record of interest happens to be at the very
**	end of the Images file.
*/
static /*inline*/
uint32_t
GetBits( int bitsToGet, const uint32_t *& srcPtr, uint32_t& srcData, int& srcBits )
{
	// JSS Changed
	uint32_t outBits = srcData >> (32 - bitsToGet);
	
	// have we exhausted this 32-bit chunk?
	if ( bitsToGet > srcBits )
		{
		bitsToGet -= srcBits;
		srcBits = 32;
		
		// load the next chunk
		srcData = EndianU32_BtoN( *srcPtr++ );
		outBits |= srcData >> (32 - bitsToGet);
		}
	
	srcData <<= bitsToGet;
	srcBits -= bitsToGet;
	
	return outBits;
}
#endif  // 0


//
// This is an alternate implementation of GetBits(), above.
// This one uses the GCC "statement expressions" extension, whereby a compound statement
// enclosed in parentheses may be used as an expression.
// It only works from inside the body of DecompressImage1[Alpha](), since it refers to
// variables local to those functions.
//
#define GETBITS( numBits2Get )						\
__extension__ ({									\
	int bitsToGet = numBits2Get;					\
	uint outBits = srcData >> (32 - bitsToGet);		\
	if ( bitsToGet > srcBits ) {					\
		bitsToGet -= srcBits;						\
		srcBits = 32;								\
													\
		srcData = EndianU32_BtoN( *srcPtr++ );		\
		outBits |= srcData >> (32 - bitsToGet);		\
	}												\
	srcData <<= bitsToGet;							\
	srcBits -= bitsToGet;							\
	outBits;										\
})


/*
**	DecompressImage1()
**
**	compression is done by using the minimum number of bits per pixel
**	and run length encoding
**	first byte of the data is the bits per pixel
**	second byte is the size of the run length in bits
**	then we have the runs
**	1 bit for run type: 0=repetition, 1=run
**	n bits for run length
**	repetition will then have 1 pixel
**	run will then have a stream of pixels
**	assumes the BitsDef is native-endian at this point (i.e. swapped earlier, like at read)
*/
static DTSError
DecompressImage1( uchar * dstPtr, const BitsDef * bits, size_t rowBytes, const uchar * colors )
{
	// local variables
	int ht = bits->bdHeight;
	const int wd = bits->bdWidth;
	const int rowOffset = rowBytes - wd * sizeof( *dstPtr );
	const uint32_t * srcPtr = reinterpret_cast<const uint32_t *>( bits->bdData );
	
	// calculate the limit for debugging
	const uchar * dstLimit = dstPtr + rowBytes * ht - rowOffset;
	
	// the first byte in the data is the number of bits per pixel
	// the second byte is the size of the run length in bits
	const uint bitsPerPixel = bits->bdData[ 0 ];
	const int runBits = bits->bdData[ 1 ];
	
	// since our "pixels" are actually indices into the 256-color CLUT,
	// they cannot be larger than 8 bits wide. Images compressed via StoreImage() will always
	// fulfil this requirement, but I guess it can't hurt to be wary: someone might have
	// maliciously meddled with the contents of their images file.
	__Check( bitsPerPixel <= 8 );
	if ( bitsPerPixel > 8 )
		return -1;	// we should have a dedicated kImageIsCorruptError
	
	// get the first 32-bit chunk of data
	uint32_t srcData = EndianU32_BtoN( *srcPtr++ );
	srcData <<= 16;		// we've already read the first 2 bytes
	int srcBits = 16;	// (bitsPerPixel and runBits)
	
	// for each row in the destination
	for ( ;  ht > 0;  --ht )
		{
		// for each pixel in the destination
		for ( int nPixels = wd;  nPixels > 0;  )
			{
			// get 1 bit
			if ( 0 == srcBits )
				{
				// reload the accumulator
				srcData = EndianU32_BtoN( *srcPtr++ );
				srcBits = 32;
				}
			
			// get 1 bit, to tell us what kind of run we have
			int32_t runType = srcData & 0x80000000;
			srcData <<= 1;
			--srcBits;
			
			// get the run length
//			uint runLength = GetBits( runBits, srcPtr, srcData, srcBits );
			uint runLength = GETBITS( runBits );
			
			// error check
			if ( dstPtr + runLength > dstLimit )
				return 1;
			
			// decrement the number of pixels remaining to be unpacked for this line
			++runLength;
			nPixels -= runLength;
			
			uint32_t pixel;
			if ( (kPackRepetition << 31) == runType )
				{
				// write the repeated pixel
//				pixel = GetBits( bitsPerPixel, srcPtr, srcData, srcBits );
				pixel = GETBITS( bitsPerPixel );
				pixel = colors[ pixel ];
#if 1
				memset( dstPtr, pixel, runLength );
				dstPtr += runLength;
#else
				for ( ; runLength > 0;  --runLength )
					*dstPtr++ = pixel;
#endif  // 1
				}
			else
				{
				// write the run of non-repeating pixels
				
				// JSS Change
				/*
				 * Special case 8 bit color to block read.
				 *
				 * Is srcBits 8, 16, 24, or 32?
				 * % is slow, and (n & (b-1)) == n % b if b is a power of 2.
				 *
				 * Also get 4 bit on even boundary
				 */
				if ( ( 8 == bitsPerPixel && not (srcBits & 0x07) )
				||   ( 4 == bitsPerPixel && not (srcBits & 0x03) && runLength >= 3 ) )
					{
					// Yes! We are home free.
					
					if ( srcBits & 0x07 )
						{
						// If we have a four bit pixel on a non-char boundary, advance it.
//						pixel = GetBits( bitsPerPixel, srcPtr, srcData, srcBits );
						pixel = GETBITS( bitsPerPixel );
						*dstPtr++ = colors[ pixel ];
						--runLength;
						}
					/*
					 * It would be nice to put this in a subroutine, to
					 * hide srcData and srcBits, but we would have to
					 * allocate an array, unless of course Tim knows there
					 * is a maximum size on pixels returned.
					 */
					
					// First, put srcData back.
					const uchar * p = reinterpret_cast<const uchar *>( --srcPtr );
					
					// In case we are not at an even long boundary.
					if ( srcBits != 32 )
						p += (32 - srcBits) >> 3;
					
					// do the main part of the run. Look, Ma: no GetBits!
					if ( 8 == bitsPerPixel )
						{
						for ( ; runLength > 0;  --runLength )
						    *dstPtr++ = colors[ *p++ ];
						}
					else
						{
						// must be 4 bpp; take 'em 2 at a time
						for ( ; runLength > 1; runLength -= 2, ++p )
							{
							*dstPtr++ = colors[ (*p) >> 4 ];
							*dstPtr++ = colors[ (*p) & 0x0F ];
							}
						}
					
					// Now make sure srcPtr, srcData, srcBits are set properly.
					
					// Step one past the next read.
#if 0
					while ( srcPtr <= reinterpret_cast<const uint32_t *>( p ) )
						++srcPtr;
#else	// no looping needed
					srcPtr += 1 + (p - reinterpret_cast<const uchar *>( srcPtr )) / 4;
#endif
					
					// Now set up srcBits
					srcBits = (reinterpret_cast<const uchar *>( srcPtr ) - p) << 3;
					
					// Fill srcData from the previous block.
					srcData = EndianU32_BtoN( srcPtr[-1] ) << (32 - srcBits);
					
					// We might have an extra four bits.
					if ( runLength )
						{
//						pixel = GetBits( bitsPerPixel, srcPtr, srcData, srcBits );
						pixel = GETBITS( bitsPerPixel );
						*dstPtr++ = colors[ pixel ];
						--runLength;
						}
					}
				else
					{
					// slower, general (non-aligned) case
					for ( ; runLength > 0; --runLength )
						{
//						pixel = GetBits( bitsPerPixel, srcPtr, srcData, srcBits );
						pixel = GETBITS( bitsPerPixel );
						*dstPtr++ = colors[ pixel ];
						}
					}
				}
			}
		
		// offset the destination to the next row
		dstPtr += rowOffset;
		}
	
	return noErr;
}


/*
**	DecompressImage()
**
**	decompress the image from the bits and the colors
*/
DTSError
DecompressImage( uchar * dest, const BitsDef * bits, size_t rowBytes, const uchar * colors )
{
	DTSError result = noErr;
	
	// decompress by pack type
	switch ( bits->bdPackType )
		{
		case kPackType1:
			result = DecompressImage1( dest, bits, rowBytes, colors );
			break;
		
		default:
			GenericError( " * Unknown bits pack type (%d). Your image file may be corrupt.",
				int( bits->bdPackType ) );
			result = -1;
			break;
		}
	
	return result;
}


#if BUILDING_CL_EDITOR
/*
**	BuildColors()
**
**	fill out the color table
**	put colors in the table in the order they are encountered in the image
**	return number of colors used (not more than 256, by definition)
*/
uint
BuildColors( const DTSImage * image, uchar colors[ 256 ] )
{
	// step through the image
	// and fill in the color table
	uint nColors = 0;
	DTSRect imageBounds;
	image->GetBounds( &imageBounds );
	int ht = imageBounds.rectBottom;	// we can assume origin at 0,0
	int wd = imageBounds.rectRight;
	const uchar * srcPtr = static_cast<uchar *>( image->GetBits() );
	int rowOffset = image->GetRowBytes() - wd;
	
	// for each source row
	for ( ;  ht > 0;  --ht )
		{
		// for each pixel in the row
		for ( int horz = wd;  horz > 0;  --horz )
			{
			// get the pixel
			uchar pixel = *srcPtr++;
			
			// search for it in the table
			uchar * testPtr = colors;
			for ( uint colorIdx = nColors;  ;  --colorIdx )
				{
				// are we at the end of the table?
				if ( 0 == colorIdx )
					{
					// store the pixel in the table
					// increase the size of the table
					++nColors;
					*testPtr = pixel;
					break;
					}
				
				// have we seen this pixel before?
				if ( *testPtr == pixel )
					break;
				++testPtr;
				}
			}
		
		// offset to the next row
		srcPtr += rowOffset;
		}
	
	// return the size
	return nColors;
}


/*
**	StoreColors()
**
**	store the color table in a kTypePictureColors record
**	search for duplicate color tables
**	color tables can match even if their sizes are different
*/
DTSError
StoreColors( DTSKeyFile * file, const uchar colors[256], size_t sz, DTSKeyID * outID )
{
	DTSKeyID id = 0;
	
	// count the number of extant color tables
	long nColorTabs;
	DTSError result = file->Count( kTypePictureColors, &nColorTabs );
	
	// search all tables looking for a match
	// (Right now we search from high to low IDs; I wonder if going the other direction
	// would be any better.
	// Ditto for bits and lighting records, although the answer there might not be the same.)
	
	if ( noErr == result )
		{
		// for each color table
		int index = nColorTabs;
		do
			{
			// have we run out of color tables?
			if ( 0 == index )
				{
				result = file->GetUnusedID( kTypePictureColors, &id );
				break;
				}
			--index;
			
			// get the ID of the first color table
			DTSKeyID testid;
			// if ( noErr == result )
				result = file->GetID( kTypePictureColors, index, &testid );
			
			// get the size of the color table
			size_t testsize;
			if ( noErr == result )
				result = file->GetSize( kTypePictureColors, testid, &testsize );
			
			// skip it if it's too small
			// (but maybe this is overly aggressive? If our new table is larger than the test
			// one, yet all the colors match -- up to the smaller length -- then why not simply
			// _replace_ the test table with this new one? Existing references won't be affected.
			// However, if we did that, then the checksum scheme WOULD need to be length-aware.)
			if ( noErr == result && testsize < sz )
				continue;
			
			// load the color table
			uchar testcolors[ 256 ];
			if ( noErr == result )
				result = file->Read( kTypePictureColors, testid, testcolors, sizeof testcolors );
			
			// look for a (prefix) match
			if ( noErr == result
			&&   0 == memcmp( colors, testcolors, sz ) )
				{
				// remember the matching ID, and we are done
				id = testid;
				result = 1;
				}
			}
		// bail if there was an error or we are done
		while ( noErr == result );
		}
	
	// okay: no match. we have an id.
	// we need to write the color table
	if ( noErr == result )
		{
		// brain damage -- there must be a better way to do this
		result = WriteData( rClientImagesFREF, kTypePictureColors, id, sz, colors );
		// old way
//		result = file->Write( kTypePictureColors, id, size, colors );
		}
	
	// correct a non-zero non-error result
	if ( result > 0 )
		result = noErr;
	
	// return the id
	*outID = id;
	
	return result;
}


/*
**	StoreBits()
**
**	store the bits in a 'kTypePictureBits' resource
**	search for duplicate ones first, though
*/
DTSError
StoreBits( DTSKeyFile * file, const BitsDef * bits, size_t sz, DTSKeyID * idout )
{
	long nBits;
	DTSKeyID id = 0;
	
	// count the number of bits
	DTSError result = file->Count( kTypePictureBits, &nBits );
	
	// search all records looking for a match
	if ( noErr == result )
		{
		// for each record
		int index = nBits;
		do
			{
			// have we run out of records?
			if ( 0 == index )
				{
				result = file->GetUnusedID( kTypePictureBits, &id );
				break;
				}
			--index;
			
			// get the ID of the first record
			// if ( noErr == result )
				result = file->GetID( kTypePictureBits, index, &id );
			
			// get the size of the record
			size_t testsize;
			if ( noErr == result )
				result = file->GetSize( kTypePictureBits, id, &testsize );
			
			// compare the record sizes
			// load the record
			uchar * testbits = nullptr;
			if ( noErr == result )
				{
				// ignore records of the wrong size
				if ( sz != testsize )
					continue;
				
				// load the record
				result = TaggedReadAlloc( file, kTypePictureBits, id, (void *&) testbits,
							"StoreBitsColr" );
				}
			
			// look for an exact match
			// Note, old versions of the compression routines could have left garbage in the
			// bdReserved field. Might be interesting to see how much extra sharing we could
			// achieve if this next comparison ignored those 2 bytes...
			if ( noErr == result
			&&   0 == memcmp( bits, testbits, sz ) )
				{
				result = 1;			// soft error
				}
			
			// dispose of the temporary buffer
			delete[] reinterpret_cast<char *>( testbits );
			}
		// bail if there was an error or we are done
		while ( noErr == result );
		}
	
	// okay we have an ID
	// we need to write the record
	if ( noErr == result )
		{
		// brain damage -- there must be a better way to do this
		result = WriteData( rClientImagesFREF, kTypePictureBits, id, sz, bits );
		// old way
//		result = file->Write( kTypePictureBits, id, size, bits );
		}
	
	// correct a non-zero non-error result
	if ( result > 0 )
		result = noErr;
	
	// return the id
	*idout = id;
	
	return result;
}


/*
**	NumToBits()
**
**	return log2 of the number
**	round up
*/
static int
NumToBits( int n )
{
	int value = 1;
	for ( int test = 2;  n > test;  ++value )
		test <<= 1;
	
	return value;
}


/*
**	AnalyzeRun()
**
**	analyze one run
*/
static void
AnalyzeRun( const uchar * srcPtr, int sz, int * runLenOut, int * numPixelsOut )
{
	// special cases
	// the paranoid case
	if ( sz <= 0 )
		{
		*runLenOut    = 0;
		*numPixelsOut = 0;
		return;
		}
	
	// a trivial case
	if ( 1 == sz )
		{
		*runLenOut    = 1;
		*numPixelsOut = 1;
		return;
		}
	
	// look at the first two bytes
	uchar pix2 = srcPtr[0];
	uchar pix1 = srcPtr[1];
	
	// a simple case
	if ( 2 == sz )
		{
		*runLenOut = 2;
		if ( pix2 == pix1 )
			*numPixelsOut = 1;
		else
			*numPixelsOut = 2;
		return;
		}
	
	// look at the third byte to decide what kind of run this is
	uchar pix0 = srcPtr[2];
	srcPtr += 3;
	sz     -= 3;
	
	// is this a run of repeated bytes?
	int runLen;
	if ( pix0 == pix1
	&&   pix0 == pix2 )
		{
		runLen = 3;
		for ( ;  sz > 0;  --sz )
			{
			// scan to the end of this run
			if ( *srcPtr != pix0 )
				break;
			++srcPtr;
			++runLen;
			}
		*runLenOut    = runLen;
		*numPixelsOut = 1;
		return;
		}
	
	// this is a run of non-repeated bytes
	runLen = 1;
	for(;;)
		{
		// stop when we have read the last pixels
		if ( sz <= 0 )
			{
			runLen += 2;
			*runLenOut    = runLen;
			*numPixelsOut = runLen;
			return;
			}
		
		// read the next pixel
		pix2 = pix1;
		pix1 = pix0;
		pix0 = *srcPtr++;
		--sz;
		
		// stop if these 3 pixels are the same
		// need to start a different run
		if ( pix0 == pix1
		&&   pix1 == pix2 )
			{
			*runLenOut    = runLen;
			*numPixelsOut = runLen;
			return;
			}
		
		// increase the run length
		++runLen;
		}
}


/*
**	AnalyzeImage()
**
**	analyze the image
**	determine the number of pixels and runs
**	determine the maximum run length
*/
static void
AnalyzeImage( const DTSImage * image, int * numPixelsOut, int * numRunsOut, int * maxRunOut )
{
	// walk through the image
	DTSRect imageBounds;
	image->GetBounds( &imageBounds );
	
	int ht = imageBounds.rectBottom;	// again, assume origin at 0,0
	int wd = imageBounds.rectRight;
	int rowOffset = image->GetRowBytes() - wd;
	const uchar * srcPtr = static_cast<uchar *>( image->GetBits() );
	
	int maxRunLen = 0;
	int numRuns = 0;
	int numPixels = 0;
	
#ifdef DEBUG_VERSION
	const uchar * const srcLimit = srcPtr + ht * ( wd + rowOffset );
#endif
	
	// for each row
	for ( ;  ht > 0;  --ht )
		{
		// analyze every run in the row
		int horz = wd;
		while ( horz > 0 )
			{
			int nPixels;
			int curRunLen;
			AnalyzeRun( srcPtr, horz, &curRunLen, &nPixels );
			
			if ( maxRunLen < curRunLen )
				maxRunLen = curRunLen;
			
			++numRuns;
			numPixels += nPixels;
			horz      -= curRunLen;
			srcPtr    += curRunLen;
			}
		
		// offset to the next row
		srcPtr += rowOffset;
		
#ifdef DEBUG_VERSION
		if ( horz < 0 )
			VDebugStr( "Huh?" );
		if ( srcPtr > srcLimit )
			VDebugStr( "Huh?" );
#endif
		}
	
	// return the results
	*numPixelsOut = numPixels;
	*numRunsOut   = numRuns;
	*maxRunOut    = maxRunLen;
}


/*
**	PutBits()
**
**	inline routine
**	append variably-sized chunks of bits onto a stream of 32-bit words
*/
static /*inline*/
void
PutBits( uint32_t inBits, int bitsToPut, uint32_t *& dstPtr, uint32_t& dstData, int& dstBits )
{
	int shift = 32 - bitsToPut - dstBits;
	if ( shift > 0 )
		{
		dstData |= inBits << shift;
		dstBits += bitsToPut;
		}
	else
		{
		dstBits = - shift;
		dstData |= inBits >> dstBits;
		*dstPtr++ = EndianU32_NtoB( dstData );
		
		// on intel, (x<<32) == x ... but on PPC, (x<<32) == 0
		if ( dstBits )
			dstData = inBits << ( 32 - dstBits );
		else
			dstData = 0;
		}
}


/*
**	CompressBits1a()
**
**	actually compress the bits into the buffer
*/
void
CompressBits1a( const DTSImage * image, BitsDef * bits, size_t sz, int bitsRunLen,
				int bitsPerPixel, const uchar inverse[ 256 ] )
{
#if defined( DEBUG_VERSION )
	// init the limit for debugging
	const uint32_t * const dstLimit = reinterpret_cast<uint32_t *>( (char *) bits + sz );
#else
# pragma unused( sz )
#endif
	
	// walk through the image
	DTSRect imageBounds;
	image->GetBounds( &imageBounds );
	int ht = imageBounds.rectBottom;	// assume 0,0 origin
	int wd = imageBounds.rectRight;
	int rowOffset = image->GetRowBytes() - wd;
	const uchar * srcPtr = static_cast<uchar *>( image->GetBits() );
	
	// fill in the entire BitsDef header
	bits->bdPackType = kPackType1;		// endian-neutral
	bits->bdReserved = 0;	// don't leave random junk here, to maximize sharing
	bits->bdHeight   = EndianS16_NtoB( ht );
	bits->bdWidth    = EndianS16_NtoB( wd );
	
	// vars to put the bits
	uint32_t * dstPtr = reinterpret_cast<uint32_t *>( bits->bdData );
	
	// the first two bytes are log2(bpp) and log2(maxRunLength)
	uint32_t dstData = (bitsPerPixel << 24) + (bitsRunLen << 16);
	int dstBits = 8 + 8;
	
	// for each row
	for ( ;  ht > 0;  --ht )
		{
		// analyze every run in the row
		int horz = wd;
		while ( horz > 0 )
			{
			int nPixels;
			int curRunLen;
			AnalyzeRun( srcPtr, horz, &curRunLen, &nPixels );
			
			// is this a non-repeating run?
			uint32_t data;
			if ( curRunLen == nPixels )
				{
				PutBits( kPackRun, 1, dstPtr, dstData, dstBits );
				PutBits( curRunLen - 1, bitsRunLen, dstPtr, dstData, dstBits );
				for ( ;  nPixels > 0;  --nPixels )
					{
					data = inverse[ *srcPtr++ ];
					PutBits( data, bitsPerPixel, dstPtr, dstData, dstBits );
					}
				}
			else
				{
				// this is a repeating run
				PutBits( kPackRepetition, 1, dstPtr, dstData, dstBits );
				PutBits( curRunLen - 1, bitsRunLen, dstPtr, dstData, dstBits );
				data = inverse[ *srcPtr ];
				PutBits( data, bitsPerPixel, dstPtr, dstData, dstBits );
				srcPtr += curRunLen;
				}
			
			// decrement the number of pixels to be written in this line
			horz -= curRunLen;
			
#ifdef DEBUG_VERSION
			// paranoid check
			if ( dstPtr > dstLimit )
				VDebugStr( "dstPtr > dstLimit" );
#endif
			}
		
		srcPtr += rowOffset;
		}
	
	// flush the data
	PutBits( 0, 31, dstPtr, dstData, dstBits );
	
#ifdef DEBUG_VERSION
	// paranoid check
	if ( dstPtr > dstLimit )
		VDebugStr( "dstPtr > dstLimit" );
#endif
}


/*
**	CompressBits1()
**
**	compress by run length encoding
**	use the minimum number of bits per pixel
**	and the minimum number of bits for the run length
**	first byte of the data is the bits per pixel
**	second byte is the size of the run length in bits
**	then we have the runs
**	1 bit for run type: 0=repetition, 1=run
**	n bits for run length
**	repetition will then have 1 pixel
**	run will then have a stream of pixels
*/
static DTSError
CompressBits1( const DTSImage * image, BitsDef ** bitsout, size_t * sizeout,
			   const uchar colors[ 256 ], int numColors )
{
	// analyze the image to see how many runs there are
	// and how many pixels will need to be written
	int numPixels;
	int numRuns;
	int maxRun;
	AnalyzeImage( image, &numPixels, &numRuns, &maxRun );
	
	// set the size in bits of the pixels and the run length
	int bitsPerPixel = NumToBits( numColors );
	int bitsRunLen   = NumToBits( maxRun );
	
	// compute the size of the buffer needed, including the BitsDef header (sans bdData),
	// the two special data-leader bytes (bpp & maxRunLen), and of course the data itself.
	size_t sz = offsetof( BitsDef, bdData ) + 2
			 + ((numRuns * (bitsRunLen + 1) + numPixels * bitsPerPixel + 7) >> 3);
	
	// allocate the buffer
	// return the bits
	// bail if there was an error
	size_t allocSize = ( sz + 3 ) & ~3;
	BitsDef * bits = reinterpret_cast<BitsDef *>( NEW_TAG("CompressedBits") char[ allocSize ] );
	*bitsout = bits;
	*sizeout = allocSize;	// used to be 'sz', but we always want the Bits data to be
							// a whole multiple of 4 bytes in length, on disk
	if ( not bits )
		return memFullErr;
	
	// create the inverse table
	uchar inverse[ 256 ];
//	memset( inverse, 0, sizeof inverse );
	for ( int index = numColors - 1;  index >= 0;  --index )
		inverse[ colors[ index ] ] = index;
	
	// actually compress the image
//	memset_pattern4( bits, "\xF1\xF2\xF3\xF4", allocSize );		// ** DEBUG
	CompressBits1a( image, bits, allocSize, bitsRunLen, bitsPerPixel, inverse );
	
	return noErr;
}


/*
**	CompressBits()
**
**	compress the bits using all available compression methods
**	return the smallest one
**	Right now, that simply means CompressBits1()
*/
DTSError
CompressBits( const DTSImage * image, BitsDef ** bitsout, size_t * sizeout,
			  const uchar colors[ 256 ], int numColors )
{
	return CompressBits1( image, bitsout, sizeout, colors, numColors );
}


/*
**	StoreLightInfo()
**
**	store the (native-endian) light info in a kTypeLightingData key record.
**	search for duplicate tables.
*/
DTSError
StoreLightInfo( DTSKeyFile * file, const LightingData * info, DTSKeyID * idout )
{
	// easy case if there isn't any lighting data
	if ( info->ilCValue == 0
	&&	 info->ilRadius == 0
	&&	 info->ilPlane  == 0 )
		{
		*idout = 0;
		return noErr;
		}
	
	// byteswap if necessary
#if DTS_LITTLE_ENDIAN
	LightingData ldbe = *info;
	NativeToBigEndian( &ldbe );
	// fudge the data pointer
	info = &ldbe;
#endif  // DTS_LITTLE_ENDIAN
	
	long nLights;
	DTSKeyID id = 0;
	
	// count the number of lighting records
	DTSError result = file->Count( kTypeLightingData, &nLights );
	
	// search all records looking for a match
	if ( noErr == result )
		{
		// for each table
		int index = nLights;
		do
			{
			// have we run out of color tables?
			if ( 0 == index )
				{
				// get a fresh new unused ID
				result = GetUnusedIDNotZero( file, kTypeLightingData, id );
				break;
				}
			--index;
			
			// get the id of the first table
			DTSKeyID testid;
			result = file->GetID( kTypeLightingData, index, &testid );
			
			// load the lighting data (big-endian on disk)
			LightingData test;
			if ( noErr == result )
				result = file->Read( kTypeLightingData, testid, &test, sizeof test );
			
			// look for an exact match
			if ( noErr == result
			&&   0 == memcmp( info, &test, sizeof test ) )
				{
				// remember the ID, and we are done
				id = testid;
				result = 1;
				break;
				}
			}
		// bail if there was an error or we are done
		while ( noErr == result );
		}
	
	// okay we have an ID
	// we need to write the lighting data
	// unless we found a previous match, i.e. err == 1, in which case we can skip this
	if ( noErr == result )
		result = WriteData( rClientImagesFREF, kTypeLightingData, id, sizeof *info, info );
	
	// correct a non-zero non-error result
	if ( result > 0 )
		result = noErr;
	
	// return the ID
	*idout = id;
	
	return result;
}


/*
**	GetImageSize()
**
**	return the visible size of the image.
**	It knows about multi-frame animations, but NOT about mobiles
**	(and, therefore, not about custom-colored mobiles, either)
*/
DTSError
GetImageSize( DTSKeyFile * file, DTSKeyID id, int * width, int * height )
{
	// not sure we require this much paranoia...
//	if ( not width || not height )
//		return paramErr;
	
	// load the PictDef (but don't bother byte-swapping it)
	PictDef pd;
	DTSError result = file->Read( kTypePictureDefinition, id, &pd, sizeof pd );
	
	// and the BitsDef portion of the Bits record
	BitsDef bits;
	if ( noErr == result )
		{
		DTSKeyID bitsID = EndianS32_BtoN( pd.pdBitsID );
		result = file->Read( kTypePictureBits, bitsID, &bits, sizeof bits );
		}
	
	if ( noErr == result )
		{
		int ht = EndianS16_BtoN( bits.bdHeight );
		int nf = EndianS16_BtoN( pd.pdNumFrames );
		if ( nf > 1 )
			ht /= nf;
		
		*width  = EndianS16_BtoN( bits.bdWidth );
		*height = ht;
		}
	
	// if any errors occurred, report sizes as zero
	if ( noErr != result )
		{
		*width  = 0;
		*height = 0;
		}
	
	return result;
}


/*
**	GetImageNumColors()
**
**	return the approximate number of colors used by the image
**	(to the next nearest power-of-two. For an image with just 13 colors, this function
**	would report an answer of 16. There's no quick way to get a truly accurate answer
**	without decompressing the pixels and enumerating them directly.)
**	Used only by Editor.
*/
DTSError
GetImageNumColors( DTSKeyFile * file, DTSKeyID bitsid, int * numColors )
{
	int ncolors = 0;
	BitsDef bits;
	
	// read the BitsDef portion of the Bits record
	DTSError result = file->Read( kTypePictureBits, bitsid, &bits, sizeof bits );
	
	if ( noErr == result )
		{
		if ( kPackType1 == bits.bdPackType )	// endian-neutral
			ncolors = 1 << bits.bdData[0];
		else	// some new packing scheme that we don't understand
			result = -1;
		}
	
	*numColors = ncolors;
	return result;
}
#endif  // BUILDING_CL_EDITOR


#if defined( DEBUG_VERSION) || defined( DEBUG_VERSION_TAGS )
/*
**	TaggedReadAlloc()
**
**	a debugging wrapper that gives generic calls to ReadAlloc() a useful memory block ID tag.
*/
DTSError
TaggedReadAlloc( DTSKeyFile * file, DTSKeyType ttype, DTSKeyID id, void *& buf, const char * tag )
{
#ifndef DEBUG_VERSION_TAGS
# pragma unused( tag )
#endif
	
	size_t sz;
	char * localbuffer = nullptr;
	
	DTSError result = file->GetSize( ttype, id, &sz );
	if ( noErr == result )
		{
		localbuffer = NEW_TAG(tag) char[ sz ];
		if ( not localbuffer )
			result = memFullErr;
		}
	
	if ( noErr == result )
		result = file->Read( ttype, id, localbuffer, sz );
	
	if ( result != noErr )
		{
		delete[] localbuffer;
		localbuffer = nullptr;
		}
	
	buf = localbuffer;
	return result;
}
#endif  // DEBUG_VERSION || DEBUG_VERSION_TAGS


#pragma mark -
#pragma mark Color Palette and CGImage Support

//	CGImage support

//
// Standard Clan Lord color table, as 8-bit-each XRGB quadlets in host byte order (cf. RGBColor8).
// Original data taken from ::GetCTable( 8 + 64 ); then somewhat massaged.
// Note that the "alpha" values, in the high-order 8 bits, are all set to 0 here.
// We're baking this table into the client and editor because GetCTable() has been deprecated,
// but both apps still occasionally need to do index-to-color lookups.
//
static const uint32_t
gStandardClut[ 256 ] =
{
	// By law, the first entry always has to be white, and the last one always black.
	
	0x00FFFFFF,	0x00FFFFCC,	0x00FFFF99, 0x00FFFF66,	0x00FFFF33,	0x00FFFF00, // 5
	0x00FFCCFF,	0x00FFCCCC,	0x00FFCC99, 0x00FFCC66,	0x00FFCC33,	0x00FFCC00, // 11
	0x00FF99FF,	0x00FF99CC,	0x00FF9999, 0x00FF9966,	0x00FF9933,	0x00FF9900, // 17
	0x00FF66FF,	0x00FF66CC,	0x00FF6699,	0x00FF6666,	0x00FF6633,	0x00FF6600,	// 23
	0x00FF33FF,	0x00FF33CC,	0x00FF3399,	0x00FF3366,	0x00FF3333,	0x00FF3300,	// 29
	0x00FF00FF,	0x00FF00CC,	0x00FF0099,	0x00FF0066,	0x00FF0033,	0x00FF0000,	// 35
	0x00CCFFFF,	0x00CCFFCC,	0x00CCFF99,	0x00CCFF66,	0x00CCFF33,	0x00CCFF00,	// 41
	0x00CCCCFF,	0x00CCCCCC,	0x00CCCC99,	0x00CCCC66,	0x00CCCC33,	0x00CCCC00,	// 47
	0x00CC99FF,	0x00CC99CC,	0x00CC9999,	0x00CC9966,	0x00CC9933,	0x00CC9900,	// 53
	0x00CC66FF,	0x00CC66CC,	0x00CC6699,	0x00CC6666,	0x00CC6633,	0x00CC6600,	// 59
	0x00CC33FF,	0x00CC33CC,	0x00CC3399,	0x00CC3366,	0x00CC3333,	0x00CC3300,	// 65
	0x00CC00FF,	0x00CC00CC,	0x00CC0099,	0x00CC0066,	0x00CC0033,	0x00CC0000,	// 71
	0x0099FFFF,	0x0099FFCC,	0x0099FF99,	0x0099FF66,	0x0099FF33,	0x0099FF00,	// 77
	0x0099CCFF,	0x0099CCCC,	0x0099CC99,	0x0099CC66,	0x0099CC33,	0x0099CC00,	// 83
	0x009999FF,	0x009999CC,	0x00999999,	0x00999966,	0x00999933,	0x00999900,	// 89
	0x009966FF,	0x009966CC,	0x00996699,	0x00996666,	0x00996633,	0x00996600,	// 95
	0x009933FF,	0x009933CC,	0x00993399,	0x00993366,	0x00993333,	0x00993300,	// 101
	0x009900FF,	0x009900CC,	0x00990099,	0x00990066,	0x00990033,	0x00990000,	// 107
	0x0066FFFF,	0x0066FFCC,	0x0066FF99,	0x0066FF66,	0x0066FF33,	0x0066FF00,	// 113
	0x0066CCFF,	0x0066CCCC,	0x0066CC99,	0x0066CC66,	0x0066CC33,	0x0066CC00,	// 119
	0x006699FF,	0x006699CC,	0x00669999,	0x00669966,	0x00669933,	0x00669900,	// 125
	0x006666FF,	0x006666CC,	0x00666699,	0x00666666,	0x00666633,	0x00666600,	// 131
	0x006633FF,	0x006633CC,	0x00663399,	0x00663366,	0x00663333,	0x00663300,	// 137
	0x006600FF,	0x006600CC,	0x00660099,	0x00660066,	0x00660033,	0x00660000,	// 143
	0x0033FFFF,	0x0033FFCC,	0x0033FF99,	0x0033FF66,	0x0033FF33,	0x0033FF00,	// 149
	0x0033CCFF,	0x0033CCCC,	0x0033CC99,	0x0033CC66,	0x0033CC33,	0x0033CC00,	// 155
	0x003399FF,	0x003399CC,	0x00339999,	0x00339966,	0x00339933,	0x00339900,	// 161
	0x003366FF,	0x003366CC,	0x00336699,	0x00336666,	0x00336633,	0x00336600,	// 167
	0x003333FF,	0x003333CC,	0x00333399,	0x00333366,	0x00333333,	0x00333300,	// 173
	0x003300FF,	0x003300CC,	0x00330099,	0x00330066,	0x00330033,	0x00330000,	// 179
	0x0000FFFF,	0x0000FFCC,	0x0000FF99,	0x0000FF66,	0x0000FF33,	0x0000FF00,	// 185
	0x0000CCFF,	0x0000CCCC,	0x0000CC99,	0x0000CC66,	0x0000CC33,	0x0000CC00,	// 191
	0x000099FF,	0x000099CC,	0x00009999,	0x00009966,	0x00009933,	0x00009900,	// 197
	0x000066FF,	0x000066CC,	0x00006699,	0x00006666,	0x00006633,	0x00006600,	// 203
	0x000033FF,	0x000033CC,	0x00003399,	0x00003366,	0x00003333,	0x00003300,	// 209
	0x000000FF,	0x000000CC,	0x00000099,	0x00000066,	0x00000033,				// 214
	
	// after the 6x6x6 color cube, we get the pure-color and grayscale ramps.
	// (some shades are skipped here, because we've already had them in the 6x6x6, above.)
	0x00EE0000, 0x00DD0000,	0x00BB0000,	0x00AA0000,	0x00880000, // 219 -- ten shades of red
	0x00770000, 0x00550000, 0x00440000,	0x00220000,	0x00110000, // 224
	0x0000EE00,	0x0000DD00, 0x0000BB00, 0x0000AA00,	0x00008800, // 229 -- and green
	0x00007700,	0x00005500,	0x00004400,	0x00002200, 0x00001100,	// 234
	0x000000EE,	0x000000DD,	0x000000BB,	0x000000AA,	0x00000088,	// 239  -- and blue
	0x00000077,	0x00000055,	0x00000044,	0x00000022,	0x00000011,	// 244
	0x00EEEEEE, 0x00DDDDDD,	0x00BBBBBB,	0x00AAAAAA,	0x00888888, // 249  -- and gray
	0x00777777,	0x00555555, 0x00444444,	0x00222222,	0x00111111, // 254
	0x00000000	// 255 - black brings up the rear
};


/*
**	GetCLColor()
**
**	returns the idx'th color from the above system-standard 8-bit color palette.
*/
DTSColor
GetCLColor( uint8_t idx )
{
	const RGBColor8 * pc = reinterpret_cast<const RGBColor8 *>( &gStandardClut[ idx ] );
	
	// RGBColor8's have 8 bits per component; DTSColors have 16.
	// So we need to smear them out.
	return DTSColor( pc->red   << 8 | pc->red,
					 pc->green << 8 | pc->green,
					 pc->blue  << 8 | pc->blue );
}


/*
**	GetCLColorTable()
**
**	For efficiency reasons, some callers may wish to muck around in the table itself,
**	wholesale, versus tiptoeing one-by-one via GetCLColor().  Exporting the table
**	through this function seems mildly cleaner than simply marking gStandardClut
**	as "extern" instead of "static".  Plus, the table would probably need a better name
**	if it were publicly visible, and I lack the imagination to think of a good one.
*/
const RGBColor8 *
GetCLColorTable()
{
	return reinterpret_cast<const RGBColor8 *>( gStandardClut );
}


//
//	Pixel-packing formats for the generated CGImages
//
// not sure which, if any, is best out of ARGB, RGBA, BGRA, or ABGR
// (or any of the other 20 possible permutations...).  Anyway, these macros
// enumerate the options available below.  Mnemonic: R=1 G=2 B=3 A=4
#define D2C_RGBA	1234
#define D2C_ARGB	4123
#define D2C_BGRA	3214	// implementation incomplete
#define	D2C_ABGR	4321	// ditto
	
// let's pick one at random.
#define DT2CG_FMT	D2C_RGBA
//#define DT2CG_FMT	D2C_ARGB

//
// The last time I tried it (admittedly quite a long while ago...), NSBitmapImageReps
// would only accept big-endian data, whereas CGImageRefs will take either endianness.
// Turn this on to be NSImage-friendlier.
//
#define FORCE_BIG_ENDIAN_FOR_NSIMAGE	0


//
// Compile-time constants derived from the selected pixel-packing format.
//
// 'kAlphaShift' is how far you have to left-shift an 8-bit alpha value
// to move it to the correct spot in a 32-bit pixel.
//
// 'kAllWhiteBits' is the representation of (the RGB-bits portion of) an all-white pixel,
// i.e. one with a presumptive alpha of 0.
//
#if DT2CG_FMT == D2C_ARGB || DT2CG_FMT == D2C_ABGR
const int		kAlphaShift		= 24;
const uint32_t	kAllWhiteBits	= 0x00FFFFFFU;
#elif DT2CG_FMT == D2C_RGBA || DT2CG_FMT == D2C_BGRA
const int		kAlphaShift		= 0;
const uint32_t	kAllWhiteBits	= 0xFFFFFF00U;
#else
# error "Unknown pixel format"
#endif

	
//
// The gStandardClut table maps CL color indices to plain RGB, in host byte-order.
// This next table -- constructed at runtime -- instead maps indices to actual pixel values,
// arranged as per the DT2CG_FMT setting.  Values extracted from this table are almost
// ready to be used in the final image bitmap; all that remains is to insert the proper
// alpha bits (which cannot be determined beforehand; they depend on each PictDef's
// blending & transparency settings).
//
// By precomputing these values, we eliminate the need for lots of bit-shifting, repeated
// for each and every pixel, whilst assembling the final bitmap contents.  All that's needed
// is a single logical-OR operation.
//


/*
**	GenerateD2CPixelTable()
**
**	create the almost-direct pixel lookup table
*/
static void
GenerateD2CPixelTable()
{
	uint32_t * table = NEW_TAG("D2CPixelTable") uint32_t[ 256 ];
	
	// if we couldn't get memory, we're hosed.
	if ( not table )
		return;
	
	uint32_t * dst = table;
	const uint32_t * src = gStandardClut;
	
	// if I were really clever I'd find a way to get the compiler to build
	// this table directly at compile time, via devious C++ template voodoo.
	
	do {
		const RGBColor8 * rgb = reinterpret_cast<const RGBColor8 *>( src );
		
		// get the full 8 bits of each component
		uint8_t red = rgb->red;
		uint8_t green = rgb->green;
		uint8_t blue = rgb->blue;
		
		// Reassemble the components into the desired order/placement.
		// Note that for ABGR & BGRA formats, a subsequent byteswap will
		// likely be necessary elsewhere in the conversion process, unless
		// we do something subtle with the kCGBitmapByteOrderXXXX flags
		// that'll eventually be supplied to CGImageCreate().
#if DT2CG_FMT == D2C_RGBA  || DT2CG_FMT == D2C_ABGR
		uint32_t value = (red << 24) | (green << 16) | (blue << 8);
#elif DT2CG_FMT == D2C_ARGB || DT2CG_FMT == D2C_BGRA
		uint32_t value = (red << 16) | (green << 8) | blue;
#endif
		
		// store this pixel value
		*dst++ = value;
	} while ( *src++ != 0 );	// gStandardClut ends with pure black (0x0)
	
	// Not that I don't trust my own math, but better safe than sorry.
	__Check( dst - table == 256 );
	__Check( src - gStandardClut == 256 );
	
	gD2CPixelTable = table;
}

	
/*
**	MyImageReleaserCB()
**
**	This gets called by CoreGraphics when it's time to release
**	the pixel data associated with a CGImageRef.
*/
static void
MyImageReleaserCB( void * ud, const void * data, size_t len )
{
	// 'ud' is the user-data value ("refcon") that was passed as
	// the first argument to CGDataProviderCreateWithData().
	// We passed NULL, cos we don't need any user-data here.
	// 'len' is the length of the bitmap data, but again we don't need it,
	// since we allocated that data with operator new[]
	// which already magically knows its length.
#pragma unused( ud, len )
	
	delete[] static_cast<const UInt32 *>( data );
}


/*
**	CGImageCreateFromDTSImage()
**
**	Given a valid DTSImage, this produces an equivalent CGImageRef.
**	'pdFlags' is the same value you'd find in a PictDef, although only the
**	blending and transparency bits are honored here.  Unlike the existing "BlitBlend"
**	routines in the client and editor, the CGImages we produce have true alpha-levels
**	(insofar as possible: ultimately we're limited by the fact that the PD flags can only
**	specify five distinct degrees of transparency: 0, 25, 50, 75, or 100%).
**
**	If any conversion failures occur, this function returns NULL.
**	Otherwise, the resulting CGImage must be released by the caller.
**
**	The image uses 32 bits per pixel (ARGB or RGBA), so memory usage will be approx.
**	4x higher than that of the input DTSImage.
*/
CGImageRef
CGImageCreateFromDTSImage( const DTSImage * img, uint32_t pdFlags )
{
	// paranoia
	if ( not img || not img->GetBits() )
		return nullptr;
	
	// one-time setup of the pixel-value lookup table
	if ( not gD2CPixelTable )
		{
		GenerateD2CPixelTable();
		
		// if we couldn't get memory for the table, we're hosed
		if ( not gD2CPixelTable )
			return nullptr;
		}
	
	CGImageRef cgImg = nullptr;	// the output image
	
	
	// Step 1: Extract transparency & alpha info from the pictDefFlags
	
	const bool bWhiteIsTransparent = (pdFlags & kPictDefFlagTransparent) != 0;
	
#if 0
	// the simple, obvious, perspicuous way of calculating the alpha bits
	uint32_t alpha = 0;
	switch ( pdFlags & kPictDefBlendMask )
		{
		case kPictDefNoBlend:	alpha = 0x0FFu;	break;	// 100% opaque
		case kPictDef25Blend:	alpha = 0x0BFu;	break;	//  75%
		case kPictDef50Blend:	alpha = 0x07Fu;	break;	//  50%
		case kPictDef75Blend:	alpha = 0x03Fu;	break;	//  25%
		}
	alpha <<= kAlphaShift;
#else
	// totally impenetrable method, but which can be assigned to a const.
	// skeptical readers should prove to themselves that these two
	// formulations are in fact equivalent.
	// (or that they're not; in which case, please let me know ASAP!)
	const uint32_t alpha = ((
			(( ~((pdFlags & kPictDefBlendMask) << 2) ) & 0x0FU)		// [0-3] => 0x[F,B,7,3]
				<< 4) | 0x0FU)										// => 0xFF, BF, 7F, 3F
			<< kAlphaShift;											// account for packing order
#endif  // 0
	
	
	// Step 2: get height, width, and row-bytes of source image
	
	DTSRect box;
	img->GetBounds( &box );
	const int width = box.rectRight - box.rectLeft;
	const int height = box.rectBottom - box.rectTop;
	const int srcRB = img->GetRowBytes();
	
	
	// Step 3: determine characteristics of the output image
	enum {
		bpc = 8,		// 8 bits per component
	
	// bitmap info
#if FORCE_BIG_ENDIAN_FOR_NSIMAGE
		kEndianInfo = kCGBitmapByteOrder32Big,
#else
		kEndianInfo = kCGBitmapByteOrder32Host,
#endif
#if DT2CG_FMT == D2C_ARGB
		kAlfaInfo = kCGImageAlphaFirst,	// ARGB
#elif DT2CG_FMT == D2C_RGBA
		kAlfaInfo = kCGImageAlphaLast,	// RGBA
#else
# error "unimplemented pixel format"
	// I think they're gonna be basically the same as the first two except
	// for specifying the reverse ("foreign"?) byte order, i.e., 
	//	 ( __BIG_ENDIAN__ )	? kCGBitmapByteOrder32Little : k_ByteOrder32Big
	// Or, to put it another way, just XOR the above values with
	//	(kCGBitmapByteOrder32Big | kCGBitmapByteOrder32Little).
#endif
		
		// combine into a CGBitmapInfo
		kBMInfo = kAlfaInfo | kEndianInfo,
	
		bytesPerPixel = (bpc * 4) / 8,	// 4 components per pixel; 8 bits/byte == 4
		};
	
	// ensure each row is aligned to a 16-byte boundary
	const int bytesPerRow = (width * bytesPerPixel + 15) & ~15;
	
	
	// Step 4: pick a color space for the resulting image.
	
	// Dox say Generic RGB space is favored over DeviceRGB, but in my limited experience
	// so far, the latter produces results more faithful to the old QD-based images.
	
//	CGColorSpaceRef cSpace = CGColorSpaceCreateWithName( kCGColorSpaceGenericRGB );
	CGColorSpaceRef cSpace = CGColorSpaceCreateDeviceRGB();
	if ( cSpace )
		{
		// Step 5: allocate space for the image's bits
		// (released later, via MyImageReleaserCB)
		
		UInt32 * pixels = NEW_TAG("CGImageBits") UInt32[ height * bytesPerRow ];
		if ( pixels )
			{
			// Step 6: transfer pixels from source to destination
			
			// point to source image's bits, each byte of which is an index into
			// the standard CL 8-bit color palette.
			const UInt8 * src = static_cast<UInt8 *>( img->GetBits() );
			
			// how many pixels to skip at the end of each source/destination row
			const int srcRowOffset = srcRB - width * sizeof(*src);	// bytes
			const int dstRowOffset = (bytesPerRow / 4) - width;		// U32's
			
			// start copying pixels
			UInt32 * dst = pixels;
			for ( int row = 0; row < height; ++row )
				{
				for ( int col = 0; col < width; ++col )
					{
					// get the pixel's color index
					UInt8 pixel = *src++;
					
					// fabricate the corresponding ARGB (or what-have-you) value
					UInt32 dstPixel;
					if ( 0 == pixel )	// white
						{
						// white pixels are either fully transparent or
						// "blendable" white, depending on the PDef flags
						if ( bWhiteIsTransparent )
							dstPixel = kAllWhiteBits;	// all-white RGB, alpha = 0
						else
							dstPixel = alpha | kAllWhiteBits;	// white w/possibly non-0 alpha
						}
					else
						{
						// for any other value, grab its RGB bits from the color table,
						// and blend in the alpha bits.
						dstPixel = gD2CPixelTable[ pixel ] | alpha;
						}
					
					// store the final pixel value.
#if FORCE_BIG_ENDIAN_FOR_NSIMAGE
					*dst++ = EndianU32_NtoB( dstPixel );
#else
					*dst++ = dstPixel;	// host order: cf kEndianInfo above
#endif
					}
				
				// go to next row
				src += srcRowOffset;
				dst += dstRowOffset;
				}
			
			// post-loop sanity checks on my arithmetic
			__Check( (src - static_cast<const UInt8 *>( img->GetBits() )) * sizeof *src
					== size_t( height ) * img->GetRowBytes() );
			__Check( (dst - pixels) * sizeof *dst == size_t( height ) * bytesPerRow );
			
			// Step 7: now that we have our ARGB/RGBA pixel data, wrap it up in a nice
			// comfy CGDataProvider, just like CGImageCreate() wants.  This is also
			// where we specify how to _dispose_ of that data, eventually:
			// viz., via the releaser callback function.
			CGDataProviderRef prov = CGDataProviderCreateWithData(
						nullptr,				// no special user-data
						pixels,					// the data
						height * bytesPerRow,	// its size
						MyImageReleaserCB );	// releaser function
			
			// Step 8: bake at 1000C for 0.02 nanoseconds, or until golden-brown.
			if ( prov )
				{
				// OK, we're finally ready to create the darn thing!
				cgImg = CGImageCreate(
						width, height,			// dimensions
						bpc,					// bits per component
						8 * bytesPerPixel,		// bits per pixel
						bytesPerRow,			// rowbytes
						cSpace,					// color space
						kBMInfo,				// pixel packing format
						prov,					// data provider/releaser
						nullptr,				// no decode array
						false,					// no interpolation
						kCGRenderingIntentDefault );	// whatever
				__Check( cgImg );
				
				// We're done with the data-provider.
				// (This won't actually release any data, since the CGImage holds its own
				// retain on that provider.  Not until the image itself is released will the
				// provider -- and the big bitmap buffer it's managing -- truly go away.)
				CGDataProviderRelease( prov );
				}
			else
				{
				// uh oh, something went wrong creating the provider.
				// Nothing we can do to fix that, but at least we'd better
				// drop the now-useless pixel buffer.
				delete[] pixels;
				}
			}
		
		// done with the color space
		CFRelease( cSpace );
		}
	
	// Step 9: we're done
	
	return cgImg;
}


// support for decompressing an on-disk image directly into a CGImage
#define	DO_DIRECT_TO_CGIMAGE		1

#if DO_DIRECT_TO_CGIMAGE
/*
**	Set1PixelTransparency()
**
**	given a CL pixel-color index and a destination pointer,
**	lookup the appropriate RGBColor, massage it into the correct format according
**	to the DT2CG_FMT setting, and lastly store it.
**	Returns the incremented destination pointer, where the following pixel should go.
*/
static inline
//__attribute__(( always_inline ))
UInt32 *
Set1PixelTransparency( uchar cIndex, UInt32 * loc, UInt32 alpha )
{
	UInt32 value = gD2CPixelTable[ cIndex ] | alpha;
	
	// if we were building this bitmap to feed into an NSBitmapImageRep,
	// we'd need to force bit-endian pixel layout. But for a CGImageRef,
	// we can use host endianness.
# if FORCE_BIG_ENDIAN_FOR_NSIMAGE
	*loc++ = EndianU32_NtoB( value );
# else
	*loc++ = value;
# endif
	
	return loc;
}


// HACK: shorthand for Set1PixelTransparency(). This works because 'alpha' and 'isTransparent'
// are attributes of the entire picture, and thus don't change from one pixel to the next.
// Likewise, 'pixel' is a local variable in DecompressImage1Alpha().
// For transparent all-white pixels, we cheat by forcing 'alpha' to 0.
#define SetOnePixel( loc )	\
	Set1PixelTransparency( pixel, (loc), (0 == pixel && isTransparent) ? 0 : alpha )

/*
**	DecompressImage1Alpha()
**
**	like DecompressImage1() above, but generates 32-bit RGBA (or ARGB) pixels into 'data'.
**	Blending and transparency settings are derived from 'flags'.
**	In all other respects, deliberately written to be as close as possible to DecompImg1(),
**	in hopes that maybe one day we can combine them.
*/
static DTSError
DecompressImage1Alpha( UInt32 * dstPtr,	// destination
		const BitsDef * bits,			// source data
		uint32_t flags,					// PictDef's pdFlags
		size_t rowBytes,				// dimensions of dest
		const uchar * colors )			// colorization vector
{
	// local variables
	int ht = bits->bdHeight;
	const int wd = bits->bdWidth;
	
	// Both rowBytes and wd are integral multiples of 4, so it's safe to make
	// rowOffset be the number of UInt32 _words_ to advance, not _bytes_.
	const int rowOffset = rowBytes / sizeof( *dstPtr ) - wd;
	const uint32_t * srcPtr = reinterpret_cast<const uint32_t *>( bits->bdData );
	
	// calculate the limit for debugging
	const UInt32 * dstLimit = dstPtr + (rowBytes / 4) * ht - rowOffset;
	
	// the first byte in the data is the number of bits per pixel
	// the second byte is the size of the run length in bits
	const uint bitsPerPixel = bits->bdData[ 0 ];
	const int runBits = bits->bdData[ 1 ];
	
	// since our "pixels" are actually indices into the 256-color CLUT,
	// they cannot be larger than 8 bits wide. Images compressed via StoreImage() will always
	// fulfil this requirement, but I guess it can't hurt to be wary: someone might have
	// maliciously meddled with the contents of their images file.
	__Check( bitsPerPixel <= 8 );
	if ( bitsPerPixel > 8 )
		return -1;	// we should have a dedicated kImageIsCorruptError
	
	// get the first 32-bit chunk of data
	uint32_t srcData = EndianU32_BtoN( *srcPtr++ );
	srcData <<= 16;		// we've already read the first 2 bytes
	int srcBits = 16;	// (bitsPerPixel and runBits)
	
	// constant, preshifted alpha byte (determined by blending flags)
	const uint32_t alpha = ((
			(( ~((flags & kPictDefBlendMask) << 2) ) & 0x0FU)		// [0-3] => 0x[F,B,7,3]
				<< 4) | 0x0FU)										// => 0xFF, BF, 7F, 3F
			<< kAlphaShift;											// account for packing order
	
	// transparency flag
	const bool isTransparent = (flags & kPictDefFlagTransparent) != 0;
	
	// set up the lookup table
	if ( not gD2CPixelTable )
		{
		GenerateD2CPixelTable();
		if ( not gD2CPixelTable )
			return -1;
		}
	
	// for each row in the destination
	for ( ;  ht > 0;  --ht )
		{
		// for each pixel in the destination
		for ( int nPixels = wd;  nPixels > 0;  )
			{
			// get 1 bit
			if ( 0 == srcBits )
				{
				// reload the accumulator
				srcData = EndianU32_BtoN( *srcPtr++ );
				srcBits = 32;
				}
			
			// get 1 bit, to tell us what kind of run we have
			int32_t runType = srcData & 0x80000000;
			srcData <<= 1;
			--srcBits;
			
			// get the run length
			uint runLength = GETBITS( runBits );
			
			// error check
			if ( dstPtr + runLength > dstLimit )
				{
#ifdef DEBUG_VERSION
				VDebugStr( "DecompressBits1Alpha overflow! (%td)",
					(dstPtr + runLength) - dstLimit );
#endif
				return -1;
				}
			
			// decrement the number of pixels remaining to be unpacked for this line
			++runLength;
			nPixels -= runLength;
			
			uint32_t pixel;
			if ( (kPackRepetition << 31) == runType )
				{
				// write the repeated pixel
				pixel = GETBITS( bitsPerPixel );
				pixel = colors[ pixel ];
				
				// This hoists the innards of SetOnePixel() out of the loop.
				// For transparent all-white pixels, we cheat by forcing alpha to 0% opacity.
				if ( 0 == pixel && isTransparent )
					pixel = kAllWhiteBits;
				else
					pixel = gD2CPixelTable[ pixel ] | alpha;
				
#if FORCE_BIG_ENDIAN_FOR_NSIMAGE
				pixel = EndianU32_NtoB( pixel );
#endif
				
				// memset_pattern4() requires 10.5+
#if MAC_OS_X_VERSION_MIN_REQUIRED > 1040
				memset_pattern4( dstPtr, &pixel, runLength * sizeof( *dstPtr ) );
				dstPtr += runLength;
#else
				for ( ; runLength > 0;  --runLength )
					*dstPtr++ = pixel;
#endif  // 10.5+
				}
			else
				{
				// write the run of non-repeating pixels
				
				// special case for 8-bit color; also even-aligned 4-bit color.
				// we can do those faster than the general-case code down at the bottom,
				// since we can avoid a lot of the overhead of GetBits()
				
				if ( ( 8 == bitsPerPixel && not (srcBits & 0x07) )
				||   ( 4 == bitsPerPixel && not (srcBits & 0x03) && runLength >= 3 ) )
					{
					// Yes! We are home free.
					
					if ( srcBits & 0x07 )
						{
						// If we have a four bit pixel on a non-char boundary, advance it.
						pixel = GETBITS( bitsPerPixel );
						pixel = colors[ pixel ];
						dstPtr = SetOnePixel( dstPtr );
						--runLength;
						}
					
					// First, put srcData back.
					const uchar * p = reinterpret_cast<const uchar *>( --srcPtr );
					
					// In case we are not at a 32-bit boundary, make it so we are.
					if ( srcBits != 32 )
						p += (32 - srcBits) >> 3;
					
					// do the main part of the run. Look, Ma: no GetBits!
					if ( 8 == bitsPerPixel )
						{
						for ( ; runLength > 0;  --runLength )
							{
							pixel = colors[ *p++ ];
							dstPtr = SetOnePixel( dstPtr );
							}
						}
					else
						{
						// must be 4 bpp; take 'em 2 at a time
						for ( ; runLength > 1; runLength -= 2, ++p )
							{
							pixel = colors[ (*p) >> 4 ];
							dstPtr = SetOnePixel( dstPtr );
							pixel = colors[ (*p) & 0x0F ];
							dstPtr = SetOnePixel( dstPtr );
							}
						}
					// OK that was fun while it lasted. We now return to drab mundanity.
					
					// Now make sure srcPtr, srcData, srcBits are set properly.
					
					// Step one past the next read.
					//	[You'd think this could be as simple as...
					//		srcPtr = (p + 4) & ~3;
					//	... but that'd fail if the Bits data originally came in via, say,
					//	mmap(2), and thus might not have been long-aligned to begin with.]
					
#if 0
					while ( srcPtr <= reinterpret_cast<const uint32_t *>( p ) )
						++srcPtr;
#else	// no looping needed
					srcPtr += 1 + (p - reinterpret_cast<const uchar *>( srcPtr )) / 4;
#endif
					
					// Now set up srcBits
					srcBits = (reinterpret_cast<const uchar *>( srcPtr ) - p) << 3;
					
					// Fill srcData from the previous block.
					srcData = EndianU32_BtoN( srcPtr[-1] ) << (32 - srcBits);
					
					// We might have an extra four bits.
					if ( runLength )
						{
						pixel = GETBITS( bitsPerPixel );
						pixel = colors[ pixel ];
						dstPtr = SetOnePixel( dstPtr );
						--runLength;
						}
					}
				else
					{
					// slower, general (non-aligned) case
					for ( ; runLength > 0; --runLength )
						{
						pixel = GETBITS( bitsPerPixel );
						pixel = colors[ pixel ];
						dstPtr = SetOnePixel( dstPtr );
						}
					}
				}
			}
		
		// offset the destination to the next row
		dstPtr += rowOffset;
		}
	
	return noErr;
}


/*
**	LoadImage()		Direct-to-CGImageRef flavor
**
**	This is just like the classic LoadImage(), with a few key differences:
**	1. It does not use any DTSImages or CLImages.  Instead you supply a PictDef
**		from which it reads the Bits and Colors IDs, as well as the pdFlags
**		(but see LoadImageX() below for a more convenient interface).
**	2. It produces a CGImageRef as output, which the caller must release.
**	3. Since it has no CLImage to play with, it cannot verify image checksums,
**		and therefore never returns kImageChecksumError.
**	4. For the same reason, it ignores any LightingData associated with the PictDef.
*/
DTSError
LoadImage( const PictDef& pdef, CGImageRef * oImg, DTSKeyFile * file,
			int nColr /* = 0 */, const uchar * custColrs /* = nullptr */ )
{
	BitsDef * bits = nullptr;
	uchar colors[ 256 ];
	
	// load the picture bits (good candidate for mmap(2)... someday)
	DTSError result = TaggedReadAlloc( file, kTypePictureBits, pdef.pdBitsID,
						(void *&) bits, "LoadImageBits" );
	
	// load the colors
	if ( noErr == result )
		{
		memset( colors, 0, sizeof colors );
		result = file->Read( kTypePictureColors, pdef.pdColorsID, colors, sizeof colors );
		}
	
#if 0 // this would need an actual CLImage*
	// assume there's no lighting data
	bool bHadLights = false;
	LightingData ld;
	memset( &ld, 0, sizeof ld );
	
	// but if there is, load it
	if ( noErr == result
	&&   pdef.pdLightingID != 0 )
		{
		result = file->Read( kTypeLightingData, pdef.pdLightingID, &ld, sizeof ld );
		if ( noErr == result )
			{
			bHadLights = true;
			BigToNativeEndian( &ld );
			}
		}
	
	// verify the checksum
	bool bChecksumErr = false;
	if ( not (pdef.pdFlags & kPictDefFlagNoChecksum) )
		{
		uint32_t sum;
		if ( noErr == result )
			result = CLImage::CalculateChecksum( file, bits, colors,
						bHadLights ? &ld : nullptr, &sum );
		
		if ( noErr != result
		&&   sum != pdef.pdChecksum )
			{
			bChecksumErr = true;
			}
		}
#endif  // 0
	
	// initialize the CGImage
	CGImageRef img = nullptr;
	UInt32 * data = nullptr;
	if ( noErr == result )
		{
#if DTS_LITTLE_ENDIAN
		bits->bdHeight   = EndianS16_BtoN( bits->bdHeight );
		bits->bdWidth    = EndianS16_BtoN( bits->bdWidth );
		bits->bdPackType = EndianS16_BtoN( bits->bdPackType );
		// bdReserved: unused, don't bother to swap
		// bdData[] -- will be handled elsewhere
#endif  // DTS_LITTLE_ENDIAN
		
		// allocate pixel buffer. This memory will be owned (ultimately) by the CGImageRef,
		// and will be automatically freed when that eventually gets released.
		size_t bytesPerRow = ((bits->bdWidth * 4) + 15) & ~15;
		size_t buffSize = bits->bdHeight * bytesPerRow;
		data = NEW_TAG("CGImagePixels") UInt32[ (buffSize + 3) / 4 ];
		if ( not data )
			result = memFullErr;
		
		// overwrite the default colors with the custom colors, if any
		// decompress bits into buffer
		if ( noErr == result )
			{
			if ( pdef.pdFlags & kPictDefCustomColors )
				memcpy( colors, custColrs, nColr );
			
			if ( kPackType1 == bits->bdPackType )
				result = DecompressImage1Alpha( data, bits, pdef.pdFlags, bytesPerRow, colors );
			else
				result = -1;
			}
		
		// create a data provider for that buffer
		CGDataProviderRef provider = nullptr;
		if ( noErr == result )
			{
			provider = CGDataProviderCreateWithData( nullptr, data, buffSize, MyImageReleaserCB );
			__Check( provider );
			}
		
		// the provider now owns 'data'
		if ( provider )
			data = nullptr;
		
		// see above
		CGColorSpaceRef cspace = CGColorSpaceCreateDeviceRGB();
		
		// combine all the ingredients into a CGImage
		if ( noErr == result && provider && cspace )
			{
			// parameters of the CGImage
			enum {
				bitsPerComponent	= 8,
				numComponents		= 4,	// A,R,G,B
				bitsPerPixel		= bitsPerComponent * numComponents,
				
				// its CGImageAlphaInfo, which depends on DT2CG_FMT
#if DT2CG_FMT == D2C_ARGB
				kAlfaInfo			= kCGImageAlphaFirst,	// ARGB
#elif DT2CG_FMT == D2C_RGBA
				kAlfaInfo			= kCGImageAlphaLast,	// RGBA
#else
# error "unimplemented pixel format"
#endif
				
				// and its endianness
#if FORCE_BIG_ENDIAN_FOR_NSIMAGE
				kEndianInfo			= kCGBitmapByteOrder32Big,
#else
				kEndianInfo			= kCGBitmapByteOrder32Host,
#endif
				
				// its CGBitmapInfo, which combines the previous two
				kBitmapInfo			= kAlfaInfo | kEndianInfo
				};
			
			img = CGImageCreate( bits->bdWidth, bits->bdHeight,
						bitsPerComponent,
						bitsPerPixel,
						bytesPerRow,
						cspace,
						kBitmapInfo,
						provider,
						nullptr,	// decode array
						false,		// interpolate?
						kCGRenderingIntentDefault );
			__Check( img );
			}
		
		CGColorSpaceRelease( cspace );
		CGDataProviderRelease( provider );
		}
	
	delete[] reinterpret_cast<char *>( bits );
	
	// if we failed to create the provider, don't leak the pixel buffer
	delete[] data;
	
	// undetected failure?
	if ( noErr == result && not img )
		result = coreFoundationUnknownErr;
	
//	if ( bChecksumErr && noErr == result )
//		result = kImageChecksumError;
	
	*oImg = img;
	
	return result;
}


/*
**	LoadImageX()
**
**	Mostly just a convenience wrapper for the previous routine.
**	This one takes a pictID, and produces a CGImageRef.
**	If the 'transparent' flag is set, overrides the corresponding bit in the pdFlags:
**	useful for known mobiles, which the client always draws transparent, regardless of
**	the flag setting.
*/
DTSError
LoadImageX( DTSKeyID pictID, CGImageRef * oImg, DTSKeyFile * file,
			bool transparent /* = false */,
			int numColors /* = 0 */,
			const uchar * custColrs /* = nullptr */ )
{
	PictDef pd;
	DTSError result = file->Read( kTypePictureDefinition, pictID, &pd, sizeof pd );
	if ( result )
		return result;
	
#if DTS_LITTLE_ENDIAN
	BigToNativeEndian( &pd );
#endif
	
	if ( transparent )
		pd.pdFlags |= kPictDefFlagTransparent;
	
	return LoadImage( pd, oImg, file, numColors, custColrs );
}
#endif  // DO_DIRECT_TO_CGIMAGE


#pragma mark -
#pragma mark Endian Support

#if DTS_LITTLE_ENDIAN

// shorthand
#define B2N( x )	(x) = BigToNativeEndian( x )
#define N2B( x )	(x) = NativeToBigEndian( x )


/*
**	BigToNativeEndian( PictDef * )
*/
void
BigToNativeEndian( PictDef * pd )
{
	B2N( pd->pdVersion );
	B2N( pd->pdBitsID );
	B2N( pd->pdColorsID );
	B2N( pd->pdChecksum );
	B2N( pd->pdFlags );
	B2N( pd->pdUnusedFlags );
	B2N( pd->pdUnusedFlags2 );
	B2N( pd->pdLightingID );
	B2N( pd->pdPlane );
	B2N( pd->pdNumFrames );
	B2N( pd->pdNumAnims );
	
	for ( int i = 0; i < pd->pdNumAnims; ++i )
		B2N( pd->pdAnimFrameTable[i] );
}


/*
**	NativeToBigEndian( PictDef * )
*/
void
NativeToBigEndian( PictDef * pd )
{
	N2B( pd->pdVersion );
	N2B( pd->pdBitsID );
	N2B( pd->pdColorsID );
	N2B( pd->pdChecksum );
	N2B( pd->pdFlags );
	N2B( pd->pdUnusedFlags );
	N2B( pd->pdUnusedFlags2 );
	N2B( pd->pdLightingID );
	N2B( pd->pdPlane );
	N2B( pd->pdNumFrames );
	
	int16_t nAnims = pd->pdNumAnims;
	pd->pdNumAnims = NativeToBigEndian( nAnims );
	
	for ( int i = 0; i < nAnims; ++i )
		N2B( pd->pdAnimFrameTable[i] );
}

/*
**	NativeToBigEndian( LightingData * )
*/
void
NativeToBigEndian( LightingData * ld )
{
	// ilCValue is OK as-is
	N2B( ld->ilPlane );
	N2B( ld->ilRadius );
}


/*
**	BigToNativeEndian( LightingData * )
*/
void
BigToNativeEndian( LightingData * ld )
{
	// skip ilCValue
	B2N( ld->ilPlane );
	B2N( ld->ilRadius );
}
#endif  // DTS_LITTLE_ENDIAN


