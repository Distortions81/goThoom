/*
**	Movie_cl.cp		ClanLord
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
#include "VersionNumber_cl.h"
#include "Frame_cl.h"
#include "Movie_cl.h"

/*
**	static class data
*/
CCLMovie *	CCLMovie::sCurrentMovie		= nullptr;
ulong		CCLMovie::sFrameTicks		= 0;


/*
**	CCLMovie::CCLMovie()
**
**	movie constructor
*/
CCLMovie::CCLMovie( DTSFileSpec * inMovie ) :
	mFileRefNum( -1 ),
	mWriteMode( false ),
	mBufferPos( 0 ),
	mBufferLen( 0 ),
	mReadFrameDelay( ticksPerFrame ),
	mPlaybackFrame( 0 )
{
	__Verify_noErr( DTSFileSpec_CopyToFSRef( inMovie, &mFileSpec ) );
}


/*
**	CCLMovie::~CCLMovie()
**
**	movie destructor -- ensure file is closed
*/
CCLMovie::~CCLMovie()
{
	Close();
}


/*
**	CCLMovie::Open()
**
**	open the movie's file, for reading or writing
*/
DTSError
CCLMovie::Open( bool inWriteMode )
{
	// don't open it twice
	if ( mFileRefNum != -1 )
		return opWrErr;
	
	mWriteMode = inWriteMode;
	mBufferPos = 0;
	mBufferLen = 0;
	
	SInt16 refNum;
	DTSError err = FSOpenFork( &mFileSpec, 0, nullptr, inWriteMode ? fsWrPerm : fsRdPerm, &refNum );
	__Check_noErr( err );
	mFileRefNum = refNum;
	if ( noErr != err )
		return err;
	
	if ( not mWriteMode )
		{
		// we are reading
		// read the first part of the header
		err	= Read( &mFileHead, sizeof(SFileHead_Old) );
		if ( noErr == err )
			{
			// sanity check on the header so far
			if ( BigToNativeEndian( mFileHead.signature ) != packet_Sign )
				err = paramErr;
			else
			if ( sizeof(SFileHead_Old) == BigToNativeEndian( mFileHead.len ) )
				{
				// fixup an old header
				mFileHead.revision = 0;	// make one up; doesn't matter
				mFileHead.oldestReader = NativeToBigEndian( 80 << 8 );  // will re-swap below
				}
			else
			if ( sizeof(SFileHead) == BigToNativeEndian( mFileHead.len ) )
				{
				// it's a current one
				// read the extended header part
				err = Read( &mFileHead.revision,
						sizeof mFileHead.revision + sizeof mFileHead.oldestReader );
				}
			else
				{
				// some wacko length we never heard of
				err = paramErr;
				}
			}
		
		// we could maybe read game state data here..
		gNumMobiles	= 0;	// needed anyway
		
		// don't read old movies, because
		// descriptor records changed format in v97.x and again in v105.x
		// at some point we have to forego backward compatibility
		enum
			{
#if DTS_BIG_ENDIAN
			kOldestMovieVersion = 80	// I made this up. I dunno.
#else
			kOldestMovieVersion = 193	// I don't want to deal with byteswapping
										// those older movie formats
#endif  // DTS_BIG_ENDIAN
			};
		
#if DTS_LITTLE_ENDIAN
		if ( noErr == err )
			SwapEndian( mFileHead );
		
#endif  // DTS_LITTLE_ENDIAN
		
#ifdef ARINDAL
		// Arindal version numbers are factor 100
		// higher than CL numbers. This causes a
		// type short overrun. Quick hack to fix:
		mFileHead.version = uint16_t( mFileHead.version / 100 );
#endif // ARINDAL
		
		if ( noErr == err && mFileHead.version < kOldestMovieVersion )
			err = kMovieTooOldErr;
		
		// also make sure this client is not too old for this movie
		if ( noErr == err && mFileHead.oldestReader > kFullVersionNumber )
			err = kMovieTooNewErr;
		}
	else
		{
		// we are writing
		// write a first empty header. frames will follow.
		// at the end, we'll come back and write the filled-in header
		mFileHead.signature		= packet_Sign;
		mFileHead.version		= kVersionNumber;
		mFileHead.len			= sizeof mFileHead;
		mFileHead.frames		= 0;
		
		UInt32 now;
#if TARGET_API_MAC_OS8
		::GetDateTime( &now );
#else
		__Verify_noErr( UCConvertCFAbsoluteTimeToSeconds( CFAbsoluteTimeGetCurrent(), &now ) );
#endif
		mFileHead.starttime		= now;
		
		// additional fields as of 105.2
		mFileHead.revision		= kSubVersionNumber;
		
		// v353+ movies might have longer frames
		// which could cause earlier readers to misbehave
		mFileHead.oldestReader	= (353 << 8) + 0;
		err = WriteHeader();
		
		// append game state data at end of file
		if ( noErr == err )
			{
			err = FSSetForkPosition( mFileRefNum, fsFromLEOF, 0 );
			__Check_noErr( err );
			}
		if ( noErr == err )
			err = SaveGameStateData();
		}
	
	return err;
}


#pragma mark ** Low level IO

/*
**	CCLMovie::Close()
**	close the movie's file
*/
void
CCLMovie::Close()
{
	// don't close it twice!
	if ( mFileRefNum != -1 )
		{
		if ( mWriteMode )
			{
			Flush();
			WriteHeader();
			}
		__Verify_noErr( FSCloseFork( mFileRefNum ) );
		
		// mark it closed now
		mFileRefNum = -1;
		}
}


/*
**	CCLMovie::Flush()
**	flush buffer to disk
*/
DTSError
CCLMovie::Flush()
{
	if ( -1 == mFileRefNum || not mWriteMode )
		return rfNumErr;
	
	if ( 0 == mBufferLen )
		return noErr;
	
//	ByteCount count;
	DTSError err = FSWriteFork( mFileRefNum, fsFromMark, 0, mBufferLen, mBuffer, nullptr );
	mBufferLen = 0;
	
	return err;
}


/*
**	CCLMovie::WriteHeader()		[HFS+ version]
**
**	update the first-frame header data in the file
*/
DTSError
CCLMovie::WriteHeader()
{
	if ( -1 == mFileRefNum || not mWriteMode )
		return rfNumErr;
	
	// save position in file
	SInt64 pos;
	OSStatus err = FSGetForkPosition( mFileRefNum, &pos );
	__Check_noErr( err );
	
	// (over-)write the header at the start of the file
	if ( noErr == err )
		{
#if DTS_LITTLE_ENDIAN
		SFileHead tmpHead = mFileHead;
		SwapEndian( tmpHead );
		err = FSWriteFork( mFileRefNum, fsFromStart, 0, sizeof mFileHead, &tmpHead, nullptr );
#else
		err = FSWriteFork( mFileRefNum, fsFromStart, 0, sizeof mFileHead, &mFileHead, nullptr );
#endif
		}
	
	// restore position in file
	if ( noErr == err )
		{
		err = FSSetForkPosition( mFileRefNum, fsFromStart, pos );
		__Check_noErr( err );
		}
	
	return err;
}


/*
**	CCLMovie::Write()
**
**	Write some data into the file.
**	pending writes are chunked up in the buffer to minimize IO calls.
*/
DTSError
CCLMovie::Write( const void * inData, size_t inDataLen )
{
	DTSError err = noErr;
	size_t len = inDataLen;
	
	// don't overflow the buffer
	if ( buffer_Size < inDataLen + mBufferLen )
		len = buffer_Size - mBufferLen;
	
	// copy data to buffer
//	::BlockMoveData( inData, &mBuffer[mBufferLen], len );
	memcpy( &mBuffer[ mBufferLen ], inData, len );
	mBufferLen += len;
	
	// if buffer is full, spill it to disk.
	if ( buffer_Size == mBufferLen )
		err = Flush();
	
	// recursively write anything that didn't fit last time
	if ( noErr == err && inDataLen > len )
		Write( (Ptr) inData + len, inDataLen - len );
	
	return err;
}


/*
**	CCLMovie::Read()
**	read a chunk of data into the buffer
*/
DTSError
CCLMovie::Read( void * outData, size_t inDataLen )
{
	DTSError err = noErr;
	
	// fill buffer from file, if empty
	if ( mBufferLen == mBufferPos )
		{
		mBufferPos = 0;
		ByteCount len;
		err = FSReadFork( mFileRefNum, fsFromMark, 0, buffer_Size, mBuffer, &len );
		
		mBufferLen = len;
		if ( eofErr == err && mBufferLen )
			err = noErr;	// ignore eof until all data is read
		}
	
	// disgorge from buffer to consumer
	if ( noErr == err )
		{
		size_t len = inDataLen;
		if ( mBufferLen < inDataLen + mBufferPos )
			len = mBufferLen - mBufferPos;
		
//		::BlockMoveData( &mBuffer[ mBufferPos ], outData, len );
		memcpy( outData, &mBuffer[ mBufferPos ], len );
		mBufferPos += len;
		
		// call self recursively to process the excess over buffer size
		if ( len < inDataLen )
			err = Read( (Ptr) outData + len, inDataLen - len );
		}
	return err;
}


#pragma mark ** Endian

#if DTS_LITTLE_ENDIAN

/*
**	CCLMovie::SwapEndian()
*/
#define SE(x)	(x) = ::SwapEndian( x )


/*
**	CCLMovie::SwapEndian( SFileHead )
**	byteswap a movie file header
*/
void
CCLMovie::SwapEndian( SFileHead& head )
{
	SE( head.signature );
	SE( head.version );
	SE( head.len );
	SE( head.frames );
	SE( head.starttime );
	SE( head.revision );
	SE( head.oldestReader );
}


/*
**	CCLMovie::SwapEndian( SFrameHead )
**	byteswap a movie frame header
*/
void
CCLMovie::SwapEndian( SFrameHead& head )
{
	SE( head.signature );
	SE( head.frame );
	SE( head.size );
	SE( head.flags );
}


//	This maybe belong in DTSLib's Endian.h, or in View_dts.h

/*
**	SwapEndian( DTSPoint )
**	byteswap a Point
*/
static inline void
SwapEndian( DTSPoint& p )
{
	SE( p.ptH );
	SE( p.ptV );
}


/*
**	SwapEndian( DTSRect )
**	byteswap a rectangle
*/
static inline void
SwapEndian( DTSRect& r )
{
	SE( r.rectTop );
	SE( r.rectLeft );
	SE( r.rectBottom );
	SE( r.rectRight );
}


// and these maybe belong in Frame_cl.h?
/*
**	CCLMovie::SwapEndian( DSMobile )
**	byteswap a Mobile
*/
void
CCLMovie::SwapEndian( DSMobile& m )
{
	SE( m.dsmIndex );
	SE( m.dsmState );
	SE( m.dsmHorz );
	SE( m.dsmVert );
	SE( m.dsmColors );
}


/*
**	CCLMovie::SwapEndian( DescTable )
**	byteswap a descriptor
*/
void
CCLMovie::SwapEndian( DescTable& d )
{
	SE( d.descID );
	SE( d.descCacheID );
//	SE( d.descMobile );		// DSMobile *
	SE( d.descSize );
	SE( d.descType );
	SE( d.descBubbleType );
	SE( d.descBubbleLanguage );
	SE( d.descBubbleCounter );
	SE( d.descBubblePos );
	SE( d.descBubbleLastPos );
	::SwapEndian( d.descBubbleBox );
	SE( d.descNumColors );
	::SwapEndian( d.descBubbleLoc );
//	d.descColors is a byte array
//	d.descName ditto
	::SwapEndian( d.descLastDstBox );
//	SE( d.descPlayerRef );		// PlayerNode *
# if AUTO_HIDENAME
	SE( d.descSeenFrame );
	SE( d.descNameVisible );
# endif
//	d.descBubbleText is a byte array too, and besides we read/write it in a second, separate pass
}


/*
**	SwapEndian( SFramePict )
**	byteswap a frame-picture structure
*/
static void
SwapEndian( CCLFrame::SFramePict& p )
{
	SE( p.mPictID );
	SE( p.mH );
	SE( p.mV );
}

#endif  // DTS_LITTLE_ENDIAN


#pragma mark ** High level IO

/*
**	CCLMovie::WriteFrame()
**
**	write a frame, prepending the header
*/
DTSError
CCLMovie::WriteFrame( const void * inFrame, size_t inFrameLen, uint inFlags )
{
	SFrameHead head =
		{
		packet_Sign,
		(int32_t) mFileHead.frames++,
		(int16_t) inFrameLen,
		(uint16_t) inFlags
		};
#if DTS_LITTLE_ENDIAN
	SwapEndian( head );
#endif
	DTSError err = Write( &head, sizeof head );
	if ( noErr == err )
		err = Write( inFrame, inFrameLen );	// this is the stream data we got from server;
			// it's already (and always) bigendian.
	return err;
}


/*
**	CCLMovie::ReadFrame()
**
**	read a frame & process any flagged extra data
*/
DTSError
CCLMovie::ReadFrame( void * outFrame, size_t& oFrameLen, uint& oFlags )
{
	SFrameHead head;
	DTSError err = Read( &head, sizeof head );
	if ( err != noErr )
		return err;
	
	// safety first
	if ( BigToNativeEndian( head.signature ) != packet_Sign )
		return paramErr;
	
#if DTS_LITTLE_ENDIAN
	SwapEndian( head );
#endif
	
	//
	// Got a state data frame, let's handle it
	//
	if ( head.flags & flag_MobileData )
		{
		err = ReadMobileTable();
		if ( noErr == err )
			return ReadFrame( outFrame, oFrameLen, oFlags );
		}
	
	if ( head.flags & flag_GameState )
		{
		err = ReadGameState();
		if ( noErr == err )
			return ReadFrame( outFrame, oFrameLen, oFlags );
		}
	
	if ( head.flags & flag_PictureTable )
		{
		err = ReadPictureTable();
		if ( noErr == err )
			return ReadFrame( outFrame, oFrameLen, oFlags );
		}
	
	oFrameLen = head.size;
	oFlags = head.flags;
	mPlaybackFrame = head.frame;
	
	err = Read( outFrame, oFrameLen );
	return err;
}


#pragma mark ** Game State Saving/Restoring

/*
**	CCLMovie::SaveGameState()
**
**	save game state data
*/
DTSError
CCLMovie::SaveGameState()
{
	int32_t data[ 6 ];
	data[0]	= NativeToBigEndian( gLeftPictID );			// DTSKeyID
	data[1]	= NativeToBigEndian( gRightPictID );		// DTSKeyID
	data[2]	= NativeToBigEndian( gStateMode );			// int
	data[3]	= NativeToBigEndian( gStateMaxSize );		// int
	data[4]	= NativeToBigEndian( gStateCurSize );		// int
	data[5]	= NativeToBigEndian( gStateExpectSize );	// int
	
	// but don't write trash
	if ( not gStatePtr
	||	 ( gStateCurSize > gStateMaxSize ) )
		{
		data[3]	= data[4] = 0;	// endian OK
		}
	
	// write the game-state pseudo-frame
	SFrameHead head = { packet_Sign, mFileHead.frames++, 0, flag_GameState };
#if DTS_LITTLE_ENDIAN
	SwapEndian( head );
#endif
	DTSError err = Write( &head, sizeof head );
	
	// and the data...
	if ( noErr == err )
		err = Write( data, sizeof data );
	
	// and the state stuff, if any
	if ( noErr == err && gStateMaxSize )
		err = Write( gStatePtr, gStateMaxSize );
	
	return err;
}


/*
**	CCLMovie::ReadGameState()
**	inverse of above
*/
DTSError
CCLMovie::ReadGameState()
{
	int32_t data[ 6 ];
	DTSError err = Read( data, sizeof data );
	if ( noErr == err )
		{
		gLeftPictID			= BigToNativeEndian( data[0] );
		gRightPictID		= BigToNativeEndian( data[1] );
		gStateMode			= BigToNativeEndian( data[2] );
		gStateMaxSize		= BigToNativeEndian( data[3] );
		gStateCurSize		= BigToNativeEndian( data[4] );
		gStateExpectSize	= BigToNativeEndian( data[5] );
		
		if ( gStateMaxSize )
			{
			gStatePtr = NEW_TAG("CCLMovie::ReadGameState") uchar[ gStateMaxSize ];
			if ( not gStatePtr )
				err = memFullErr;
			if ( noErr == err )
				err = Read( gStatePtr, gStateMaxSize );
			}
		}
	return err;
}


/*
**	CCLMovie::SaveMobileTable()
**	tricky footwork to preserve the mobile table
*/
DTSError
CCLMovie::SaveMobileTable()
{
	if ( 0 == gNumMobiles )			// Well, nothing to do obviously. table is empty
		return 0;
	
	int mobileCounter = 0;
	DTSError err = noErr;
	const size_t maxWriteSize = sizeof(DSMobile) + sizeof(DescTable);
//	const size_t minDescSize = (gDescTable[0].descBubbleText - (char*)&gDescTable[0]);
	const size_t minDescSize = offsetof(DescTable, descBubbleText);
	
	//
	// We first save the descriptor pointed to by a Mobile. To be sure,
	// we also save the mobile
	//
	do
		{
		// the frame length is always zero, since we don't know the length anyway
		// note the special flag
		SFrameHead	head = { packet_Sign, mFileHead.frames++, 0, flag_MobileData };
#if DTS_LITTLE_ENDIAN
		SwapEndian( head );
#endif
		err = Write( &head, sizeof head );
		
		size_t maxSize = 0x7fff - sizeof(int32_t);	// what we are allowed to write
		
		// fill up each frame
		for ( ; mobileCounter < gNumMobiles && maxSize >= maxWriteSize && noErr == err;
				++mobileCounter )
			{
			const DSMobile * mob = &gDSMobile[ mobileCounter ];
			const DescTable * desc = &gDescTable[ mob->dsmIndex ];
			
			int16_t bubbleLen = desc->descBubbleCounter ? std::strlen( desc->descBubbleText ) : 0;
			
			// Write mobile
#if DTS_BIG_ENDIAN
			/* err = */ Write( mob, sizeof(DSMobile) );
#else
			{
			DSMobile tm = *mob;
			SwapEndian( tm );
			/* err = */ Write( &tm, sizeof tm );
			}
#endif
			
			// We write the first part of the descriptor, without the bubble text
#if DTS_BIG_ENDIAN
			err	= Write( desc, minDescSize );
#else
			{
			DescTable td = *desc;
			SwapEndian( td );
			err = Write( &td, minDescSize );
			}
#endif
			
			// If the bubble text is relevant, we write it, with its length
			if ( noErr == err && bubbleLen )
				{
#if DTS_BIG_ENDIAN
				err = Write( &bubbleLen, sizeof bubbleLen );
#else
				int16_t bubLenBE = NativeToBigEndian( bubbleLen );
				err = Write( &bubLenBE, sizeof bubLenBE );
#endif  // DTS_BIG_ENDIAN
				
				if ( noErr == err )
					err = Write( desc->descBubbleText, bubbleLen );
				}
			
			// keep tabs of how much we wrote
			maxSize -= sizeof(DSMobile) + minDescSize + bubbleLen;
			if ( bubbleLen )
				maxSize -= sizeof bubbleLen;
			}
		
		if ( noErr == err )
			{
			const int32_t trailer = -1;	// to know we reached the end of the frame (endian OK)
			err	= Write( &trailer, sizeof trailer );
			}
		} while ( mobileCounter < gNumMobiles && noErr == err );
	
	//
	// Then we save all the descriptors that 'look' used somehow
	// we don't store any mobile
	//
	if ( noErr == err )
		{
		int descCounter	= 0;
		do
			{
			// the frame length is always zero, since we don't know the length anyway
			// note the special flag
			SFrameHead head = { packet_Sign, mFileHead.frames++, 0, flag_MobileData };
#if DTS_LITTLE_ENDIAN
			SwapEndian( head );
#endif
			err = Write( &head, sizeof head );
			
			size_t maxSize = 0x7fff - sizeof(int32_t);	// what we are allowed to write
			
			// fill up the frame as much as possible
			for ( ; descCounter < kDescTableSize && maxSize >= maxWriteSize && noErr == err;
					++descCounter )
				{
				bool save = true;
				
				// skip the ones we already saved above
				for ( int m = 0; m < gNumMobiles && save; ++m )
					{
					if ( gDSMobile[m].dsmIndex == descCounter )
						save = false;
					}
				
				const DescTable * desc = &gDescTable[ descCounter ];
				
				// skip nameless / id-less ones also
				if ( save )
					save = desc->descName[0] || desc->descID;
				
				// OK, we found one worth saving
				if ( save )
					{
					int16_t bubbleLen = desc->descBubbleCounter
						? std::strlen(desc->descBubbleText) : 0;
					int32_t counter = NativeToBigEndian( descCounter + kDescTableSize );
					
					// Write mobile
					/* err = */ Write( &counter, sizeof counter );
					
					// We write the first part of the descriptor, without the bubble text
#if DTS_BIG_ENDIAN
					err	= Write( desc, minDescSize );
#else
					{
					DescTable td = *desc;
					SwapEndian( td );
					err = Write( &td, minDescSize );
					}
#endif  // DTS_BIG_ENDIAN
					
					// If the bubble text is relevant, we write it, with its length
					if ( noErr == err && bubbleLen )
						{
#if DTS_BIG_ENDIAN
						err = Write( &bubbleLen, sizeof bubbleLen );
#else
						int16_t bubLenBE = NativeToBigEndian( bubbleLen );
						err = Write( &bubLenBE, sizeof bubLenBE );
#endif  // DTS_BIG_ENDIAN
						
						if ( noErr == err )
							err = Write( desc->descBubbleText, bubbleLen );
						}
					// I think this is wrong:
//					maxSize -= sizeof(DSMobile) + minDescSize + bubbleLen;
					// instead:
					maxSize -= sizeof counter + minDescSize + bubbleLen;
					if ( bubbleLen )
						maxSize -= sizeof bubbleLen;
					}
				}
			
			// terminate the frame
			if ( noErr == err )
				{
				const int32_t trailer = -1;		// we reached the end of the frame (endian OK)
				err	= Write( &trailer, sizeof trailer );
				}
			} while ( descCounter < kDescTableSize && noErr == err );
		}
	
	return err;
}


/*
**	CCLMovie::ReadMobileTable()
**
**	inverse of above. Even trickier, because it has to handle backward compatibility
*/
DTSError
CCLMovie::ReadMobileTable()
{
	// upon entering, we KNOW the header was already parsed by ReadFrame, so we don't care
	DTSError err = noErr;
	
	// now, read descriptors
	int32_t descIndex = 0;
	do
		{
		// read descriptor index
		err = Read( &descIndex, sizeof descIndex );
		
		// stop at end or error
		if ( noErr != err || -1 == descIndex )	// endian-OK
			continue;
		
		descIndex = BigToNativeEndian( descIndex );
		if ( descIndex < 0 )
			{
			ShowMessage( BULLET " Invalid descIndex: %d", int( descIndex ) );
			err = ioErr;
			}
		else
		if ( descIndex < kDescTableSize )
			{
			// it's a regular descriptor
			DSMobile * mob = &gDSMobile[ gNumMobiles++ ];
			mob->dsmIndex = descIndex;
			err = Read( &mob->dsmState, sizeof(DSMobile) - sizeof descIndex );
			
#if DTS_LITTLE_ENDIAN
			if ( noErr == err )
				{
				SwapEndian( *mob );
				mob->dsmIndex = descIndex;	// re-overwrite this
				}
#endif  // DTS_LITTLE_ENDIAN
			}
		else
			{
			// it's a thought pseudo-descriptor
			descIndex -= kDescTableSize;
			
			// ... or maybe it's utter garbage -- be safe!
			if ( descIndex >= kDescTableSize )
				{
				ShowMessage( BULLET " DescIndex too big: %d", (int) descIndex );
				err = ioErr;
				}
			}
		
		// we read the remainder of the descriptor
		DescTable * desc = &gDescTable[ descIndex ];
		if ( noErr == err )
			{
			// handle old descriptors
			err = Read1Descriptor( desc );
			}
		
		if ( noErr == err )
			{
			// first, let's clean it properly
			desc->descCacheID	= 0;
			desc->descPlayerRef	= nullptr;
			desc->descMobile	= nullptr;
			
			// now read the bubble text, if any
			if ( desc->descBubbleCounter )
				{
				int16_t bubbleLen = 0;
				err = Read( &bubbleLen, sizeof bubbleLen );
				if ( noErr == err )
					{
					bubbleLen = BigToNativeEndian( bubbleLen );
					err	= Read( desc->descBubbleText, bubbleLen );
					}
				}
			}
		} while ( noErr == err && descIndex != -1 );
	
	return err;
}


/*
**	CCLMovie::Read1Descriptor()
**
**	Descriptor table entries have sustained lots of minor changes over the years; and
**	they've also been written out into movie files, in all different flavors.
**	This is where we try to convert any of those obsolete structures into the
**	current format, whatever it might be.
*/
DTSError
CCLMovie::Read1Descriptor( DescTable * desc )
{
	DTSError err = noErr;
	size_t minDescSize = offsetof(DescTable, descBubbleText);
	
	// this is the breakpoint between current and prior versions.
	// at the time of writing it was equal to: 192
	enum { kOldestCurrentVersion = MakeLong( 192, 0 ) };
	
	// convert an old-style descriptor
	
	int fullVersion = (mFileHead.version << kMovieMajorVersionShift) + mFileHead.revision;
	if ( fullVersion >= kOldestCurrentVersion )
		{
		// the descriptors do not need translation
		err = Read( desc, minDescSize );
		if ( noErr != err )
			return err;
		
		// from v795 thru v804, the Mac client's 'descUnused' field was in the wrong place
		// [independent of byte-swapping]
		enum {
			kFirstBadVersionAfter795  = MakeLong( 795, 0 ),
			kFirstGoodVersionAfter804 = MakeLong( 804, 1 )
			};
		if ( fullVersion >= kFirstBadVersionAfter795 && fullVersion < kFirstGoodVersionAfter804 )
			{
			// which means that the descName and descLastDstBox fields
			// might need to be moved up by two bytes
			// ... or it might not need any adjustment, if recorded by a non-Mac client.
			// unfortunately we don't know which client recorded it!
			// we can only hope that the wrongly-placed unused bytes are 0
			if ( 0 == desc->descName[ 0 ] && 0 == desc->descName[ 1 ] )
				{
				memmove( &desc->descName[ 0 ], &desc->descName[ 2 ],
					sizeof desc->descName + sizeof desc->descLastDstBox );
			
				// ... and, just for good hygiene, clear the now-exposed unused fields
				desc->descUnused[0] = desc->descUnused[1] = '\0';
				}
			}
#if DTS_LITTLE_ENDIAN
		SwapEndian( *desc );
#endif

#if 0 && defined( DEBUG_VERSION )
		// some of my local movies, made while endianizing this very code,
		// are outright corrupt.
		if ( desc->descNumColors < 0 || desc->descNumColors > kNumCustColors )
			{
			ShowMessage( "Impossible # colors: %d", int( desc->descNumColors ) );
			return ioErr;
			}
#endif
		}
	
	// ** NOTE **
	// Each time the current descriptor format changes incompatibly, you
	// need to do two things: add a case [here] to handle the previous version,
	//		_AND_
	// adjust all of the other cases below as well
	
	else
#if DTS_LITTLE_ENDIAN
	// i am not about to cope with byte-swapping all this mess.
	// in theory this case has already been ruled out by the kOldestMovieVersion check
	// way up in CCLMovie::Open(), but let's be extra safe.
		err = kMovieTooOldErr;
#else
	if ( mFileHead.version > 141 )
		{
		// read the v142-191 descriptor
		DescTable_191 tempDesc;
		minDescSize = offsetof( DescTable_191, descBubbleText );
		err = Read( &tempDesc, minDescSize );
		if ( noErr == err )
			{
			// field-wise copy is the only sure way, alas
			desc->descID				= tempDesc.descID;
//			desc->descCacheID			= tempDesc.descCacheID;	// not needed; will be zeroed
//			desc->descMobile			= nullptr;					// ditto
			desc->descSize				= tempDesc.descSize;
			desc->descType				= tempDesc.descType;
			desc->descBubbleType		= tempDesc.descBubbleType;
			desc->descBubbleLanguage	= tempDesc.descBubbleLanguage;
			desc->descBubbleCounter		= tempDesc.descBubbleCounter;
			desc->descBubblePos			= tempDesc.descBubblePos;
			desc->descBubbleLastPos		= tempDesc.descBubbleLastPos;
			desc->descBubbleBox			= tempDesc.descBubbleBox;
			desc->descNumColors			= tempDesc.descNumColors;
			desc->descBubbleLoc			= tempDesc.descBubbleLoc;
			std::memcpy( desc->descColors, tempDesc.descColors,	sizeof tempDesc.descColors	);
			std::memcpy( desc->descName,   tempDesc.descName,	sizeof tempDesc.descName	);
			desc->descLastDstBox		= tempDesc.descLastDstBox;
//			desc->descPlayerRef			= tempDesc.descPlayerRef;
# ifdef AUTO_HIDENAME
			desc->descSeenFrame			= tempDesc.descSeenFrame;
			desc->descNameVisible		= tempDesc.descNameVisible;
# endif
			// bubble text is handled elsewhere
			}
		}
	else
	if ( mFileHead.version > 113 )
		{
		// read the v113-141 descriptor
		DescTable_141 tempDesc;
		minDescSize = offsetof( DescTable_141, descBubbleText );
		
		err = Read( &tempDesc, minDescSize );
		if ( noErr == err )
			{
			// field-wise copy is the only sure way, alas
			desc->descID				= tempDesc.descID;
//			desc->descCacheID			= tempDesc.descCacheID;	// not needed; will be zeroed
//			desc->descMobile			= nullptr;				// ditto
			desc->descSize				= tempDesc.descSize;
			desc->descType				= tempDesc.descType;
			desc->descBubbleType		= tempDesc.descBubbleType;
			desc->descBubbleLanguage	= 0;								// new field
			desc->descBubbleCounter		= tempDesc.descBubbleCounter;
			desc->descBubblePos			= tempDesc.descBubblePos;
			desc->descBubbleLastPos		= tempDesc.descBubbleLastPos;
			desc->descBubbleBox			= tempDesc.descBubbleBox;
			desc->descNumColors			= tempDesc.descNumColors;
			desc->descBubbleLoc			= tempDesc.descBubbleLoc;
			std::memcpy( desc->descColors, tempDesc.descColors,	sizeof tempDesc.descColors	);
			std::memcpy( desc->descName, tempDesc.descName,		sizeof tempDesc.descName	);
			desc->descLastDstBox		= tempDesc.descLastDstBox;
//			desc->descPlayerRef			= tempDesc.descPlayerRef;
# ifdef AUTO_HIDENAME
			desc->descSeenFrame			= tempDesc.descSeenFrame;
			desc->descNameVisible		= tempDesc.descNameVisible;
# endif
			// bubble text is handled elsewhere
			}
		}
	else
	if ( mFileHead.version > 105 )
		{
		// read the v105..113 descriptor
		DescTable_113 tempDesc;
		minDescSize = offsetof( DescTable_113, descBubbleText );
		
		err = Read( &tempDesc, minDescSize );
		if ( noErr == err )
			{
			desc->descID				= tempDesc.descID;
//			desc->descCacheID			= tempDesc.descCacheID;	// not needed; will be zeroed
//			desc->descMobile			= nullptr;				// ditto
			desc->descSize				= tempDesc.descSize;
			desc->descType				= tempDesc.descType;
			desc->descBubbleType		= tempDesc.descBubbleType;
			desc->descBubbleLanguage	= 0;								// new
			desc->descBubbleCounter		= tempDesc.descBubbleCounter;
			desc->descBubblePos			= tempDesc.descBubblePos;
			desc->descBubbleLastPos		= tempDesc.descBubbleLastPos;
			desc->descBubbleBox			= tempDesc.descBubbleBox;
			desc->descNumColors			= tempDesc.descNumColors;
			desc->descBubbleLoc			= tempDesc.descBubbleLoc;
			std::memcpy( desc->descColors, tempDesc.descColors,	sizeof tempDesc.descColors	);
			std::memcpy( desc->descName, tempDesc.descName,		sizeof tempDesc.descName	);
			desc->descLastDstBox		= tempDesc.descLastDstBox;
//			desc->descPlayerRef			= tempDesc.descPlayerRef;
# ifdef AUTO_HIDENAME
			desc->descSeenFrame			= 0;								// new
			desc->descNameVisible		= kName_Visible;					// new
# endif
			// bubble text is handled elsewhere
			}
		}
	else
	if ( mFileHead.version > 97 )
		{
		// read the v97..105 descriptor
		DescTable_105 tempDesc;
		minDescSize = offsetof( DescTable_105, descBubbleText );
		
		err = Read( &tempDesc, minDescSize );
		if ( noErr == err )
			{
			desc->descID				= tempDesc.descID;
//			desc->descCacheID			= tempDesc.descCacheID;	// not needed; will be zeroed
//			desc->descMobile			= nullptr;				// ditto
			desc->descSize				= tempDesc.descSize;
			desc->descType				= tempDesc.descType;
			desc->descBubbleType		= tempDesc.descBubbleType;
			desc->descBubbleLanguage	= 0;								// new
			desc->descBubbleCounter		= tempDesc.descBubbleCounter;
			desc->descBubblePos			= tempDesc.descBubblePos;
			desc->descBubbleLastPos		= tempDesc.descBubblePos;			// new
			desc->descBubbleBox.Set(0, 0, 0, 0);							// new
			desc->descNumColors			= tempDesc.descNumColors;
			desc->descBubbleLoc			= tempDesc.descBubbleLoc;
			std::memcpy( desc->descColors, tempDesc.descColors,	sizeof tempDesc.descColors	);
			std::memcpy( desc->descName, tempDesc.descName,		sizeof tempDesc.descName	);
			desc->descLastDstBox		= tempDesc.descLastDstBox;
//			desc->descPlayerRef			= tempDesc.descPlayerRef;
# ifdef AUTO_HIDENAME
			desc->descSeenFrame			= 0;
			desc->descNameVisible		= kName_Visible;
# endif
			// bubble text is handled elsewhere
			}
		}
	else
		{
		// read the v80..97 descriptor
		// (if the movie is older than v80, we've already rejected it.)
		DescTable_97 tempDesc;
		minDescSize = offsetof( DescTable_97, descBubbleText );
		
		err = Read( &tempDesc, minDescSize );
		if ( noErr == err )
			{
			desc->descID				= tempDesc.descID;
//			desc->descCacheID			= tempDesc.descCacheID;	// not needed; will be zeroed
//			desc->descMobile			= nullptr;				// ditto
			desc->descSize				= tempDesc.descSize;
			desc->descType				= tempDesc.descType;
			desc->descBubbleType		= tempDesc.descBubbleType;
			desc->descBubbleLanguage	= 0;								// new
			desc->descBubbleCounter		= tempDesc.descBubbleCounter;
			desc->descBubblePos			= tempDesc.descBubblePos;
			desc->descBubbleLastPos		= tempDesc.descBubblePos;			// new
			desc->descBubbleBox.Set(0, 0, 0, 0);							// new
			desc->descNumColors			= tempDesc.descNumColors;
			desc->descBubbleLoc			= tempDesc.descBubbleLoc;
			std::memcpy( desc->descColors, tempDesc.descColors,	sizeof tempDesc.descColors	);
			std::memcpy( desc->descName, tempDesc.descName,		sizeof tempDesc.descName	);
			desc->descLastDstBox		= tempDesc.descLastDstBox;
//			desc->descPlayerRef			= tempDesc.descPlayerRef;
# ifdef AUTO_HIDENAME
			desc->descSeenFrame			= 0;
			desc->descNameVisible		= kName_Visible;
# endif
			// bubble text is handled elsewhere
			}
		}
#endif  // DTS_LITTLE_ENDIAN
	
	return err;
}


/*
**	CCLMovie::ReadPictureTable()
**	read saved picture table, for the "again" picture optimization
*/
DTSError
CCLMovie::ReadPictureTable()
{
	DTSError err = noErr;
	int16_t pictAgain = 0;
	err = Read( &pictAgain, sizeof pictAgain );
	if ( noErr == err )
		{
		pictAgain = BigToNativeEndian( pictAgain );
		
		// better make sure we have a frame to put these into
		if ( not gFrame )
			InitFrames();
		
		// read that many remembered images
		// they will all fit into one frame because
		// each one is 6 bytes long and there are at most 131
		// which is 786 bytes total. Drop in the bucket.
		
		gFrame->mNumPictAgain = pictAgain;
		CCLFrame::SFramePict * pict = &gFrame->mPict[0];
		for ( int i = 0;  i < pictAgain && noErr == err;  ++i, ++pict )
			{
			err = Read( pict, sizeof(CCLFrame::SFramePict) );
#if DTS_LITTLE_ENDIAN
			if ( noErr == err )
				::SwapEndian( *pict );
#endif
			}
		}
	if ( noErr == err )
		{
		int32_t trailer = 0;
		err = Read( &trailer, sizeof trailer );
		if ( noErr == err && trailer != -1 )	// endian-OK
			err = paramErr;
		}
	return err;
}


/*
**	CCLMovie::SavePictureTable()
**
**	write the current picture table
*/
DTSError
CCLMovie::SavePictureTable()
{
	SFrameHead head = { packet_Sign, mFileHead.frames++, 0, flag_PictureTable };
#if DTS_LITTLE_ENDIAN
	SwapEndian( head );
#endif
	
	DTSError err = Write( &head, sizeof head );
	if ( noErr != err )
		return err;
	
	// make sure we have a frame to read from
	int pictAgain = gFrame ? gFrame->mNumPictAgain : 0;
	int16_t pictAgainBE = NativeToBigEndian( int16_t(pictAgain) );
	err = Write( &pictAgainBE, sizeof pictAgainBE );
	if ( noErr == err )
		{
		const CCLFrame::SFramePict * pict = &gFrame->mPict[0];
		for ( int i = 0;  i < pictAgain && noErr == err;  ++i, ++pict )
			{
#if DTS_BIG_ENDIAN
			err = Write( pict, sizeof(CCLFrame::SFramePict) );
#else
			CCLFrame::SFramePict tp = *pict;
			::SwapEndian( tp );
			err = Write( &tp, sizeof tp );
#endif
			}
		}
	
	if ( noErr == err )
		{
		const int32_t trailer = -1;		// endian OK
		err = Write( &trailer, sizeof trailer );
		}
	return err;
}


/*
**	CCLMovie::SaveGameStateData()
**	save all non-frame data
*/
DTSError
CCLMovie::SaveGameStateData()
{
	DTSError err = SaveGameState();
	if ( noErr == err )
		err = SaveMobileTable();
	if ( noErr == err )
		err = SavePictureTable();
	return err;
}


#pragma mark ** External API

/*
**	CCLMovie::CreateNewMovieFile()
**	create the file for a newly opened movie.
**	do any OS-related housekeeping.
*/
DTSError
CCLMovie::CreateNewMovieFile( DTSFileSpec * spec )
{
	// save the current directory
	DTSFileSpec savedir;
	savedir.GetCurDir();
	
	// First, create the folder DTSFileSpec
	DTSFileSpec	folderspec;
	folderspec.SetFileName( kCLMovieFolderName );
	
	// change to it
	DTSError err = folderspec.SetDir();
	
	// If it does not exists, create the folder
	if ( noErr != err )
		{
		err = folderspec.CreateDir();
		if ( noErr == err )
			err = folderspec.SetDir();
		}
	
	// now find the character movie folder
	if ( noErr == err )
		{
		folderspec.SetFileName( gPrefsData.pdCharName );
		
		// change to it
		err = folderspec.SetDir();
		
		// if it didn't exist, make it
		if ( noErr != err )
			{
			err = folderspec.CreateDir();
			if ( noErr == err )
				err = folderspec.SetDir();
			}
		}
	
	//
	// Ok, now let's create the file, with a proper name
	//
	if ( noErr == err )
		{
		DTSDate now;
		now.Get();
		
		{
		char buff[ 256 ];
		// "2001.04.01_12.34.56.clMov"	-- 25 characters
		snprintf( buff, sizeof buff,
				"%04d.%02d.%02d_%02d.%02d.%02d.clMov",
				now.dateYear, now.dateMonth, now.dateDay,
				now.dateHour, now.dateMinute, now.dateSecond);
		spec->SetFileName( buff );
		}
		
		spec->Delete();		// we remove the file if it exists
		
		// then we make the file
		FSRef parentDir, theFile;
		HFSUniStr255 fname;
		err = DTSFileSpec_CopyToFSRef( spec, &parentDir, &fname );
		if ( noErr == err )
			{
			err = FSCreateFileUnicode( &parentDir, fname.length, fname.unicode,
					kFSCatInfoNone, nullptr, &theFile, nullptr );
			__Check_noErr( err );
			}
		if ( noErr == err )
			(void) LSSetExtensionHiddenForRef( &theFile, true );
		}
	
	// need type&creator else GetFile dialogs won't work (til they're updated to use UTIs)
	if ( noErr == err )
		spec->SetTypeCreator( rClientMovieFREF, rClientSignFREF );
	
	// restore the current directory
	savedir.SetDirNoPath();
	
	return err;
}


/*
**	CCLMovie::StartReadMovie()
**	start playing a movie
*/
DTSError
CCLMovie::StartReadMovie( DTSFileSpec * inMovieSpec )
{
	StopMovie();
	
	sCurrentMovie = NEW_TAG("CCLMovie:StartReadMovie") CCLMovie( inMovieSpec );
	if ( not sCurrentMovie )
		return memFullErr;
	
	DTSError err = sCurrentMovie->Open( false );
	
	if ( noErr != err )
		StopMovie();
	
	return err;
}


#if BITWISE_IMAGE_SPOOL
/*
**	CCLMovie::InterpretFramePicture()
**
**	The frame picture data format changed in v366, and might conceivably do
**	so again. This is where we unpack images written in movies, keeping track of
**	which versions used which formats.
*/
void
CCLMovie::InterpretFramePicture( DataSpool * spool, DTSKeyID& pictID, int& horz, int& vert )
{
	int majorVersion = GetVersion();
	majorVersion = (majorVersion & kMovieMajorVersionMask) >> kMovieMajorVersionShift;
	
	if ( majorVersion > 366 )
		UnspoolFramePicture( spool, pictID, horz, vert,
			kPictFrameIDNumBits, kPictFrameCoordNumBits );
	else
		UnspoolFramePicture( spool, pictID, horz, vert,
			kPictFrameIDNumBits_366, kPictFrameCoordNumBits_366 );
}
#endif	// BITWISE_IMAGE_SPOOL


/*
**	CCLMovie::StartRecordMovie()
**	start recording a new movie
*/
DTSError
CCLMovie::StartRecordMovie()
{
	StopMovie();
	
	DTSFileSpec	spec;
	DTSError err = CreateNewMovieFile( &spec );
	
	if ( noErr == err )
		{
		sCurrentMovie = NEW_TAG("CCLMovie::StartRecordMovie") CCLMovie( &spec );
		if ( not sCurrentMovie )
			err = memFullErr;
		}
	if ( noErr == err )
		err = sCurrentMovie->Open( true );
	
	if ( noErr != err )
		StopMovie();
	
	return err;
}


/*
**	CCLMovie::StopMovie()
**	stop the current movie & dispose of it
*/
void
CCLMovie::StopMovie()
{
	if ( sCurrentMovie )
		{
		delete sCurrentMovie;
		sCurrentMovie = nullptr;
		}
}


/*
**	CCLMovie::HasFrame()
**
**	return true if the next movie frame is ready to play
**	this is how we handle playback speed control
*/
bool
CCLMovie::HasFrame()
{
	if ( IsReading() )
		{
		int delay = sCurrentMovie->GetReadFrameDelay();
		if ( 0 == delay )
			return false;	// paused
		
		ulong now = GetFrameCounter();
		if ( (delay > 0 && int(now - sFrameTicks) >=  delay)
		||	 (delay < 0 && int(now - sFrameTicks) >= -delay) )
			{
			sFrameTicks = now;
			if ( delay < 0 )
				sCurrentMovie->SetReadFrameDelay( 0 );
			return true;
			}
		}
	return false;
}


/*
**	CCLMovie::GetPlaybackPosition()		[static]
**	where are we?
*/
void
CCLMovie::GetPlaybackPosition( int& oCurFrame, int& oTotalFrames )
{
	if ( sCurrentMovie )
		{
		oCurFrame = sCurrentMovie->mPlaybackFrame;
		oTotalFrames = sCurrentMovie->mFileHead.frames;
		}
	else
		oCurFrame = oTotalFrames = 0;
}


/*
**	CCLMovie::SetReadSpeed()
**	change playback speed
*/
void
CCLMovie::SetReadSpeed( int inSpeed )
{
	if ( not IsReading() )
		return;
	int delay = sCurrentMovie->GetReadFrameDelay();
	if ( inSpeed < 0 )
		inSpeed = 0;
	if ( inSpeed > speed_Turbo )
		inSpeed = speed_Turbo;
	
	switch ( inSpeed )
		{
		case speed_Pause:	delay = -delay;					break;
		case speed_Play:	delay = ticksPerFrame;			break;
		case speed_Fast:	delay = ticksPerFrameFast;		break;
		case speed_Turbo:	delay = ticksPerFrameTurbo;		break;
		}
	
	sCurrentMovie->SetReadFrameDelay( delay );
}


/*
**	CCLMovie::GetReadSpeed()
**	get current playback rate
*/
int
CCLMovie::GetReadSpeed()
{
	if ( not IsReading() )
		return speed_Play;
	
	int delay = sCurrentMovie->GetReadFrameDelay();
	
	int result;
	if ( delay <= 0 )
		result = speed_Pause;
	else
	if ( ticksPerFrame == delay )
		result = speed_Play;
	else
	if ( ticksPerFrameFast == delay )
		result = speed_Fast;
	else
		result = speed_Turbo;
	
	return result;
}


