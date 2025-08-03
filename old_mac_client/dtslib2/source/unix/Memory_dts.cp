/*
**	Memory-dts.cp		dtslib2
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


/*
**	Assumptions:
**
**	alignment is on 8 byte boundaries.
*/


// for debugging
//#define DUMBNESS 1
#undef DUMBNESS


// brain damage
//#include "Local_dts.h"
//#include "Everything_mac.h"

#include "Memory_mac.h"


// for unix
extern void log( char * text );

// mac's growzone stuff
#ifdef DTS_Mac
static pascal long MyGrowZone( long size );
StaticUPP(MyGrowZone,GrowZone)
#endif


#ifdef DEBUG_VERSION
#ifndef DEBUG_VERSION_TAGS
#define DEBUG_VERSION_TAGS 1
#endif
#endif


/*
**	Entry Routines
*/
#ifdef DEBUG_VERSION_TAGS
void DumpBlocks( const char * filename );
void DumpBlocksSummary( const char * filename );
static void DumpBlocksCommon( const char * filename, long verbose );
#endif
// you got me
#ifdef DTS_Mac
#ifdef _MSL_USING_NAMESPACE      // hh 971207 Added namespace support
	namespace std {
extern new_handler	__new_handler;
	}	
// rer 9/20/00 - take everything below -out- of namespace std.
// conceivably it belongs in a hypothetical namespace DTS, or even an unnamed one
// but definitely not in std.
#endif
#endif

#ifdef DEBUG_VERSION_TAGS
char *		NewAlloc( long size, const char * tagString );
#else
char *		NewAlloc( long size );
#endif
void		NewFree( char * ptr );


/*
**	When we need to we allocate a pool with NewPtr or malloc
**
**		+-----------------+
**		| 0x0DE1FE1FL     |		debug version only
**		| prevsize = 0    |		there is no previous block
**		| size     = size |		size of caller data + alignment bytes
**		| next     = nil  |
**		| prev     = nil  |
**		| 0x0DE1FE1FL     |		debug version only
**		+-----------------+
**		| caller data     |		pointer returned to the caller
**		|                 |
**		|                 |
**		|                 |
**		|                 |
**		|                 |
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
#define		kMaxBlockSize		(long(0x80000000-2*kHeaderSize))
#define		kMinPoolSize		(long(256*1024))
#define		kMinBlockSize		(32)
#define		kHeaderSize			(long(sizeof(NAFreeList)))
#define		kBlockAlignment		(kMinBlockSize-1)
#define		kNumBuckets			(long(26))
#define		kZapUninitialized	0xF1F1F1F1L
#define		kZapReleased		0xF3F3F3F3L
#define		kBlockMarker		0x0DE1FE1FL

#ifdef DEBUG_VERSION_TAGS
#define		kPoolHeaderSize		sizeof(double)
#else
#define		kPoolHeaderSize		0
#endif

class NAFreeList
	{
public:
#ifdef DEBUG_VERSION
	long			flBlockMarker1;		// mark the start of a block
#endif
	long			flPrevSize;			// the size of the previous block
	long			flSize;				// the size of this block
	NAFreeList *	flNext;				// pointer to the next free block
	NAFreeList *	flPrev;				// pointer to the previous free block
										// or nil if the block is in use
#ifdef DEBUG_VERSION_TAGS
	const char *	flTagString;		// pointer to constant string
	long			flUsedSize;			// the unmodified size of the block
#endif
#ifdef DEBUG_VERSION
	long			flBlockMarker2;		// mark the start of a block
#endif
	};


/*
**	Internal Routines
*/
static	NAFreeList *		NANewPool( long size );
static	void				NAFreeBlock( NAFreeList * fl );
#ifdef DUMBNESS
static	void				NADumbnessCheck( void );
#endif
static	void				NAFillLongs( long * ptr, long count, long value );
static	long				NACheckOverwrite( const char * start, const char * stop );


/*
**	Variables
*/
static	NAFreeList			gBuckets[kNumBuckets];
#ifdef DUMBNESS
static	long				gCounter;
#endif
#ifdef DEBUG_VERSION_TAGS
static	char *				gRootPool;
#endif
#ifdef DTS_Mac
static	DTSBoolean			bPoolFreed;
#endif



/*
**	NewAlloc()
**
**	allocate memory from an available pool
*/
#ifdef DEBUG_VERSION_TAGS
char * NewAlloc( long size, const char * tagString )
#else
char * NewAlloc( long size )
#endif
{
#ifdef DUMBNESS
	++gCounter;
	NADumbnessCheck();
#endif
	
	// sanity check the size
	if ( size < 0
	||   size >= kMaxBlockSize )
		{
		return nil;
		}
	
	// remember the original size
#ifdef DEBUG_VERSION
	long orgsize = size;
#endif
	
	// pin the block size to a minimum
	if ( size < kMinBlockSize )
		{
		size = kMinBlockSize;
		}
	// add alignment bytes if needed
	size = ( size + kBlockAlignment ) & ~kBlockAlignment;
	
	// select a bucket based on the size
	NAFreeList * bucket = gBuckets;
	long bucketsize = kMinBlockSize;
	for(;;)
		{
		if ( bucketsize >= size )
			{
			break;
			}
		if ( bucketsize >= kMaxBlockSize/2 )
			{
			break;
			}
		++bucket;
		bucketsize += bucketsize;
		}
	
	// is there a block in this bucket?
	NAFreeList * fl = bucket->flNext;
	if ( fl == nil )
		{
		// no, try the bigger buckets
		NAFreeList * startbucket = bucket;
		do
			{
			// did we run out of buckets?
			if ( bucket >= &gBuckets[kNumBuckets-1] )
				{
				// allocate a new pool
				if ( size < kMinPoolSize )
					{
					fl = NANewPool( kMinPoolSize );
					}
				if ( fl == nil )
					{
					fl = NANewPool( size );
					if ( fl == nil )
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
								for ( ;  ;  fl = fl->flNext )
									{
									if ( fl == bucket )
										{
										fl = nil;
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
						return nil;
						}
					}
				break;
				}
			
			// check the next bigger bucket
			++bucket;
			fl = bucket->flNext;
			}
		while ( fl == nil );
		}
	
#ifdef DUMBNESS
	// dumbness check
	if ( fl->flSize < size )
		{
		gCounter = gCounter;
		}
#endif
	
	// at this point we have a block
	// remove it from its bucket
	// and mark it as in use
	NAFreeList * nextfl = fl->flNext;
	NAFreeList * prevfl = fl->flPrev;
	if ( nextfl != prevfl )
		{
		prevfl->flNext = nextfl;
		nextfl->flPrev = prevfl;
		}
	else
		{
		nextfl->flNext = nil;
		nextfl->flPrev = nil;
		}
#ifdef DUMBNESS
	fl->flNext = nil;
#endif
	fl->flPrev = nil;
	
	// check if the block is big enough to split
	long blocksize = fl->flSize;
	long splitsize = blocksize - size - kHeaderSize;
	if ( splitsize >= kMinBlockSize )
		{
		// get pointer to the new header in the middle
		// and pointer to the trailing header
		// ie the header for the next block
		NAFreeList * splitfl = (NAFreeList *)( long(fl)      + kHeaderSize + size      );
		NAFreeList * trailfl = (NAFreeList *)( long(splitfl) + kHeaderSize + splitsize );
		
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
#ifdef DEBUG_VERSION
		splitfl->flBlockMarker1 = kBlockMarker;
		splitfl->flUsedSize     = 0;
		splitfl->flBlockMarker2 = kBlockMarker;
#endif
		
		// free the newly split block
		NAFreeBlock( splitfl );
		}
	
	// point to the block
	char * ptr = (char *)( long(fl) + kHeaderSize );
	
	// zap the returned block with the value meaning uninitialized
#ifdef DEBUG_VERSION_TAGS
	fl->flTagString = tagString;
#endif
#ifdef DEBUG_VERSION
	fl->flUsedSize  = orgsize;
	NAFillLongs( (long *) ptr, fl->flSize, kZapUninitialized );
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
NewFree( char * ptr )
{
	if ( nil == ptr )
		return;
	
	// check every block in every pool for consistency
#ifdef DUMBNESS
	NADumbnessCheck();
#endif
	
	// point to the header
	NAFreeList * fl = (NAFreeList *)( long(ptr) - kHeaderSize );
	
#ifdef DEBUG_VERSION
	// check the block markers for consistency
	long size = fl->flSize;
	long orgsize = fl->flUsedSize;
	NAFreeList * trailfl = (NAFreeList *)( long(ptr) + size );
	if ( kBlockMarker !=      fl->flBlockMarker1
	||   kBlockMarker !=      fl->flBlockMarker2
	||   kBlockMarker != trailfl->flBlockMarker1
	||   kBlockMarker != trailfl->flBlockMarker2 )
		{
	#ifdef DTS_Mac
		VDebugStr( "Bad block marker." );
	#else
		log( "Bad block marker.\n" );
		abort();
	#endif
		}
	
	// check for overwrites
	else
	if ( orgsize != 0
	&&   NACheckOverwrite( ptr + orgsize, ptr + size ) )
		{
	#ifdef DTS_Mac
		VDebugStr( "End of block overwritten." );
	#else
		log( "End of block overwritten.\n" );
		abort();
	#endif
		}
	
	// check if this block has already been freed
	else
	if ( fl->flPrev )
		{
	#ifdef DTS_Mac
		VDebugStr( "Block double deleted." );
	#else
		log( "Block double deleted.\n" );
		abort();
	#endif
		}
	else
#endif /* DEBUG_VERSION */
		{
#ifdef DEBUG_VERSION
		// zap the block with the value meaning released
		NAFillLongs( (long *) ptr, size, kZapReleased );
#endif /* DEBUG_VERSION */
		
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
**	allocate a pool of the size
**	assume the size is good
*/
static NAFreeList *
NANewPool( long size )
{
#ifdef DTS_Mac
	// remove our grow zone proc
	// if the newptr fails the new handler will be called anyway
	SetGrowZone( nil );
	
	NAFreeList * fl = (NAFreeList *) NewPtr( size + 2*kHeaderSize + kPoolHeaderSize );
	
	// install our grow zone proc
	SetGrowZone( MakeUPP(MyGrowZone,GrowZone) );
#else
	NAFreeList * fl = (NAFreeList *) malloc( size + 2*kHeaderSize + kPoolHeaderSize );
#endif
	if ( fl )
		{
#ifdef DEBUG_VERSION_TAGS
		* (char **) fl = gRootPool;
		gRootPool = (char *) fl;
		fl = (NAFreeList *)( long(fl) + kPoolHeaderSize );
#endif
		
		// select a bucket based on the size
		NAFreeList * bucket = gBuckets;
		long bucketsize = 2*kMinBlockSize;
		for(;;)
			{
			if ( bucketsize > size )
				{
				break;
				}
			if ( bucketsize >= kMaxBlockSize/4 )
				{
				break;
				}
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
		NAFreeList * trailfl = (NAFreeList *)( long(fl) + kHeaderSize + size );
		trailfl->flPrevSize = size;
		trailfl->flSize     = 0;
		trailfl->flPrev     = nil;
		trailfl->flNext     = nil;
		
		// add the block markers to the headers
		// zap the the entire pool with the value meaning uninitialized
#ifdef DEBUG_VERSION_TAGS
		fl     ->flTagString    = "PoolHeader";
		trailfl->flTagString    = "PoolTrailer";
#endif
#ifdef DEBUG_VERSION
		fl     ->flBlockMarker1 = kBlockMarker;
		fl     ->flUsedSize     = 0;
		fl     ->flBlockMarker2 = kBlockMarker;
		trailfl->flBlockMarker1 = kBlockMarker;
		trailfl->flUsedSize     = 0;
		trailfl->flBlockMarker2 = kBlockMarker;
		NAFillLongs( (long *)( long(fl) + kHeaderSize ), size, kZapUninitialized );
#endif
		}
	
	return fl;
}


/*
**	NAFreeBlock()
**
**	free the block
*/
static void
NAFreeBlock( NAFreeList * fl )
{
	NAFreeList * prevfl;
	NAFreeList * nextfl;
	
	long size = fl->flSize;
	NAFreeList * trailfl = (NAFreeList *)( long(fl) + kHeaderSize + size );
	
	// is there a previous block?
	long prevsize = fl->flPrevSize;
	if ( prevsize )
		{
		// is it free?
		prevfl = (NAFreeList *)( long(fl) - kHeaderSize - prevsize );
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
				nextfl->flNext = nil;
				nextfl->flPrev = nil;
				}
			
			// zap the old previous header with the value meaning released
#ifdef DEBUG_VERSION
			NAFillLongs( (long *) fl, kHeaderSize, kZapReleased );
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
			nextfl->flNext = nil;
			nextfl->flPrev = nil;
			}
		
		// zap the the old trailing header with the value meaning released
		// be careful to not step on any needed fields
		long trailsize = trailfl->flSize;
#ifdef DEBUG_VERSION
		NAFillLongs( (long *) trailfl, kHeaderSize, kZapReleased );
#endif
		
		// merge them
		size               += trailsize + kHeaderSize;
		fl->flSize          = size;
		trailfl             = (NAFreeList *)( long(fl) + kHeaderSize + size );
		trailfl->flPrevSize = size;
		}
	
	// is the whole pool now free?
	if ( fl->flPrevSize  == 0
	&&   trailfl->flSize == 0 )
		{
#ifdef DTS_Mac
		bPoolFreed = true;
#endif
		
		char * pool = (char *)( long(fl) - kPoolHeaderSize );
		
		// remove it from the linked list
#ifdef DEBUG_VERSION_TAGS
		char * test;
		for ( char ** link = &gRootPool;  ;  link = (char **) test )
			{
			test = *link;
			if ( nil == test )
				{
#ifdef DTS_Mac
				VDebugStr( "Pool not found in linked list." );
				break;
#else
				log( "Pool not found in linked list.\n" );
				abort();
#endif
				}
			if ( test == pool )
				{
				*link = * (char **) pool;
				break;
				}
			}
#endif
		// dispose of the now unused pool
#ifdef DTS_Mac
		DisposePtr( pool );
#else
		free( pool );
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
		long bucketsize = 2*kMinBlockSize;
		for(;;)
			{
			if ( bucketsize > size )
				{
				break;
				}
			if ( bucketsize >= kMaxBlockSize/4 )
				{
				break;
				}
			++bucket;
			bucketsize += bucketsize;
			}
		
		// add the block to the bucket
		// phd question: in order to minimize fragmentation...
		// do we add the block to the front or the end of the list?
		nextfl = bucket->flNext;
		// the point is moot, the list is empty
		if ( nil == nextfl )
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
static void
NADumbnessCheck()
{
	for ( NAFreeList * bucket = gBuckets;  bucket < &gBuckets[kNumBuckets];  ++bucket )
		{
		NAFreeList * f0 = bucket;
		NAFreeList * f1 = f0->flNext;
		if ( f1 )
			{
			for(;;)
				{
				if ( f1->flPrev != f0 )
					{
					gCounter = gCounter;
					}
				if ( f1 == bucket )
					{
					break;
					}
				f0 = f1;
				f1 = f0->flNext;
				}
			}
		}
}
#endif


#ifdef DEBUG_VERSION
/*
**	NAFillLongs()
**
**	fill memory with a long value
*/
static void
NAFillLongs( long * ptr, long count, long value )
{
	// convert byte count to long count
	count >>= 2;
	
	// store longs to make the count a multiple of 8
	long remainder = count & 7;
	switch ( remainder )
		{
		case 7:		ptr[6] = value;
		case 6:		ptr[5] = value;
		case 5:		ptr[4] = value;
		case 4:		ptr[3] = value;
		case 3:		ptr[2] = value;
		case 2:		ptr[1] = value;
		case 1:		ptr[0] = value;
			ptr += remainder;
		}
	
	// store longs 8 at a time
	count >>= 3;
	for ( ;  count > 0;  --count )
		{
		ptr[0] = value;
		ptr[1] = value;
		ptr[2] = value;
		ptr[3] = value;
		ptr[4] = value;
		ptr[5] = value;
		ptr[6] = value;
		ptr[7] = value;
		ptr += 8;
		}
}
#endif


#ifdef DEBUG_VERSION
/*
**	NACheckOverwrite()
**
**	check if the block has been overwritten
**	return true if it has been
**	return false if not
*/
static long
NACheckOverwrite( const char * ptr, const char * stop )
{
	for (  ;  ptr < stop;  ++ptr )
		{
		if ( *ptr != (char ) kZapUninitialized )
			return true;
		}
	return false;
}
#endif


#ifdef DEBUG_VERSION_TAGS
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
static void
DumpBlocksCommon( const char * filename, long verbose )
{
	FILE * stream = fopen( filename, "w+" );
	if ( stream == nil )
		return;
	
	const char * tagString;
	const char * pool;
	const NAFreeList * fl;
	
	long npools  = 0;
	long nfree   = 0;
	long nused   = 0;
	long memfree = 0;
	long memused = 0;
	
	// show all pools - verbose only
	// collect summary info
	for ( pool = gRootPool;  pool;  pool = * (char **) pool )
		{
		++npools;
		if ( verbose )
			fprintf( stream, "pool at 0x%X\n", pool );
		
		fl = reinterpret_cast<const NAFreeList *>( long(pool) + kPoolHeaderSize );
		for(;;)
			{
			long size = fl->flSize;
			const NAFreeList * prev = fl->flPrev;
			if ( verbose )
				{
				fprintf( stream, "%8.8X %8.8X %8.8X %8.8X %8.8X",
					fl, fl->flPrevSize, size, fl->flNext, prev );
#ifdef DEBUG_VERSION
				if ( fl->flBlockMarker1 != kBlockMarker )
					fprintf( stream, " (bad block marker1)" );
				if ( fl->flBlockMarker2 != kBlockMarker )
					fprintf( stream, " (bad block marker2)" );
#endif
				tagString = fl->flTagString;
				if ( tagString )
					fprintf( stream, " %s", tagString );
				fprintf( stream, "\n" );
				}
			if ( size == 0 )
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
			
			fl = reinterpret_cast<const NAFreeList *>( long(fl) + kHeaderSize + size );
			}
		}
	
	// show summary info
	fprintf( stream, "%ld pools, %ld/%ld free, %ld/%ld in use.\n\n",
		npools, nfree, memfree, nused, memused );
	
	// show all of the free lists - verbose only
	if ( verbose )
		{
		const NAFreeList * bucket = gBuckets;
		long losize = kMinBlockSize;
		for ( long count = kNumBuckets;  count > 0;  --count )
			{
			long hisize = losize + losize;
			
			fprintf( stream, "bucket size %8.8X to %8.8X:\n", losize, hisize-1 );
			const NAFreeList * walk = bucket->flNext;
			if ( walk )
				{
				for ( ;  walk != bucket;  walk = walk->flNext )
					{
					char ch = ' ';
					long size = walk->flSize;
					if ( size <  losize
					||   size >= hisize )
						{
						ch = '*';
						}
					fprintf( stream, "  %8.8X %8.8X %c\n", walk, size, ch );
					}
				}
			
			++bucket;
			losize = hisize;
			}
		fprintf( stream, "\n" );
		}
	
	// show how much memory is being used by what
	const char * laststr = nil;
	long maxlen = 0;
	for(;;)
		{
		// count the next tag string
		const char * thisstr = nil;
		for ( pool = gRootPool;  pool;  pool = * (char **) pool )
			{
			fl = reinterpret_cast<const NAFreeList *>( long(pool) + kPoolHeaderSize );
			for(;;)
				{
				long size = fl->flSize;
				if ( size == 0 )
					break;
				
				// get the tag string
				tagString = fl->flTagString;
				if ( tagString == nil )
					tagString = "<untagged>";
				
				// find the longest tag string
				if ( laststr == nil )
					{
					long len = strlen( tagString );
					if ( len > maxlen )
						maxlen = len;
					}
				
				// skip strings we have already counted
				if ( laststr == nil
				||   strcmp( tagString, laststr ) > 0 )
					{
					// do we have a new string to track?
					if ( thisstr == nil
					||   strcmp( tagString, thisstr ) < 0 )
						{
						nused   = 1;
						memused = fl->flSize;
						thisstr = tagString;
						}
					// did we find a matching tag string?
					else
					if ( tagString == thisstr
					||   strcmp( tagString, thisstr ) == 0 )
						{
						++nused;
						memused += fl->flSize;
						}
					}
				
				fl = reinterpret_cast<const NAFreeList *>( long(fl) + kHeaderSize + size );
				}
			}
		
		// done
		if ( thisstr == nil )
			break;
		
		// show how many times this tag has been used
		fprintf( stream, "%-*s count %ld usage %ld\n",
			maxlen, thisstr, nused, memused );
		
		// go to the next one
		laststr = thisstr;
		}
	
	fclose( stream );
}
#endif


#ifdef DTS_Mac
/*
**	MyGrowZone()
**
**	custom grow zone procedure
*/
static pascal long
MyGrowZone( long sizeneeded )
{
#pragma unused( sizeneeded )
	
	// call the new handler until a pool is freed
	// or the new handler gives up
	bPoolFreed = false;
	while ( std::__new_handler )
		{
		(*std::__new_handler)();
		if ( bPoolFreed )
			{
			return 1;	// a non-zero number indicating success
			}
		}
	
	// oh deer
	// we're likely to be in trouble now
	return 0;	// failure
}
#endif

