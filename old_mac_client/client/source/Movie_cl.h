/*
**	Movie_cl.h		ClanLord
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

#ifndef MOVIE_CL_H
#define MOVIE_CL_H

#if PRAGMA_STRUCT_ALIGN
# pragma options align=mac68k
#elif defined( __GNUC__ )
# pragma pack(2)
#else
# error "How to align?"
  // # pragma ... ?
#endif

#define kCLMovieFolderName	":Movies"

// version of movie: hi word is version, lo word is sub-version
const int kMovieMajorVersionShift	= 16;
const int kMovieMajorVersionMask	= 0xFFFF0000;

class CCLMovie
{
public:
	//
	// "Outside" API
	//
static	bool				HasMovie()
	{ return sCurrentMovie != nullptr; }
static	bool				IsReading()
	{ return sCurrentMovie ? not sCurrentMovie->mWriteMode : false; }
static 	bool				IsRecording()
	{ return sCurrentMovie ? sCurrentMovie->mWriteMode : false; }
	
static	DTSError			StartReadMovie( DTSFileSpec * inMovie );
static	DTSError			StartRecordMovie();
static	void				StopMovie();

static	DTSError			WriteOneFrame( const void * inFrame, size_t frameLen, uint inFlags )
	{ return sCurrentMovie ? sCurrentMovie->WriteFrame( inFrame, frameLen, inFlags ) : 0; }
static	DTSError			ReadOneFrame( void * outFrame, size_t &oFrameLen, uint &oFlags )
	{ return sCurrentMovie ? sCurrentMovie->ReadFrame( outFrame, oFrameLen, oFlags ) : 0; }

	// Is it time for a new frame to be read?
static	bool				HasFrame();

	// playback position

static	void				GetPlaybackPosition( int& oCurFrame, int& oTotalFrames );

	// Set get read speed/pause
static	void				SetReadSpeed( int inSpeed );
static	int					GetReadSpeed();

#if BITWISE_IMAGE_SPOOL
	// parse incoming image data
static	void				InterpretFramePicture( DataSpool * spool,
								DTSKeyID& pictID, int& horz, int& vert );
#endif	// BITWISE_IMAGE_SPOOL
	
	// return version of movie: hi word is version, lo word is sub-version
static	int					GetVersion()
	{	return sCurrentMovie ? sCurrentMovie->Version() : 0; }
	
	// Extra packet flags
	enum
		{
		flag_Stale			= 0x01,
		flag_MobileData		= 0x02,		// frame contains descriptor data
		flag_GameState		= 0x04,		// frame contains saved game state
		flag_PictureTable	= 0x08		// frame contains cached images
		};
	enum
		{
		speed_Pause			= 0,
		speed_Play,
		speed_Fast,
		speed_Turbo
		};
	enum
		{
		kMovieTooOldErr		= -129,
		kMovieTooNewErr		= -130
		};
	
protected:
	//
	// No, you don't have access to the constructor from outside
	//
							CCLMovie( DTSFileSpec * inMovie );
	virtual					~CCLMovie();
	
	DTSError				Open( bool inWriteMode );
	void					Close();
	
	DTSError				Flush();
	DTSError				WriteHeader();
	DTSError				WriteFrame( const void * inFrame, size_t inLen, uint inFlags );
	DTSError				Write( const void * inData, size_t inDataLen );
	
	DTSError				ReadFrame( void * outFrame, size_t &oFrameLen, uint &oFlags );
	DTSError				Read( void * outData, size_t inDataLen );
	
	DTSError				ReadMobileTable();
	DTSError				SaveMobileTable();
	DTSError				SavePictureTable();
	DTSError				ReadPictureTable();
	DTSError				ReadGameState();
	DTSError				SaveGameState();
	DTSError				SaveGameStateData();
	
	DTSError				Read1Descriptor( DescTable * desc );
	
	//
	// Frame delay (pause, play, fast forward) assessors
	//
	ulong					GetReadFrameDelay() const
		{	return mReadFrameDelay; }
	void					SetReadFrameDelay( ulong inFrameDelay )
		{	mReadFrameDelay = inFrameDelay; }
	void					PauseRead()
		{	SetReadFrameDelay( 0 ); }
	bool					IsReadPaused() const
		{	return 0 == mReadFrameDelay; }
	bool					IsFastRead() const
		{	return ticksPerFrameFast == mReadFrameDelay; }
	
	int						Version() const
		{ return (mFileHead.version << kMovieMajorVersionShift)
				 | ushort( mFileHead.revision ); }
	
	//
	// Create a new file into the movie folder, with a standard name
	// Does NOT open the file
	//
static	DTSError			CreateNewMovieFile( DTSFileSpec * outSpec );

static	CCLMovie *			sCurrentMovie;
static	ulong				sFrameTicks;
	
	enum
		{
		buffer_Size			= 4 * 1024,
		packet_Sign			= 0xdeadbeef,		// mark beginning of packet
		
		// this used to be simply 15 (i.e. 4 FPS), but, from 2/16/2005, the CL server's
		// baseline framerate increased to 14 ticks/frame (~4.3 fps). There are also
		// "peak" and "off-off-peak" framerates (of 15 and 13 ticks, or 4 and ~4.6 fps,
		// respectively). One day we might somehow be able to record the true current
		// framerate IN our movies, so they'll always play at the "right" speed;
		// but for now, let's just bump the default speed to match the server's
		// current standard.
		// In mid-2021 the default server rate became 12 ticks/frames (5 fps).
		ticksPerFrame		= 12,	// or maybe (60 / kFramesPerSecond) ?
		ticksPerFrameFast	= ticksPerFrame / 4,
		ticksPerFrameTurbo	= 1
		};
	
	// Written at file start
	struct SFileHead
		{
		uint32_t			signature;			// packet_Sign
		int16_t				version;			// which version of CL did record this
		int16_t				len;				// header len;
		int32_t				frames;				// number of frames
		uint32_t			starttime;			// when was that recorded
		int32_t				revision;			// for intra-version changes
		int32_t				oldestReader;		// clients older than this cannot read
		};
	
	struct SFileHead_Old
		{
		// movies created prior to v105.1 used this format instead
		// how do we tell the difference between 105.0 and 105.1? I guess we
		// look at 'len'.
		uint32_t			signature;			// packet_Sign
		int16_t				version;			// which version of CL did record this
		int16_t				len;				// header len;
		int32_t				frames;				// number of frames
		uint32_t			starttime;			// when was that recorded
		};
	
	// Written before frame data
	struct SFrameHead
		{
		uint32_t			signature;			// packet_Sign
		int32_t				frame;				// frame index
		int16_t				size;				// frame size to follow
		uint16_t			flags;				// various flags.
		};
	
	FSRef					mFileSpec;
	int						mFileRefNum;		// FileMgr refNum of opened file
	SFileHead				mFileHead;			// current header data
	bool					mWriteMode;			// false -> reading, true -> writing
	uchar					mBuffer[ buffer_Size ];
	size_t					mBufferPos;			// read/write offset in buffer
	size_t					mBufferLen;			// used buffer size
	int						mReadFrameDelay;	// control playback speed
	int						mPlaybackFrame;		// where we are at

#if DTS_LITTLE_ENDIAN
	static void				SwapEndian(	SFrameHead& );
	static void				SwapEndian(	SFileHead& );
	static void				SwapEndian( DSMobile& );
	static void				SwapEndian( DescTable& );
#endif  // DTS_LITTLE_ENDIAN
};


//
//	Older movie descriptor table layouts
//

struct DescTable_191
{
//	before v193, this structure had 68k alignment
//	(mostly, anyway - some carbon clients waffled, causing problems)
//	Note: from v193 on, descriptors use 'power' alignment
	int32_t				descID;
	int32_t				descCacheID;
	DSMobile *			descMobile;
	int32_t				descSize;
	int32_t				descType;
	int32_t				descBubbleType;
	int32_t				descBubbleLanguage;
	int32_t				descBubbleCounter;
	int32_t				descBubblePos;
	int32_t				descBubbleLastPos;
	DTSRect				descBubbleBox;
	int32_t				descNumColors;
	DTSPoint			descBubbleLoc;
	uchar				descColors[ kNumPlyColors ];
	char				descName[ kMaxNameLen ];
	DTSRect				descLastDstBox;
	PlayerNode *		descPlayerRef;
#ifdef AUTO_HIDENAME
	int32_t				descSeenFrame;
	int32_t				descNameVisible;
#endif
	char				descBubbleText[ kMaxDescTextLength ];
};

struct DescTable_141
{
/* 0		*/	int32_t				descID;
/* 4		*/	int32_t				descCacheID;
/* 8		*/	DSMobile *			descMobile;
/* 12		*/	int32_t				descSize;
/* 16		*/	int32_t				descType;	// v74a: kDescPlayer, monster, npc, other
/* 20		*/	int32_t				descBubbleType;
// added descBubbleLanguage v142
//				int32_t				descBubbleLanguage;
/* 24		*/	int32_t				descBubbleCounter;
/* 28		*/	int32_t				descBubblePos;
/* 32		*/	int32_t				descBubbleLastPos;
/* 36		*/	DTSRect				descBubbleBox;
/* 44		*/	int32_t				descNumColors;
/* 48		*/	DTSPoint			descBubbleLoc;
/* 52		*/	uchar				descColors[ kNumPlyColors ];
/* 52+c		*/	char				descName[ kMaxNameLen ];
/* 52+c+n	*/	DTSRect				descLastDstBox;
/* 60+c+n	*/	PlayerNode *		descPlayerRef;
#ifdef AUTO_HIDENAME
/* 64+c+n	*/	int32_t				descSeenFrame;
/* 68+c+n	*/	int32_t				descNameVisible;		// can be 0, 25, 50 75, 100
#endif
/* 72+c+n	*/	char				descBubbleText[ kMaxDescTextLength ];
// total: 72+c+n + strlen(bubble)
};

struct DescTable_113
{
/* 0		*/	int32_t				descID;
/* 4		*/	int32_t				descCacheID;
/* 8		*/	DSMobile *			descMobile;
/* 12		*/	int32_t				descSize;
/* 16		*/	int32_t				descType;	// v74a: kDescPlayer, monster, npc, other
/* 20		*/	int32_t				descBubbleType;
/* 24		*/	int32_t				descBubbleCounter;
/* 28		*/	int32_t				descBubblePos;
/* 32		*/	int32_t				descBubbleLastPos;
/* 36		*/	DTSRect				descBubbleBox;
/* 44		*/	int32_t				descNumColors;
/* 48		*/	DTSPoint			descBubbleLoc;
/* 52		*/	uchar				descColors[ kNumPlyColors ];
/* 52+c		*/	char				descName[ kMaxNameLen ];
/* 52+c+n	*/	DTSRect				descLastDstBox;
/* 60+c+n	*/	void *				descPlayerRef;
#ifdef AUTO_HIDENAME
//				int32_t				descSeenFrame;
//				int32_t				descNameVisible;		// can be 0, 25, 50 75, 100
#endif
/* 64+c+n	*/	char				descBubbleText[ kMaxDescTextLength ];
// total: 64+c+n + strlen(bubble)
};

struct DescTable_105
{
	int32_t				descID;						// 0
	int32_t				descCacheID;				// 4
	DSMobile *			descMobile;					// 8
	int32_t				descSize;					// 12
	int32_t				descType;					// 16
	int32_t				descBubbleType;				// 20
	int32_t				descBubbleCounter;			// 24
	int32_t				descBubblePos;				// 28
//	int32_t				descBubbleLastPos;	// this field was added v105.1a
//	DTSRect				descBubbleBox;		// this field was added v105.1
	int32_t				descNumColors;				// 32
	DTSPoint			descBubbleLoc;				// 36
	uchar				descColors[ kNumPlyColors ];	// 40
	char				descName[ kMaxNameLen ];	// 40 + kNumPlyColors
	DTSRect				descLastDstBox;				// 40 + kNumPlyColors + kMaxNameLen
	void *				descPlayerRef;				// 48 + ditto
	char				descBubbleText[ kMaxDescTextLength ];	// 52 + ditto
	// total: 52 + strlen(descBubbleText) + kNumPlyColors + kMaxNameLen
};

struct DescTable_97
{
	int32_t				descID;
	int32_t				descCacheID;
//	DSMobile *			descMobile;	 <- this field added in v97.x; breaking old movies
	int32_t				descSize;
	int32_t				descType;
	int32_t				descBubbleType;
	int32_t				descBubbleCounter;
	int32_t				descBubblePos;
	int32_t				descNumColors;
	DTSPoint			descBubbleLoc;
	uchar				descColors[ kNumPlyColors ];
	char				descName[ kMaxNameLen ];
	DTSRect				descLastDstBox;
	void *				descPlayerRef;
	char				descBubbleText[ kMaxDescTextLength ];
	// total: 48 + strlen(descBubbleText) + kNumPlyColors + kMaxNameLen
};

#if BITWISE_IMAGE_SPOOL
//
// Older packed image data layouts
//

// v366 and earlier
const int	kPictFrameCoordNumBits_366	= 10;
const int	kPictFrameIDNumBits_366		= 12;
#endif //	BITWISE_IMAGE_SPOOL

#if PRAGMA_STRUCT_ALIGN
# pragma options align=reset
#elif defined( __GNUC__ )
# pragma pack()
#else
// # pragma ... ?
#endif

#endif  // MOVIE_CL_H

