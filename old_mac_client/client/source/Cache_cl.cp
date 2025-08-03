/*
**	Cache_cl.cp		ClanLord
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
#include "Movie_cl.h"
#include "LaunchURL_cl.h"
#ifdef USE_OPENGL
# include "OpenGL_cl.h"
#endif	// USE_OPENGL


/*
**	Entry Routines
*/
/*
void				InitCaches();
ImageCache *		CachePicture( DTSKeyID id );
ImageCache *		CachePicture( DescTable * desc );
ImageCache * 		CachePicture( DTSKeyID id, DTSKeyID * cacheID, int nColors, const uchar * colors );
ImageCache *		CachePictureNoFlush( DTSKeyID id );
void				UpdateAnims();
void				FlushCaches( bool bEverything );
DTSError			HandleChecksumError( const CLImage * image );
void				ShowChecksumError( DTSKeyID id );
DTSError			ChecksumUsualSuspects()
*/


/*
**	Internal Definitions
*/
const int16_t sUsualImageSuspects[] =
{
	// high-plane black images
	150,
	254,
	285,
	362, 363, 364, 365, 366, 367, 368, 369, 370, 371, 372, 373,
	1266,
	2317,
	2455,
	3381,
	// pebbles
	431, 432, 433, 434, 435, 436,
	// PF marks
	395, 396, 397, 398, 399, 401, 402,
	1789,
	// terminator
	0
};


// pseudo-ID of a non-existent cache entry
const DTSKeyID	kNoSuchCacheID = DTSKeyID(0x80000000U);


/*
**	Internal Routines
*/
static DTSError	LoadRequestData( DTSKeyID id, PictDef * pd );
static DTSError	LoadCachePicture( DTSKeyID id, const PictDef * pd, ImageCache ** cache,
								  int numColors, const uchar * colors );
static void		CacheInit( ImageCache * cache );
#ifndef IRREGULAR_ANIMATIONS
static void		Update1Anim( ImageCache * cache );
#endif  // IRREGULAR_ANIMATIONS
static void		MyNewHandler();


/*
**	Internal Variables
*/
static DTSKeyID				gCacheID;
uint						CacheObject::sCachedImageCount = 0;
static std::new_handler		gOldNewHandler;


/*
**	InitCaches()
**
**	initialize the caches
*/
void
InitCaches()
{
	gCacheID = kNoSuchCacheID;
	
	CacheObject::sCachedImageCount = 0;
	
	gOldNewHandler = std::set_new_handler( MyNewHandler );
}


/*
**	MyNewHandler()
**
**	free memory
*/
void
MyNewHandler()
{
	// find something we can delete
	if ( gRootCacheObject->RemoveOneObject() )
		return;
	
	// ugh, didn't find something we can delete
	// call the old new_handler if there was one
	std::new_handler oldnh = gOldNewHandler;
	if ( oldnh )
		{
		(*oldnh)();
		return;
		}
	
	// we're dead.
	// stop playing the game.
	// attempt to warn the user.
	// attempt to bail gracefully.
	// the caller should certainly be able to handle new returning NULL.
	if ( gPlayingGame )
		{
		gPlayingGame = false;
		ShowInfoText( "\r" );
		ShowInfoText( "*** Out of memory.\r*** It is likely you will need to quit now.");
		}
	else
		gDoneFlag = true;
	
	std::set_new_handler( nullptr );
}


/*
**	CacheObject::~CacheObject()
**
**	deconstructor
*/
CacheObject::~CacheObject()
{
}


/*
**	CacheObject::CanBeDeleted()
**
**	default routine.
**	returns true if the object can be deleted.
**	false if it cannot currently be deleted.
**	like say, something's pointing at it.
**	or a sound is still playing.
*/
bool
CacheObject::CanBeDeleted() const
{
	return true;
}


/*
**	CacheObject::Touch()
**
**	touch this object so it is the last one to be deleted
*/
void
CacheObject::Touch()
{
	GoToBack( gRootCacheObject );
}


/*
**	CacheObject::RemoveOneObject()		[static]
**	find the first deletable cache object. kill it.
*/
bool
CacheObject::RemoveOneObject()
{
	// find something we can delete
	for ( CacheObject * walk = gRootCacheObject;  walk;  walk = walk->linkNext )
		{
		if ( walk->CanBeDeleted() )
			{
			walk->Remove( gRootCacheObject );
			
			// keep the books
			if ( walk->IsImage() )
				--walk->sCachedImageCount;
			
			delete walk;
			return true;
			}
		}
	return false;
}


/*
**	CachePicture()
**
**	using default colors
*/
ImageCache *
CachePicture( DTSKeyID id )
{
	// if the image is already in the cache then we're done
	// look for the picture in the cache and move it to the end of the list
	ImageCache * cache = nullptr;
	for ( CacheObject * walk = gRootCacheObject;  walk;  walk = walk->linkNext )
		{
		if ( CacheObject::kCacheTypeImage == walk->coType )
			{
			cache = static_cast<ImageCache *>( walk );
			if ( cache->icImage.cliPictDefID == id )
				{
				cache->Touch();
				return cache;
				}
			}
		}
	
	// make sure we have the data
	PictDef pd;
	DTSError result = LoadRequestData( id, &pd );
	
	// load the data and decompress the image
	if ( noErr == result )
		result = LoadCachePicture( id, &pd, &cache, 0, nullptr );
	
	// init the cache and put it in the list
	if ( noErr == result )
		CacheInit( cache );
	
	return cache;
}


/*
**	CachePicture()
**
**	for a mobile, using (possibly) customized colors
*/
ImageCache *
CachePicture( DTSKeyID pictID, DTSKeyID * cacheID, int numColors, const uchar * colors )
{
	if ( 0 == numColors )
		{
		// fall back to non-custom-color method
		return CachePicture( pictID );
		}
	
	// there are custom colors
	// check if the thing is already in the color cache
	int cachedTotal = 0;
	
	for ( CacheObject * walk = gRootCacheObject;  walk;  walk = walk->linkNext )
		{
		if ( CacheObject::kCacheTypeImageColor == walk->coType )
			{
			++cachedTotal;
			
			ImageColorCache	* ccache = static_cast<ImageColorCache *>( walk );
			if ( pictID != *cacheID
			&&	 ccache->icImage.cliPictDefID == *cacheID )
				{
//				ShowMessage( "Had %d in cached form", (int) pictID );
				ccache->Touch();
				return ccache;
				}
			
			if ( pictID == *cacheID )
				{
				if ( ccache->icImageID == pictID
				&&	 ccache->icNumColors == numColors
				&&	 0 == memcmp( ccache->icColors, colors, numColors ) )
					{
//					ShowMessage( "Found already colored %d", (int) pictID );
					*cacheID = ccache->icImage.cliPictDefID;
					ccache->Touch();
					return ccache;
					}
				}
			}
		}
	
//	ShowMessage( "Allocate colored %d, %d cached", int( pictID ), cachedTotal );
	
	// assign a new cache-ID
	if ( 0 == (*cacheID & kNoSuchCacheID) )
		*cacheID = gCacheID++;
	
	// make sure we have this data
	PictDef pd;
	DTSError result = LoadRequestData( pictID, &pd );
	
	// load the data and deal with memory issues
	ImageCache * cache = nullptr;
	if ( noErr == result )
		result = LoadCachePicture( pictID, &pd, &cache, numColors, colors );
	
	// init the cache and put it in the list
	if ( noErr == result )
		{
		// modify the cache-ID
		cache->icImage.cliPictDefID = *cacheID;
		CacheInit( cache );
		}
	
	return cache;
}


/*
**	CachePicture()
**
**	convenience routine for descriptors
*/
ImageCache *
CachePicture( DescTable * desc )
{
	return CachePicture( desc->descID, &desc->descCacheID, desc->descNumColors, desc->descColors );
}


/*
**	LoadRequestData()
**
**	make sure we have locally all the data we need
*/
DTSError
LoadRequestData( DTSKeyID id, PictDef * pd )
{
	// first check if we have the picture definition
	DTSError result = gClientImagesFile.Exists( kTypePictureDefinition, id );
	
	// we don't have it
	if ( result != noErr )
		return result;
	
	// we do have it, so read it
	result = gClientImagesFile.Read( kTypePictureDefinition, id, pd, sizeof *pd );
#if DTS_LITTLE_ENDIAN
	BigToNativeEndian( pd );
#endif
	
	// we do have it but we couldn't read it.
	// shit, now what?
	if ( result != noErr )
		return result;
	
	// check if we have the bits
	id = pd->pdBitsID;
	result = gClientImagesFile.Exists( kTypePictureBits, id );
	
	// check if we have the colors
	if ( noErr == result )
		{
		id = pd->pdColorsID;
		result = gClientImagesFile.Exists( kTypePictureColors, id );
		}
	
	// check if we have the lighting (if any)
	if ( noErr == result )
		{
		id = pd->pdLightingID;
		if ( 0 != id )
			result = gClientImagesFile.Exists( kTypeLightingData, id );
		}
	
	return result;
}


/*
**	LoadCachePicture()
**
**	allocate a cache record
**	load the image
*/
DTSError
LoadCachePicture( DTSKeyID id, const PictDef * pd, ImageCache ** pc, int numColors,
				  const uchar * colors )
{
	// begin the mess
	ImageCache * cache = nullptr;
	DTSError result = noErr;
	
	// allocate an image cache record
//	if ( noErr == result )
		{
		// we have colors. allocate a special cached object
		if ( numColors )
			{
			if ( ImageColorCache * ccache = NEW_TAG( "ImageColorCache" ) ImageColorCache )
				{
				ccache->icImageID	= id;
				ccache->icNumColors = numColors;
				memcpy( ccache->icColors, colors, numColors );
				cache = ccache;
				}
			}
		else
			cache = NEW_TAG("ImageCache") ImageCache;
		
		if ( not cache )
			result = memFullErr;
		}
	
	// load the image
	if ( noErr == result )
		{
		cache->icImage.cliPictDefID = id;
		cache->icImage.cliPictDef   = * pd;
		result = LoadImage( &cache->icImage, &gClientImagesFile, numColors, colors );
		}
	
	// verify the checksum
	if ( kImageChecksumError == result )
		{
		if ( not (cache->icImage.cliPictDef.pdFlags & kPictDefFlagNoChecksum)
		&&	 not gChecksumMsgGiven )
			{
			result = HandleChecksumError( &cache->icImage );
			}
		}
	
	// recover from an error
	if ( result != noErr )
		{
		delete cache;
		cache = nullptr;
		}
	
#ifdef TEMP_LIGHTING_DATA
	FillTempLightingDataStruct( cache->icImage );
#endif	// TEMP_LIGHTING_DATA
	
	// return the cache record and the result
	*pc = cache;
	return result;
}


/*
**	HandleChecksumError()
**
**	report bad image checksum
*/
DTSError
HandleChecksumError( const CLImage * image )
{
	DTSKeyID id = image->cliPictDefID;
	DTSError result = kImageChecksumError;
	
#ifdef DEBUG_VERSION
	result = noErr;
	GenericError( "Your image file appears to be corrupted (image %d).", int(id) );
	ShowMessage( "** Corrupt image! ID %d, file %8.8x, vers %u, "
				 "flags %u %u %u, light %d, plane %d, frames %d, anim %d",
		int( id ),
		uint( image->cliPictDef.pdChecksum ),
		uint( image->cliPictDef.pdVersion ),
		uint( image->cliPictDef.pdFlags ),
		uint( image->cliPictDef.pdUnusedFlags ),
		uint( image->cliPictDef.pdUnusedFlags2 ),
		int( image->cliPictDef.pdLightingID ),
		int( image->cliPictDef.pdPlane ),
		int( image->cliPictDef.pdNumFrames ),
		int( image->cliPictDef.pdNumAnims ) );
#else
	ShowChecksumError( id );
#endif // DEBUG_VERSION
	
	return result;
}


/*
**	ShowChecksumError()
**
**	error dialog
*/
void
ShowChecksumError( DTSKeyID id )
{
	char buff[ 256 ];
	snprintf( buff, sizeof buff,
		"Your image file appears to be corrupted (-%d).\r"
		"Please download a new copy from %s.", int( id ), kDataURL );
	ShowInfoText( buff );
	GenericError( "%s", buff );
	
	gChecksumMsgGiven = true;
	if ( CCLMovie::IsRecording() )
		CCLMovie::StopMovie();
	gPlayingGame = false;
}


/*
**	ChecksumUsualSuspects()
**
**	verify checksums for some of the most commonly modified images
**	unfortunate that this fills up the cache with images that may not be needed
*/
DTSError
ChecksumUsualSuspects()
{
	DTSError result = noErr;
	for ( const int16_t * p = sUsualImageSuspects; *p; ++p )
		{
		(void) CachePicture( static_cast<DTSKeyID>( *p ) );
		if ( gChecksumMsgGiven )
			{
			result = kImageChecksumError;
			break;
			}
		}
	
	return result;
}


/*
**	CacheInit()
**
**	initialize the cache
*/
void
CacheInit( ImageCache * cache )
{
	cache->icImage.cliImage.GetBounds( &cache->icBox );
	int height = cache->icBox.rectBottom;
	int frames = cache->icImage.cliPictDef.pdNumFrames;
	if ( frames > 1 )
		height /= frames;
	
	cache->icHeight = height;
	cache->icUsage  = 0;
	
	cache->InstallLast( gRootCacheObject );
	++CacheObject::sCachedImageCount;
	
#ifdef IRREGULAR_ANIMATIONS
	cache->UpdateAnim();
#else
	Update1Anim( cache );
#endif	// IRREGULAR_ANIMATIONS
	
	// ShowMessage( "Caching picture id %d.", (int) cache->icImage.cliPictDefID );
}


/*
**	UpdateAnims()
**
**	update the animation frames
*/
void
UpdateAnims()
{
	for ( CacheObject * walk = gRootCacheObject;  walk;  walk = walk->linkNext )
		{
		if ( CacheObject::kCacheTypeImage == walk->coType )
			{
			ImageCache * iwalk = static_cast<ImageCache *>( walk );
			
#ifdef IRREGULAR_ANIMATIONS
			iwalk->UpdateAnim();
#else
			Update1Anim( iwalk );
#endif	// IRREGULAR_ANIMATIONS
			}
		}
}


#ifdef IRREGULAR_ANIMATIONS
/*
**	ImageCache::UpdateAnim()
**
**	update one animation (randomly, if necessary)
*/
void
ImageCache::UpdateAnim( bool isRandom /* = false */ )
{
	// is this image animated?
	int count = icImage.cliPictDef.pdNumAnims;
	if ( count <= 0 )
		return;
	
	// go to the next frame
	int aframe;
	if ( isRandom )
		aframe = GetRandom( count );
	else
		aframe = gFrameCounter % count;
	
	// get the picture frame
	int pframe = icImage.cliPictDef.pdAnimFrameTable[ aframe ];
	
	// check for bad data
	if ( pframe < 0
	||   pframe >= icImage.cliPictDef.pdNumFrames )
		{
		pframe = 0;
		ShowMessage( "* Picture %d has bad animation frames.", (int) icImage.cliPictDefID );
		}
	
	// setup the image bounds
	icBox.rectTop    = pframe * icHeight;
	icBox.rectBottom = icBox.rectTop + icHeight;
}

#else  // ! IRREGULAR_ANIMATIONS

/*
**	Update1Anim()
**
**	update one animation
*/
void
Update1Anim( ImageCache * cache )
{
	// is this image animated?
	int count = cache->icImage.cliPictDef.pdNumAnims;
	if ( count <= 0 )
		return;
	
	// go to the next frame
	int aframe = gFrameCounter % count;
	
	// get the picture frame
	int pframe = cache->icImage.cliPictDef.pdAnimFrameTable[ aframe ];
	
	// check for bad data
	if ( pframe < 0
	||   pframe >= cache->icImage.cliPictDef.pdNumFrames )
		{
		pframe = 0;
		ShowMessage( "* Picture %d has bad animation frames.", (int) cache->icImage.cliPictDefID );
		}
	
	// setup the image bounds
	int height = cache->icHeight;
	int top    = pframe * height;
	cache->icBox.rectTop    = top;
	cache->icBox.rectBottom = top + height;
}
#endif	// IRREGULAR_ANIMATIONS


/*
**	ImageColorCache::~ImageColorCache()
**
*/
ImageColorCache::~ImageColorCache()
{
//	ShowMessage( "Deallocated %d", (int) icImageID);
}


/*
**	ImageCache::~ImageCache()
**
**	dispose of the memory allocated for the image bits
*/
ImageCache::~ImageCache()
{
	icImage.cliImage.DisposeBits();
	
#ifdef USE_OPENGL
	delete textureObject;
#endif	// USE_OPENGL
}


/*
**	ImageCache::CanBeDeleted()
**
**	return true if the image is not in use
**	and can be flushed
*/
bool
ImageCache::CanBeDeleted() const
{
	return 0 == icUsage;
}


/*
**	ImageCache::DisableFlush()
**
**	disable flushing
*/
void
ImageCache::DisableFlush()
{
	++icUsage;
}


/*
**	ImageCache::EnableFlush()
**
**	enable flushing
*/
void
ImageCache::EnableFlush()
{
	uint usage = icUsage;
	if ( usage > 0 )
		icUsage = usage - 1;
}


/*
**	CachePictureNoFlush()
**
**	cache the picture and disable flushing
*/
ImageCache *
CachePictureNoFlush( DTSKeyID id )
{
	if ( ImageCache * ic = CachePicture( id ) )
		{
		ic->DisableFlush();
		return ic;
		}
	
	return nullptr;
}


/*
**	FlushCaches()
**
**	flush all of the caches and log to the memory leak finding file
*/
void
FlushCaches( bool bEverything /* = false */ )
{
	CacheObject * next;
	for ( CacheObject * walk = gRootCacheObject;  walk;  walk = next )
		{
		next = walk->linkNext;
		if ( bEverything  ||  walk->CanBeDeleted() )
			{
			// remove it
			walk->Remove( gRootCacheObject );
			
			// keep the books
			if ( walk->IsImage() )
				--walk->sCachedImageCount;
			
			// and now zot it
			delete walk;
			}
		}
	
#if defined( DEBUG_VERSION ) && defined( USE_OPENGL )
	gLargestTextureDimension = 0;
	gLargestTexturePixels = 0;
#endif	// DEBUG_VERSION && USE_OPENGL
}


#ifdef USE_OPENGL
/*
**	DeleteTextureObjects()
**
**	clean out all GL texture objects associated with every cached image
*/
void
DeleteTextureObjects()
{
	for ( CacheObject * walk = gRootCacheObject;  walk;  walk = walk->linkNext )
		{
		if ( walk->IsImage() )
			{
#if defined( DEBUG_VERSION )	// USE_OPENGL already defined
			// for peace of mind, use RTTI to verify that IsImage() is sane
 			ImageCache * ic = dynamic_cast<ImageCache *>( walk );
 				// null if cast fails RTTI check
			if ( not ic )
				{
				ShowMessage( "fake ImageCache * detected" );
				continue;
				}
#else
			ImageCache * ic = static_cast<ImageCache *>( walk );
#endif	// DEBUG_VERSION
			
			delete ic->textureObject;
			ic->textureObject = nullptr;
			}
		}
#ifdef DEBUG_VERSION
	gLargestTextureDimension = 0;
	gLargestTexturePixels = 0;
#endif	// DEBUG_VERSION
}


/*
**	DecrementTextureObjectPriorities()
**
**	manage GL texture priorities
*/
void
DecrementTextureObjectPriorities()
{
	// bail if we're not doing that
	if ( not gOGLManageTexturePriority )
		return;
	
	// for each image in the cache...
	for ( CacheObject * walk = gRootCacheObject;  walk;  walk = walk->linkNext )
		{
		if ( walk->IsImage() )
			{
#ifdef DEBUG_VERSION	// USE_OPENGL already defined
			ImageCache * ic = dynamic_cast<ImageCache *>( walk );
				// null if cast fails RTTI check
			if ( not ic )
				{
				ShowMessage( "fake ImageCache * detected" );
				continue;
				}
#else
			ImageCache * ic = static_cast<ImageCache *>( walk );
#endif	// DEBUG_VERSION
			
			// decrement the texture priority
			if ( ic->textureObject )
				ic->textureObject->decrementTextureObjectPriority();
			}
		}
}


/*
**	deleteAllUnusedTextures()
**
**	look for GL textures that aren't in use, and dispose of them
**	returns true if any were deleted
*/
bool
deleteAllUnusedTextures()
{
	// abort if we are not managing priorities
	bool result = false;
	if ( not gOGLManageTexturePriority )
		return result;
	
	// for each image in the cache...
	for ( CacheObject * walk = gRootCacheObject;  walk;  walk = walk->linkNext )
		{
		if ( walk->IsImage() )
			{
#ifdef DEBUG_VERSION	// USE_OPENGL already defined
			ImageCache * ic = dynamic_cast<ImageCache *>( walk );
				// null if cast fails RTTI check
			if ( not ic )
				{
				ShowMessage( "fake ImageCache * detected" );
				continue;
				}
			else
#else
			ImageCache * ic = static_cast<ImageCache *>( walk );
#endif	// DEBUG_VERSION
			
			if ( ic->textureObject )
				{
				if ( ic->textureObject->deleteUnusedTextures() )
					result = true;
				}
			}
		}
	
	return result;
}
#endif	// USE_OPENGL


