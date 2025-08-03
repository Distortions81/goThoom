/*
**	Memory_dts.cp		dtslib2
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

#if DEBUG_VERSION_NEW

#include "Memory_dts.h"
#include "Memory_cmn.h"

//#define DEBUG_VERSION_NEW 1


/*
**	Entry Routines
**
**	NewAlloc();
**	NewFree();
**	DumpBlocks();
**	DumpBlocksSummary();
**	DumpBlocksCommon();
*/

/*
**	These entries are declared here, but not in any header file,
**	because they're not meant to be externally visible at all.
**	Only the routines in New_dts.cp should have access to them.
*/
#ifdef DEBUG_VERSION_TAGS
void *	NewAlloc( size_t size, const char * tagString ) DOES_NOT_THROW;
#else
void *	NewAlloc( size_t size ) DOES_NOT_THROW;
#endif
void	NewFree( void * ptr ) DOES_NOT_THROW;

using std::snprintf;


/*
**	Assumptions:
**
**	alignment is on 8 byte boundaries.
*/


/*
 **	When we need to, we allocate a pool with malloc:
**
**		+ - - - - - - - - +
**		: pool link       :		debug version only; for the gRootPool chain
**		: filler          :		[sizeof(double) - sizeof(pool link)]
**		+-----------------+
**		| 0x0DE1FE1FL     |		debug version only
**		| prevsize = 0    |		there is no previous block
**		| size     = size |		size of caller data + alignment bytes
**		| next     = nil  |
**		| prev     = nil  |
**		+ - - - - - - - - +
**		: tag string ptr  :		(if DEBUG_TAGS)
**		+ - - - - - - - - +
**		| used size       |		size requested by caller
**		| 0x0DE1FE1FL     |		debug version only
**		+-----------------+
**		| caller data     |		pointer returned to the caller
**		|                 |
**		|                 |
**		+-----------------+
**		| alignment bytes |
**		+-----------------+
**		| 0x0DE1FE1FL     |		debug version only
**		| prevsize = size |
**		| size     = 0    |		there is no next block
**		| next     = nil  |		debug version only
**		| prev     = nil  |		debug version only
**		| 0x0DE1FE1FL     |		debug version only
**		+-----------------+
*/


/*
**	Definitions
*/
struct NAFreeList
{
#ifdef DEBUG_VERSION_NEW
	uintptr_t		flBlockMarker1;		// mark the start of a block = kBlockMarker
#endif
	size_t			flPrevSize;			// the size of the previous block
	size_t			flSize;				// the size of this block
	NAFreeList *	flNext;				// pointer to the next free block
	NAFreeList *	flPrev;				// pointer to the previous free block
										// or nil if the block is in use
#ifdef DEBUG_VERSION_TAGS
	const char *	flTagString;		// pointer to constant string
#endif
#ifdef DEBUG_VERSION_NEW
	size_t			flUsedSize;			// the unmodified size of the block
	uintptr_t		flBlockMarker2;		// mark the end of a block = kBlockMarker
#endif
};


const size_t	kHeaderSize			= sizeof( NAFreeList );
const size_t	kMaxBlockSize		= 0x80000000U - 2 * kHeaderSize;

const size_t	kMinPoolSize		= 256 * 1024;
const size_t	kMinBlockSize		= 32;
const size_t	kBlockAlignment		= kMinBlockSize - 1;
const int		kNumBuckets			= 26;


#ifdef DEBUG_VERSION_NEW
	// Newly-allocated blocks are originally filled with this value.
const int	kZapUninitialized	= 0x0F1;
	// Released blocks are filled with this value.
const int	kZapReleased		= 0x0F3;

// See diagram above.
# if ! __LP64__
const uintptr_t	kBlockMarker		= 0x0DE1FE1FU;
# else  // __LP64__
const uintptr_t	kBlockMarker		= 0x0DE1FE1F0DE1FE1FU;
# endif  // ! __LP64__

const size_t	kPoolHeaderSize		= sizeof(double);		// ensure maximal alignment
#else
const size_t	kPoolHeaderSize		= 0;
#endif  // ! DEBUG_VERSION_NEW

// if you suspect the coder to be guilty of utter stupidity,
// turning this on may help you prove your case.
//# define DUMBNESS			1


/*
**	Internal Routines
*/
static NAFreeList *		NANewPool( size_t size ) DOES_NOT_THROW;
static void				NAFreeBlock( NAFreeList * fl ) DOES_NOT_THROW;
#ifdef DEBUG_VERSION_NEW
static bool				NACheckOverwrite( const uchar * start, const uchar * stop ) DOES_NOT_THROW;
#endif
#ifdef DEBUG_VERSION_TAGS
static void				DumpBlocksCommon( const char * filename, bool verbose );
#endif
#ifdef DUMBNESS
static void				NADumbnessCheck() DOES_NOT_THROW;
#endif


/*
**	Variables
*/
static NAFreeList			gBuckets[ kNumBuckets ];
#ifdef DEBUG_VERSION_NEW
static void *				gRootPool;
static ulong				gAllocCounter;
static ulong				gPlatformAllocCtr;
static ulong				gFreeCounter;
static ulong				gPlatformFreeCtr;
#endif  // DEBUG_VERSION_NEW

// for communicating with new-handlers, GrowZoneProcs, and such
bool						bPoolFreed;


/*
**	NewAlloc()
**
**	allocate memory from an available pool
*/
#ifdef DEBUG_VERSION_TAGS
void * NewAlloc( size_t size, const char * tagString ) DOES_NOT_THROW
#else
void * NewAlloc( size_t size ) DOES_NOT_THROW
#endif
{
#ifdef DUMBNESS
	NADumbnessCheck();
#endif
	
#ifdef DEBUG_VERSION_NEW
	++gAllocCounter;
#endif
	
	// sanity check the size
	if ( size > kMaxBlockSize - 1 )
		return nullptr;
	
	// remember the original size
#ifdef DEBUG_VERSION_NEW
	size_t orgsize = size;
#endif
	
	// pin the block size to a minimum
	if ( size <= kMinBlockSize - 1 )
		size = kMinBlockSize;
	
	// add alignment bytes if needed
	size = ( size + kBlockAlignment ) & ~kBlockAlignment;
	
	// select a bucket based on the size
	NAFreeList * bucket = gBuckets;
	size_t bucketsize = kMinBlockSize;
	for (;;)
		{
		if ( bucketsize >= size )
			break;
		if ( bucketsize >= kMaxBlockSize / 2 )
			break;
		++bucket;
		bucketsize += bucketsize;
		}
	
	// is there a block in this bucket?
	NAFreeList * fl = bucket->flNext;
	if ( not fl )
		{
		// no, try the bigger buckets
		NAFreeList * startbucket = bucket;
		do
			{
			// did we run out of buckets?
			if ( bucket >= &gBuckets[kNumBuckets - 1] )
				{
				// allocate a new pool
				if ( size <= kMinPoolSize - 1 )
					fl = NANewPool( kMinPoolSize );
				if ( not fl )
					{
					fl = NANewPool( size );
					if ( not fl )
						{
						// last ditch effort
						// search the next smaller bucket
						// there might be one there that's big enough
						if ( startbucket > gBuckets )
							{
							bucket = startbucket - 1;
							fl = bucket->flNext;
							if ( fl )
								{
								for ( ;  ; fl = fl->flNext )
									{
									if ( fl == bucket )
										{
										fl = nullptr;
										break;
										}
									if ( fl->flSize >= size )
										{
										// found one!
										break;
										}
									}
								}
							}
						
						// give up, the memory is not available
						return nullptr;
						}
					}
				break;
				}
			
			// check the next bigger bucket
			++bucket;
			fl = bucket->flNext;
			}
		while ( not fl );
		}
	
#ifdef DUMBNESS
	// dumbness check
	if ( fl->flSize < size )
		PlatformNewErrMsg( "NewAlloc found a too-small block!" );
#endif
	
	// at this point we have a block
	// remove it from its bucket
	// and mark it as in-use
	NAFreeList * nextfl = fl->flNext;
	NAFreeList * prevfl = fl->flPrev;
	if ( nextfl != prevfl )
		{
		prevfl->flNext = nextfl;
		nextfl->flPrev = prevfl;
		}
	else
		{
		nextfl->flNext = nullptr;
		nextfl->flPrev = nullptr;
		}
	
#ifdef DUMBNESS
	fl->flNext = nullptr;		// ????
#endif
	fl->flPrev = nullptr;
	
	// check if the block is big enough to split
	if ( fl->flSize >= size + kHeaderSize + kMinBlockSize )
		{
		size_t splitsize = fl->flSize - size - kHeaderSize;
		// get pointer to the new header in the middle
		// and pointer to the trailing header
		// ie the header for the next block
		NAFreeList * splitfl = reinterpret_cast<NAFreeList *>(
			reinterpret_cast<char *>( fl ) + kHeaderSize + size );
		NAFreeList * trailfl = reinterpret_cast<NAFreeList *>(
			reinterpret_cast<char *>( splitfl ) + kHeaderSize + splitsize );
		
		// change the size of this block
		fl->flSize          = size;
		splitfl->flPrevSize = size;
		
		// change the size of the split block
		splitfl->flSize     = splitsize;
		trailfl->flPrevSize = splitsize;
		
		// insert the block markers into the new header
#ifdef DEBUG_VERSION_TAGS
		splitfl->flTagString    = "SplitBlock";
#endif
#ifdef DEBUG_VERSION_NEW
		splitfl->flBlockMarker1 = kBlockMarker;
		splitfl->flUsedSize     = 0;
		splitfl->flBlockMarker2 = kBlockMarker;
#endif
		
		// free the newly split block
		NAFreeBlock( splitfl );
		}
	
	// point to the block
	void * ptr = reinterpret_cast<char *>( fl ) + kHeaderSize;
	
#ifdef DEBUG_VERSION_TAGS
	// tag it
	fl->flTagString = tagString;
#endif
	
#ifdef DEBUG_VERSION_NEW
	// zap the returned block with the value meaning uninitialized
	fl->flUsedSize = orgsize;
	memset( ptr, kZapUninitialized, fl->flSize );
#endif
	
#ifdef DUMBNESS
	NADumbnessCheck();
#endif
	
	// return a pointer to the block
	return ptr;
}


/*
**	NewFree()
**
**	free memory
*/
void
NewFree( void * ptr ) DOES_NOT_THROW
{
	if ( not ptr )
		return;
	
#ifdef DEBUG_VERSION_NEW
	++gFreeCounter;
#endif
	
	// check every block in every pool for consistency
#ifdef DUMBNESS
	NADumbnessCheck();
#endif
	
	// point to the header
	NAFreeList * fl = reinterpret_cast<NAFreeList *>( static_cast<char *>( ptr ) - kHeaderSize );
	
#ifdef DEBUG_VERSION_NEW
	// check the block markers for consistency
	size_t size = fl->flSize;
	size_t orgsize = fl->flUsedSize;
	NAFreeList * trailfl = reinterpret_cast<NAFreeList *>( static_cast<char *>( ptr ) + size );
	if ( kBlockMarker !=      fl->flBlockMarker1
	||   kBlockMarker !=      fl->flBlockMarker2
	||   kBlockMarker != trailfl->flBlockMarker1
	||   kBlockMarker != trailfl->flBlockMarker2 )
		{
		PlatformNewErrMsg( "Bad block marker." );
		}
	
	// check for overwrites
	else
	if ( orgsize != 0
	&&   NACheckOverwrite( static_cast<uchar *>( ptr ) + orgsize,
						   static_cast<uchar *>( ptr ) + size ) )
		{
		PlatformNewErrMsg( "End of block overwritten." );
		}
	
	// check if this block has already been freed
	else
	if ( fl->flPrev )
		PlatformNewErrMsg( "Block double deleted." );
	else
#endif	// DEBUG_VERSION_NEW
		{
#ifdef DEBUG_VERSION_NEW
		// zap the block with the value meaning released
		memset( ptr, kZapReleased, size );
#endif	// DEBUG_VERSION_NEW
		
		NAFreeBlock( fl );
		
		// check every block in every pool again for consistency
#ifdef DUMBNESS
		NADumbnessCheck();
#endif
		}
}


/*
**	NANewPool()
**
**	allocate a pool of the requested size
**	assume the size is good
*/
NAFreeList *
NANewPool( size_t size ) DOES_NOT_THROW
{
	NAFreeList * fl = reinterpret_cast<NAFreeList *>(
							PlatformNewAlloc( size + 2 * kHeaderSize + kPoolHeaderSize ) );
#ifdef DEBUG_VERSION_NEW
	++gPlatformAllocCtr;
#endif
	
	if ( fl )
		{
#ifdef DEBUG_VERSION_NEW
		* reinterpret_cast<void **>( fl ) = gRootPool;
		gRootPool = reinterpret_cast<char *>( fl );
		fl = reinterpret_cast<NAFreeList *>( reinterpret_cast<char *>( fl ) + kPoolHeaderSize );
#endif
		
		// select a bucket based on the size
		NAFreeList * bucket = gBuckets;
		size_t bucketsize = 2 * kMinBlockSize;
		for (;;)
			{
			if ( bucketsize > size )
				break;
			if ( bucketsize >= kMaxBlockSize / 4 )
				break;
			++bucket;
			bucketsize += bucketsize;
			}
		
		// init this header
		// put the block in the bucket
		NAFreeList * prev = bucket->flPrev;
		fl->flPrevSize = 0;
		fl->flSize     = size;
		if ( prev )
			{
			prev->flNext   = fl;
			fl->flPrev     = prev;
			fl->flNext     = bucket;
			bucket->flPrev = fl;
			}
		else
			{
			fl->flNext     = bucket;
			fl->flPrev     = bucket;
			bucket->flNext = fl;
			bucket->flPrev = fl;
			}
		
		// init the trailer
		NAFreeList * trailfl = reinterpret_cast<NAFreeList *>(
									reinterpret_cast<char *>( fl ) + kHeaderSize + size );
		trailfl->flPrevSize = size;
		trailfl->flSize     = 0;
		trailfl->flPrev     = nullptr;
		trailfl->flNext     = nullptr;
		
		// add the block markers to the headers
		// zap the the entire pool with the value meaning uninitialized
#ifdef DEBUG_VERSION_TAGS
		fl     ->flTagString    = "PoolHeader";
		trailfl->flTagString    = "PoolTrailer";
#endif
#ifdef DEBUG_VERSION_NEW
		fl     ->flBlockMarker1 = kBlockMarker;
		fl     ->flUsedSize     = 0;
		fl     ->flBlockMarker2 = kBlockMarker;
		trailfl->flBlockMarker1 = kBlockMarker;
		trailfl->flUsedSize     = 0;
		trailfl->flBlockMarker2 = kBlockMarker;
		memset( reinterpret_cast<char *>( fl ) + kHeaderSize, kZapUninitialized, size );
#endif
		}
	
	return fl;
}


/*
**	NAFreeBlock()
**
**	free the block
*/
void
NAFreeBlock( NAFreeList * fl ) DOES_NOT_THROW
{
	NAFreeList * prevfl;
	NAFreeList * nextfl;
	
	size_t size = fl->flSize;
	NAFreeList * trailfl = reinterpret_cast<NAFreeList *>(
								reinterpret_cast<char *>( fl ) + kHeaderSize + size );
	
	// is there a previous block?
	size_t prevsize = fl->flPrevSize;
	if ( prevsize )
		{
		// is it free?
		prevfl = reinterpret_cast<NAFreeList *>(
						reinterpret_cast<char *>( fl ) - kHeaderSize - prevsize );
		NAFreeList * prevprevfl = prevfl->flPrev;
		if ( prevprevfl )
			{
			// remove it from its list
			nextfl = prevfl->flNext;
			if ( nextfl != prevprevfl )
				{
				prevprevfl->flNext = nextfl;
				nextfl->flPrev     = prevprevfl;
				}
			else
				{
				nextfl->flNext = nullptr;
				nextfl->flPrev = nullptr;
				}
			
			// zap the old previous header with the value meaning released
#ifdef DEBUG_VERSION_NEW
			memset( fl, kZapReleased, kHeaderSize );
#endif
			
			// merge them
			size               += prevsize + kHeaderSize;
			prevfl->flSize      = size;
			trailfl->flPrevSize = size;
			fl                  = prevfl;
			}
		}
	
	// is the next block free?
	prevfl = trailfl->flPrev;
	if ( prevfl )
		{
		// remove it from its list
		nextfl = trailfl->flNext;
		if ( nextfl != prevfl )
			{
			prevfl->flNext = nextfl;
			nextfl->flPrev = prevfl;
			}
		else
			{
			nextfl->flNext = nullptr;
			nextfl->flPrev = nullptr;
			}
		
		// zap the the old trailing header with the value meaning released
		// be careful to not step on any needed fields
		size_t trailsize = trailfl->flSize;
#ifdef DEBUG_VERSION_NEW
		memset( trailfl, kZapReleased, kHeaderSize );
#endif
		
		// merge them
		size               += trailsize + kHeaderSize;
		fl->flSize          = size;
		trailfl             = reinterpret_cast<NAFreeList *>(
									reinterpret_cast<char *>( fl ) + kHeaderSize + size );
		trailfl->flPrevSize = size;
		}
	
	// is the whole pool now free?
	if ( 0 == fl->flPrevSize
	&&   0 == trailfl->flSize )
		{
		bPoolFreed = true;
		
		void * pool = reinterpret_cast<char *>( fl ) - kPoolHeaderSize;
		
		// remove it from the linked list
#ifdef DEBUG_VERSION_NEW
		void * test;
		for ( void ** link = &gRootPool;  ;  link = reinterpret_cast<void **>( test ) )
			{
			test = *link;
			if ( not test )
				{
				PlatformNewErrMsg( "Pool not found in linked list." );
				break;
				}
			if ( test == pool )
				{
				*link = * reinterpret_cast<char **>( pool );
				break;
				}
			}
#endif	// DEBUG_VERSION_NEW
		
		// dispose of the now unused pool
		PlatformNewFree( pool );
#ifdef DEBUG_VERSION_NEW
		++gPlatformFreeCtr;
#endif
		}
	else
		{
		// change the tag to free
#ifdef DEBUG_VERSION_TAGS
		fl->flTagString = "FreeBlock";
#endif
		
		// select a bucket based on the size
		NAFreeList * bucket = gBuckets;
		size_t bucketsize = 2 * kMinBlockSize;
		for (;;)
			{
			if ( bucketsize > size )
				break;
			if ( bucketsize >= kMaxBlockSize / 4 )
				break;
			++bucket;
			bucketsize += bucketsize;
			}
		
		// add the block to the bucket
		// phd question: in order to minimize fragmentation...
		// do we add the block to the front or the end of the list?
		nextfl = bucket->flNext;
		// the point is moot, the list is empty
		if ( not nextfl )
			{
			fl->flNext     = bucket;
			fl->flPrev     = bucket;
			bucket->flNext = fl;
			bucket->flPrev = fl;
			}
		// compare the size of this block with the first one in the list
		else
		if ( size > nextfl->flSize )
			{
			// insert larger blocks at the end of the list
			prevfl = bucket->flPrev;
			prevfl->flNext = fl;
			fl->flPrev     = prevfl;
			fl->flNext     = bucket;
			bucket->flPrev = fl;
			}
		else
			{
			// insert smaller blocks at the start of the list
			nextfl->flPrev = fl;
			fl->flNext     = nextfl;
			fl->flPrev     = bucket;
			bucket->flNext = fl;
			}
		// this gives some degree of order to the free list.
		// ie smaller blocks are more likely to be before larger blocks.
		// because of our block selection criteria...
		// we'll get a "better fit" if we use smaller blocks.
		// the split blocks will be smaller and theoretically less useful.
		// and when this block is freed it is more likely to be able to
		// be merged with adjacent blocks.
		// that's the theory anyway.
		// i could be full of shit. ;->
		// -timmer
		}
}


#ifdef DUMBNESS
/*
**	NADumbnessCheck()
**
**	check the linked lists
*/
void
NADumbnessCheck() DOES_NOT_THROW
{
	for ( const NAFreeList * bucket = gBuckets;  bucket < &gBuckets[ kNumBuckets ];  ++bucket )
		{
		const NAFreeList * f0 = bucket;
		const NAFreeList * f1 = f0->flNext;
		if ( f1 )
			{
			for (;;)
				{
				if ( f1->flPrev != f0 )
					PlatformNewErrMsg( "prev/next mismatch!" );
				
				if ( f1 == bucket )
					break;
				f0 = f1;
				f1 = f0->flNext;
				
				// maybe validate block markers and/or sizes here?
				}
			}
		}
}
#endif	// DUMBNESS


#ifdef DEBUG_VERSION_NEW
/*
**	NACheckOverwrite()
**
**	check if the block has been overwritten
**	return true if it has been
**	return false if not
*/
bool
NACheckOverwrite( const uchar * ptr, const uchar * stop ) DOES_NOT_THROW
{
	// byte-wise compare loop OK, because we never check more than a few dozen bytes, at most
	for (  ;  ptr < stop;  ++ptr )
		{
		if ( *ptr != kZapUninitialized )
			return true;
		}
	return false;
}
#endif	// DEBUG_VERSION_NEW


#ifdef DEBUG_VERSION_TAGS

// Does printf() support the "'" format code? (C99 feature; inserts thousands-separator commas)
#if OLD_SUN || defined( __MSL__ )
	// old Solaris doesn't do it, nor does MSL
# undef HAVE_PRINTF_COMMA_FORMAT
#else
	// but others do
# define HAVE_PRINTF_COMMA_FORMAT	1
#endif // SUN, Mac


/*
**	DumpBlocks()
**
**	dump all blocks
*/
void
DumpBlocks( const char * filename )
{
	DumpBlocksCommon( filename, true );
}


/*
**	DumpBlocksSummary()
**
**	dump just the summary
*/
void
DumpBlocksSummary( const char * filename )
{
	DumpBlocksCommon( filename, false );
}


/*
**	DumpBlocksCommon()
**
**	dump all blocks
*/
void
DumpBlocksCommon( const char * filename, bool verbose )
{
	DTSFileSpec spec;
	spec.SetFileName( filename );
	
	int fileref;
	DTSError result = DTS_open( &spec, true, &fileref );
	if ( fnfErr == result )
		{
		result = DTS_create( &spec, 'TEXT' );
		if ( noErr == result )
			result = DTS_open( &spec, true, &fileref );
		}
	
	if ( noErr == result )
		result = DTS_seteof( fileref, 0 );
	
	if ( result != noErr )
		return;
	
	char buff[512];
	int len;
	const char * tagString;
	const void * pool;
	const NAFreeList * fl;
	
	ulong npools  = 0;
	ulong nfree   = 0;
	ulong nused   = 0;
	size_t memfree = 0;
	size_t memused = 0;
	
	// show all pools - verbose only
	// collect summary info
	for ( pool = gRootPool;  pool;  pool = * static_cast<const void * const *>( pool ) )
		{
		++npools;
		if ( verbose )
			{
			len = snprintf( buff, sizeof buff, "pool at 0x%p\n", pool );
			DTS_write( fileref, buff, uint( len ) );
			}
		
		fl = reinterpret_cast<const NAFreeList *>( 
				static_cast<const char *>( pool ) + kPoolHeaderSize );
		for (;;)
			{
			size_t size = fl->flSize;
			const NAFreeList * prev = fl->flPrev;
			if ( verbose )
				{
				len = snprintf( buff, sizeof buff,
#if __LP64__
						"%p %lX %lX %p %p",
#else
						"%p %.8lX %.8lX %p %p",
#endif
						fl,
						static_cast<ulong>( fl->flPrevSize ),
						static_cast<ulong>( size ),
						fl->flNext,
						prev );
				DTS_write( fileref, buff, uint( len ) );
#ifdef DEBUG_VERSION_NEW
				if ( fl->flBlockMarker1 != kBlockMarker )
					{
					len = snprintf( buff, sizeof buff, " (bad block marker1)" );
					DTS_write( fileref, buff, uint( len ) );
					}
				if ( fl->flBlockMarker2 != kBlockMarker )
					{
					len = snprintf( buff, sizeof buff, " (bad block marker2)" );
					DTS_write( fileref, buff, uint( len ) );
					}
#endif	// DEBUG_VERSION_NEW
				tagString = fl->flTagString;
				if ( tagString )
					{
					len = snprintfx( buff, sizeof buff, " %s", tagString );
					DTS_write( fileref, buff, uint( len ) );
					}
//				len = snprintf( buff, sizeof buff, "\n" );
				DTS_write( fileref, "\n", 1 );
				}
			if ( 0 == size )
				break;
			if ( prev )
				{
				++nfree;
				memfree += size;
				}
			else
				{
				++nused;
				memused += size;
				}
			
			fl = reinterpret_cast<const NAFreeList *>(
					reinterpret_cast<const char *>( fl ) + kHeaderSize + size );
			}
		}
	
	// show summary info
	len = snprintf( buff, sizeof buff,
#if HAVE_PRINTF_COMMA_FORMAT
			"%'lu pools; %'lu/%'lu free; %'lu/%'lu in use.\n",
#else
			"%lu pools; %lu/%lu free; %lu/%lu in use.\n",
#endif
			npools,
			nfree,
			static_cast<ulong>( memfree ),
			nused,
			static_cast<ulong>( memused ) );
	DTS_write( fileref, buff, uint( len ) );
	
	len = snprintf( buff, sizeof buff,
#if HAVE_PRINTF_COMMA_FORMAT
			"%'lu (%'lu) allocs (platform); %'lu (%'lu) frees\n\n",
#else
			"%lu (%lu) allocs (platform); %lu (%lu) frees\n\n",
#endif
			gAllocCounter,
			gPlatformAllocCtr,
			gFreeCounter,
			gPlatformFreeCtr );
	DTS_write( fileref, buff, uint( len ) );
	
	// show all of the free lists - verbose only
	if ( verbose )
		{
		const NAFreeList * bucket = gBuckets;
		size_t losize = kMinBlockSize;
		for ( int count = kNumBuckets;  count > 0;  --count )
			{
			size_t hisize = losize + losize;
			
			len = snprintf( buff, sizeof buff,
						"bucket size %.8lX to %.8lX:\n",
						static_cast<ulong>( losize ),
						static_cast<ulong>( hisize - 1 ) );
			DTS_write( fileref, buff, uint( len ) );
			const NAFreeList * walk = bucket->flNext;
			if ( walk )
				{
				for ( ;  walk != bucket;  walk = walk->flNext )
					{
					char ch = ' ';
					size_t size = walk->flSize;
					if ( size <  losize
					||   size >= hisize )
						{
						ch = '*';
						}
					len = snprintf( buff, sizeof buff,
#if __LP64__
							"  %p %lX %c\n",
#else
							"  %p %.8lX %c\n",
#endif
							walk,
							static_cast<ulong>( size ),
							ch );
					DTS_write( fileref, buff, uint( len ) );
					}
				}
			
			++bucket;
			losize = hisize;
			}
		DTS_write( fileref, "\n", 1 );
		}
	
	// show how much memory is being used by what
	const char * laststr = nullptr;
	int maxlen = 0;
	for (;;)
		{
		// count the next tag string
		const char * thisstr = nullptr;
		for ( pool = gRootPool;  pool;  pool = * static_cast<const void * const *>( pool ) )
			{
			fl = reinterpret_cast<const NAFreeList *>(
					static_cast<const char *>( pool ) + kPoolHeaderSize );
			for (;;)
				{
				size_t size = fl->flSize;
				if ( 0 == size )
					break;
				
				// get the tag string
				tagString = fl->flTagString;
				if ( not tagString )
					tagString = "<untagged>";
				
				// find the longest tag string
				if ( not laststr )
					{
					len = static_cast<int>( std::strlen( tagString ) );
					if ( len > maxlen )
						maxlen = len;
					}
				
				// skip strings we have already counted
				if ( not laststr
				||   std::strcmp( tagString, laststr ) > 0 )
					{
					// do we have a new string to track?
					if ( not thisstr
					||   std::strcmp( tagString, thisstr ) < 0 )
						{
						nused   = 1;
						memused = fl->flSize;
						thisstr = tagString;
						}
					// did we find a matching tag string?
					else
					if ( tagString == thisstr
					||   std::strcmp( tagString, thisstr ) == 0 )
						{
						++nused;
						memused += fl->flSize;
						}
					}
				
				fl = reinterpret_cast<const NAFreeList *>(
						reinterpret_cast<const char *>( fl ) + kHeaderSize + size );
				}
			}
		
		// done
		if ( not thisstr )
			break;
		
		// print usage section header
		if ( not laststr )
			{
			len = snprintf( buff, sizeof buff,
#if HAVE_PRINTF_COMMA_FORMAT
						"%-*s\t%9s\t%12s\n",
#else
						"%-*s\t%8s\t%10s\n",
#endif
						maxlen,
						"Tag Type",
						"Count",
						"Usage" );
			DTS_write( fileref, buff, uint( len ) );
			char * p = buff;
			for ( len += 10; len > 0; --len )	// 10? to adjust for tabs etc.
				*p++ = '-';
			*p++ = '\n';
			DTS_write( fileref, buff, static_cast<size_t>( p - buff ) );
			}
		
		// show how many times this tag has been used
		len = snprintf( buff, sizeof buff,
#if HAVE_PRINTF_COMMA_FORMAT
					"%-*s\t%'9lu\t%'12lu\n",
#else
					"%-*s\t%8lu\t%10lu\n",
#endif
					maxlen,
					thisstr,
					nused,
					static_cast<ulong>( memused ) );
		DTS_write( fileref, buff, uint( len ) );
		
		// go to the next one
		laststr = thisstr;
		}
	
	DTS_close( fileref );
}
#endif	// DEBUG_VERSION_TAGS

#endif  // DEBUG_VERSION_NEW
