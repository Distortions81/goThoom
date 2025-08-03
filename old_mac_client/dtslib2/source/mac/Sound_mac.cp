/*
**	Sound_mac.cp		dtslib2
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

// AudioQueues are only available from OS 10.5 on
#if MAC_OS_X_VERSION_MIN_REQUIRED < MAC_OS_X_VERSION_10_5
# error "Sound support requires OS 10.5 or later"
#endif  // < 10.5

#include <AudioToolbox/AudioToolbox.h>

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Sound_mac.h"


/*
**	Entry Routines
**
**	DTSInitSound();
**	DTSExitSound();
**	DTSAreAllSoundsDone();
*/

/*
**	Definitions
*/
class SoundChannel;		// forward

/*
**	class DTSSoundPriv
**	this stores a (private copy of a) sound's sample data & length,
**	as well as the AudioStreamBasicDescription that characterizes it.
*/
class DTSSoundPriv
{
public:
	// constructor/destructor
					DTSSoundPriv();
					~DTSSoundPriv();
	
	// timing (for editor)
	void			DidFinishPlayback( CFAbsoluteTime startTime );
	CFTimeInterval	GetDuration() const { return sndDuration; }
	
	// no copying
private:
					DTSSoundPriv( const DTSSoundPriv& );
	DTSSoundPriv&	operator=( const DTSSoundPriv& );
	
	void			Reset();
	OSStatus		Init( const uchar * soundBytes, size_t len );
	
	// 'snd ' decoder helper
	OSStatus		InitFromSndListBytes( const uchar * sndData, size_t len );
	// we could conceivably have 'WAV', AIFF, etc. decoder helpers too...
	
	// stash the sample data, shorn of any headers
	OSStatus		SaveSampleData( const uchar * inSampleData, size_t inLen );
	
	AudioStreamBasicDescription	sndDesc;
	const uchar *				sndBytes;
	size_t						sndLen;
	SoundChannel *				sndChannel;
	CFTimeInterval				sndDuration;
	
	friend class DTSSound;
	friend class SoundChannel;
};


// a SoundChannel represents an AudioQueue and its associated buffer;
// it exists only so long as the sound is being played back
class SoundChannel : public DTSLinkedList<SoundChannel>
{
	// instance data
	AudioQueueRef				mQueue;
	AudioQueueBufferRef			mBuffer;
	DTSSoundPriv *				mSource;
	CFAbsoluteTime				mStartedTime;
	volatile bool				mStillPlaying;
	
public:
	// constructor/destructor
							SoundChannel( DTSSoundPriv * );
	virtual					~SoundChannel();
	
	// no copying
private:
							SoundChannel( const SoundChannel& );
	SoundChannel&			operator=( const SoundChannel& );
	
	// interface
public:
	OSStatus				Play( const DTSSoundPriv * sound );
	OSStatus				Stop();
	bool					IsDone() const;
	OSStatus				SetVolume( Float32 volume );
	void					SetSource( DTSSoundPriv * s ) { mSource = s; }
	
	static SoundChannel *	FindChannelForSound( const DTSSoundPriv * test );
	static int				NumInUse()	{ return gRootChannel->Count(); }
	
	// interface routines for non-class functions in Sound_dts.h
	static OSStatus			Init();
	static void				Exit();
	static void				SetAllVolumes( Float32 volume );

private:
	// class data
	static SoundChannel *		gRootChannel;
	static EventLoopTimerRef	sCleanupTimer;
	
	// AudioToolbox callbacks
#if MAC_OS_X_VERSION_MIN_REQUIRED <= MAC_OS_X_VERSION_10_5
	// can't count on having __BLOCKS__, therefore we may need this callback
	static void		BufferProc( void * ud, AudioQueueRef, AudioQueueBufferRef );
#endif  // <= 10.5
	static void		PropertyListenerProc( void * ud, AudioQueueRef, AudioQueuePropertyID );
	static void		CleanupProc( EventLoopTimerRef, void * );
	
	// queue creation
	OSStatus		CreateAudioQueue( const AudioStreamBasicDescription * );
};


/*
**	Internal Variables
*/
SoundChannel *		SoundChannel::gRootChannel;		// sounds currently or recently being played
EventLoopTimerRef	SoundChannel::sCleanupTimer;	// harvester of finished sound channels
static Float32		sDefaultVolume;					// 0.0 - 1.0
static int			sMaxSounds;						// max # simultaneously-active sounds

#pragma mark -


/*
**	DTSInitSound()
*/
void
DTSInitSound( int numChannels )
{
	sMaxSounds = numChannels;
	sDefaultVolume = 1;			// max volume
	
	// set up the timer that cleans up finished SoundChannel objects
	SoundChannel::Init();
}


/*
**	DTSExitSound()
*/
void
DTSExitSound()
{
	// remove the cleanup timer, and mop up any remaining sounds
	SoundChannel::Exit();
}


/*
**	DTSSetSoundVolume()
**
**	Set the baseline sound channel for all channels.
**	The theoretical range is 0 = silent to 0xFFFF = hardware maximum.
**	(although AudioToolbox works with floats, unlike the old Sound Manager.)
*/
void
DTSSetSoundVolume( ushort volume )
{
	// set default volume for sounds queued up in the future
	sDefaultVolume = volume / 65535.0F;
	
	// ... also adjust any that are presently being played
	SoundChannel::SetAllVolumes( sDefaultVolume );
}


#pragma mark -
#pragma mark Sound Utilities

//
// Utilities for dealing with old-fashioned "System 7" sounds
//

/*
**	IsFormatLPCM()
**
**	given a SoundMgr/QuickTime sound-format code, such as 'raw ', 'twos', 'MAC3', etc.,
**	return whether or not it represents a (non-compressed) LPCM encoding.
*/
bool
IsFormatLPCM( OSType format )
{
	switch ( format )
		{
		// known LPCM types. Only the first two have ever been seen in CL_Sounds,
		// but we can maybe afford to be more comprehensive?
		case k8BitOffsetBinaryFormat:
		case k16BitBigEndianFormat:
		
#if 0
		// we have none of these in CL_Sounds.
		// OTOH, they _are_ LPCM, within the meaning of the Act...
		// OT3H, it's not clear that ParseAIFFIntoSndHandle() would be able to handle
		// any of these formats properly if anyone ever did try to feed 'em to it.
		// So maybe it's best to pretend they don't exist.
		case k16BitLittleEndianFormat:
		case kFloat32Format:
		case kFloat64Format:
		case k24BitFormat:
		case k32BitFormat:
		case k32BitLittleEndianFormat:
#endif  // 0
			return true;
		
//		case kSoundNotCompressed:	// should this be in there too?
		}
	
	// anything else, assume it's compressed somehow
	return false;
}


//
// here are three routines that [attempt to] imitate the like-named Sound Manager APIs,
// only without triggering any deprecation warnings.  These aren't fully general emulations;
// they only work for the limited kinds of sound data actually found in CL_Sounds today.
// For example, we don't support "HyperCard"-style 'snd ' resource data; we reject any 'snd '
// that doesn't contain exactly one bufferCmd (and nothing else); and we don't understand
// most types of compression or non-LPCM samples.
//

/*
**	MyGetSoundHeaderOffset()
**
**	replacement for deprecated GetSoundHeaderOffset() API.
**	NB: takes a pointer to 'snd ' data, not a Handle.
**	Input data expected to be big-endian, but output (in 'headerOffset') is of course native.
*/
OSStatus
MyGetSoundHeaderOffset( const void * ptr, long& headerOffset )
{
	const SndListResource * slp = static_cast<const SndListResource *>( ptr );
	
	// only accept format-1 'snd 's
	if ( slp->format != EndianS16_NtoB( firstSoundFormat ) )
		return badFormat;
	
	UInt32 offset = 0;
	
	// skip past the modifiers
	short nMods = EndianS16_BtoN( slp->numModifiers );
//	__Check( nMods == 1 );	// should only be one
	const uchar * p = reinterpret_cast<const uchar *>( slp->modifierPart );
	p += nMods * sizeof(ModRef);
	
	// prepare to iterate the commandPart
	short nCmds = EndianS16_BtoN( * reinterpret_cast<const short *>( p ) );
	p += sizeof slp->numCommands;
	
	// look for a bufferCmd with its high bit set
	for ( int icmd = 0; icmd < nCmds; ++icmd )
		{
		const SndCommand * pcmd = reinterpret_cast<const SndCommand *>( p );
		p += sizeof(SndCommand);
		if ( EndianU16_NtoB(bufferCmd | dataOffsetFlag) == pcmd->cmd )
			{
			// param2 is the offset from the start of the handle to the SoundHeader
			offset = EndianU32_BtoN( pcmd->param2 );
			break;
			}
		}
	if ( not offset )
		return badFormat;
	
	// return result
	headerOffset = offset;
	
	return noErr;
}


/*
**	MyParseSndHeader()
**
**	homegrown replacement for deprecated ParseSndHeader() API.
**	NB: takes a pointer to 'snd ' data, not a Handle.
**	Input data (at *inPtr) expected to be big-endian, but results are native.
*/
OSStatus
MyParseSndHeader( const void * inPtr, SoundComponentData * cd,
				UInt32 * nFrames, UInt32 * sampleOffset )
{
	// paranoia
	if ( not cd )
		return paramErr;
	
	// locate the SoundHeader
	long hdrOffset = 0;
	OSStatus err = MyGetSoundHeaderOffset( inPtr, hdrOffset );
	if ( err )
		return err;
	
	// common values
	cd->flags = 0;
	cd->buffer = nullptr;
	cd->reserved = 0;
	
	// offset from 'inPtr' to start of samples
	UInt32 dataOff = 0;
	
	// it's hard to do pointer arithmetic on void *'s!
	const uchar * ptr = static_cast<const uchar *>( inPtr );
	const SoundHeaderUnion * up = reinterpret_cast<const SoundHeaderUnion *>( ptr + hdrOffset );
	
	// we only want the regular kind of headers, where the samples are in 'sampleArea'
	// (the 'samplePtr' field is at the same offset in all three variants of the union.)
	if ( up->stdHeader.samplePtr != nullptr )
		return badFormat;
	
	// rate field is at a constant offset, for all headers
	cd->sampleRate = EndianU32_BtoN( up->stdHeader.sampleRate );
	
	// decide what kind of header this is
	switch ( up->stdHeader.encode )
		{
		case stdSH:
			// these are exclusively for single-channel 8-bit offset binary.
			{
			cd->format = k8BitOffsetBinaryFormat;	// aka 'raw '
			cd->numChannels = 1;
			cd->sampleSize = 8;
			cd->sampleCount = EndianU32_BtoN( up->stdHeader.length );
			
			dataOff = up->stdHeader.sampleArea - ptr;
			}
			break;
		
		case cmpSH:
			// these are as close to being fully general as you can get (before CoreAudio)
			{
			// sanity-check the compressionID
			short compressionID = EndianS16_BtoN( up->cmpHeader.compressionID );
			if ( notCompressed != compressionID && fixedCompression != compressionID )
				return badFormat;
			
			cd->format = EndianU32_BtoN( up->cmpHeader.format );
			// sanity check this too
			if ( not IsFormatLPCM( cd->format ) )
				return badFormat;
			
			cd->numChannels = EndianU32_BtoN( up->cmpHeader.numChannels );
			cd->sampleSize = EndianU16_BtoN( up->cmpHeader.sampleSize );
			cd->sampleCount = EndianU32_BtoN( up->cmpHeader.numFrames );
			
			dataOff = up->cmpHeader.sampleArea - ptr;
			}
			break;
		
		case extSH:
			// these are always uncompressed, but may have more than 1 channel
			// and/or more than 8 bits/sample
			{
			cd->numChannels = EndianU32_BtoN( up->extHeader.numChannels );
			cd->sampleSize = EndianU16_BtoN( up->extHeader.sampleSize );
			cd->sampleCount = EndianU32_BtoN( up->extHeader.numFrames );
			if ( 8 == cd->sampleSize )
				cd->format = k8BitOffsetBinaryFormat;	// 'raw '
			else
			if ( 16 == cd->sampleSize )
				cd->format = k16BitBigEndianFormat;		// 'twos'
			else
				return badFormat;
			
			dataOff = up->extHeader.sampleArea - ptr;
			}
			break;
		
		default:
			return badFormat;
		}
	
	// supply results to caller
	if ( nFrames )
		*nFrames = cd->sampleCount;
	if ( sampleOffset )
		*sampleOffset = dataOff;
	
	return noErr;
}


#if 0  // unused, unless we decide to support compressed (non-LPCM) 'snd ' data.
/*
**	MyGetCompressionInfo()
**
**	home-grown replacement for deprecated GetCompressionInfo() API.
**	Unlike the official routine, this version only handles a very limited subset of compression
**	IDs and formats: specifically, 8-bit 'raw ' and 16-bit 'twos'. (Not even 'sowt'!)
**	Input data is expected to be big-endian, but results (in *ci) are native.
*/
OSStatus
MyGetCompressionInfo( short compressionID, OSType fmt,
					short numChannels, short sampleSize, CompressionInfo * ci )
{
	// paranoia
	if ( not ci )
		return paramErr;
	
	// bail for formats we don't (want to) understand
	if ( fixedCompression != compressionID && notCompressed != compressionID )
		return badFormat;
	
	if ( not IsFormatLPCM( fmt ) )
		return badFormat;
	
	// some CompressionInfo fields are constant
	ci->recordSize = sizeof *ci;
	ci->format = fmt;
	ci->compressionID = notCompressed;	// ???
//	if ( numChannels > 1 || sampleSize != 8 )
//		ci->compressionID = fixedCompression;
	ci->futureUse1 = 0;
	
	switch ( fmt )
		{
		case k8BitOffsetBinaryFormat:
		case k16BitBigEndianFormat:
			// these values should apply for any LPCM format, n'est-ce-pas?
			ci->samplesPerPacket = 1;
			ci->bytesPerFrame = numChannels * sampleSize / 8;
			ci->bytesPerPacket =
			ci->bytesPerSample = sampleSize / 8;
			break;
		
//		case k16BitLittleEndianFormat:
		default:
			return badFormat;
		}
	return noErr;
}
#endif  // 0


#pragma mark -
#pragma mark DTSSoundPriv

DTSDefineImplementFirmNoCopy(DTSSoundPriv)


/*
	It's important to note that the lifetimes of DTSSounds (and their -Privs) must be
	totally independent of those of SoundChannels. The former is controlled by the
	code that uses them; the latter lives only while the sound is actually being played.
	Hence, at one extreme you might have a transiently-allocated DTSSound object that happens
	to be asked to play a very lengthy sound (i.e. the channel sticks around for far longer
	than the DTSSound); at the other extreme you might have a DTSSound on the stack that
	receives multiple Play() requests (i.e. the channels come and go many times during the
	lifetime of the single DTSSoundPriv).
	
	So far no big deal, but it gets tricky as soon as you realize that both kinds of objects
	would like to have access to their counterparts. For example, DTSSound::Stop()
	wants to talk to the associated running SoundChannel, if there is one; conversely,
	SoundChannel::FindChannelForSound() obviously need to be able to find the channel (if any)
	that's playing a given sound.
	
	This cross-referencing is handled by the 'DTSSoundPriv::sndChannel' and
	'SoundChannel::mSource' fields. But then what about the problem of stale pointers,
	if we have no guarantees about the classes' relative lifetimes?
	My solution is to have the destructors for DTSSoundPrivs and SoundChannels cooperate
	to implement mutual auto-zeroing of their correspondents' respective back-pointers.
	Whenever either kind of object is destructed, if its counterpart still exists,
	the reverse pointer in that counterpart gets zero'ed out.
	
	What this means is that, in (e.g.) DTSSoundPriv code, you can safely write
	
		if ( sndChannel )
			sndChannel->DoStuff();
	
	with confidence that, had the channel already been deleted at that point, then the
	'sndChannel' pointer would have been nulled out; and vice-versa for SoundChannels and
	their 'mSource' pointers.
*/


/*
**	DTSSoundPriv::DTSSoundPriv()
*/
DTSSoundPriv::DTSSoundPriv() :
	sndBytes( nullptr ),
	sndLen( 0 ),
	sndChannel( nullptr ),
	sndDuration( -1 )	// -1 indicates "as yet undetermined"
{
	memset( &sndDesc, 0, sizeof sndDesc );
}


/*
**	DTSSoundPriv::~DTSSoundPriv()
*/
DTSSoundPriv::~DTSSoundPriv()
{
	// if we still have a channel, tell it we're gone
	if ( sndChannel )
		sndChannel->SetSource( nullptr );
	
	// release the sample buffer
	delete[] sndBytes;
}


/*
**	DTSSoundPriv::Reset()
**
**	release sample buffer and reset fields, in case this DTSSoundPriv ever gets reused
**	(e.g., if someone calls DTSSound::Init() twice on the same object)
*/
void
DTSSoundPriv::Reset()
{
	memset( &sndDesc, 0, sizeof sndDesc );
	delete[] sndBytes;
	sndBytes = nullptr;
	sndLen = 0;
	sndChannel = nullptr;
	sndDuration = -1;	// just in case
}


/*
**	DTSSoundPriv::Init()
**
**	given a pointer to (and length of) a chunk of bytes -- which are assumed to be in the
**	format of a (big-endian) SndListResource (i.e. the contents of a 'snd ' resource),
**	prepare to play it via an AudioQueue.
*/
OSStatus
DTSSoundPriv::Init( const uchar * inSndBytes, size_t len )
{
	// just in case
	Reset();
	
	// extract essential sound info;
	// fill out the basic-description record;
	// get pointer to first byte of the actual samples, and length thereof
	return InitFromSndListBytes( inSndBytes, len );
	
	// in theory we could have additional decoder methods such as InitFromAIFFData() or
	// InitFromWAVData or InitFromMP3File(); then this Init() method would simply need
	// an additional argument to specify which decoder to apply.
	// (Although for MP3's and the like, you'd need a more scalable buffering scheme;
	// see the Eric's Ultimate Solitaire sources for some ideas on that.)
}


/*
**	DTSSoundPriv::InitFromSndListBytes()
**
**	decode 'snd '-style data, yielding an AudioStreamBasicDescription (ASBD) as well as
**	a pointer to (and length of) a newly-allocated sample buffer.
**
**	How it works:  First, we parse the 'snd ' header and its internal SoundHeader structure
**	(which might actually be a CmpSoundHeader or an ExtSoundHeader), to determine the sound's
**	characteristics.  From that, next, we can fill out our ASBD, and also make a private copy
**	of the sound's sample data.
**
**	This copying may seem excessive and wasteful -- and it is! -- but is currently required
**	by the client's data-caching architecture.
**
**	There might be a case to be made for adding a new public DTSSound entry point:
**		DTSSound::Play( DTSKeyFile * file, DTSKeyType type, DTSKeyID sndID );
**	since, after all, our sounds' data always comes exclusively from keyfile records.
**	That way there'd be no need to store multiple copies of the sample data, which could
**	instead be read and parsed just-in-time, viz., in the AudioQueue buffer-proc, which
**	would also let us use more reasonably-sized (i.e. smaller) AQ buffers;
**	moreover, the client could stop caching sounds entirely.  In fact we could probably
**	mmap(2) the entirety of CL_Sounds and fill the AQ buffers directly from there.
*/
OSStatus
DTSSoundPriv::InitFromSndListBytes( const uchar * inSndBytes, size_t inLen )
{
	// extract essential sound info
	SoundComponentData cd;
//	memset( &cd, 0, sizeof cd );
	UInt32 numFrames( 0 ), dataOffset( 0 );
	OSStatus err = MyParseSndHeader( inSndBytes, &cd, &numFrames, &dataOffset );
	__Check_noErr( err );
	
	// get pointer to first byte of the actual samples
	size_t dataLen(0);	// can't know this until we decide if the sound was compressed
	
	// fill out the basic-description record
	if ( noErr == err )
		{
		// ... depending on whether or not there's any compression involved
		if ( IsFormatLPCM( cd.format ) )	// some sort of LPCM?
			{
			// <CoreAudioTypes.h> has a handy utility to do that for us
			FillOutASBDForLPCM( sndDesc,
				cd.sampleRate / 65536.0,	// 16-bit shift for Fixed -> double
				cd.numChannels,				// channels per frame
				cd.sampleSize,				// valid bits per sample
				cd.sampleSize,				// total bits per samle
				false,						// not float
				k16BitBigEndianFormat == cd.format );		// big-endian?
			
			// mark 8-bit 'raw ' formats as being unsigned
			// since they're stored as offset-binary in the 'snd ' sample data, not as signed
			if ( (k8BitOffsetBinaryFormat == cd.format || k16BitBigEndianFormat == cd.format)
			&&   8 == cd.sampleSize )
				{
				// turn off the is-signed bit
				sndDesc.mFormatFlags &= ~ kLinearPCMFormatFlagIsSignedInteger;
				}
			
			dataLen = cd.numChannels * cd.sampleCount * (cd.sampleSize / 8);	// @ 8 bits/byte
			}
		else
			{
#if 1
			// as per comment below, there are no more compressed sounds in CL_Sounds
			// so we need not support them any longer.
# pragma unused( inLen )
			
			err = badFormat;
#else
			// the sound IS compressed.
			// We shouldn't ever get here, since there are no longer [v837]
			// any compressed items in CL_Sounds.
			// However we can't guarantee that some GM won't manage to add some new
			// ones in future...
			
			// The only compression types we understand are IMA4 and MACE3:1
			__Check( kAudioFormatAppleIMA4 == cd.format || kAudioFormatMACE3 == cd.format );
			
			// if the sound is compressed, we'll need to learn the compression format used
			// and some compression parameters
			CompressionInfo cmpInfo;
			if ( noErr == err )
				{
				err = MyGetCompressionInfo( fixedCompression, cd.format,
							cd.numChannels, cd.sampleSize, &cmpInfo );
				__Check_noErr( err );
				
				dataLen = numFrames * cmpInfo.bytesPerFrame;
				}
			
			// paranoia : ensure our calculated sample area doesn't overflow the end of the handle
			__Check( dataOffset + dataLen <= inLen );
			
			sndDesc.mSampleRate = cd.sampleRate / 65536.0;	// Fixed -> Float64
			sndDesc.mFormatID = cd.format;
			sndDesc.mFormatFlags = 0;
			
			// this works but I don't know why.
			// ... seems like the ASBD and CompressionInfo field names have swapped their
			// interpretations of the words "frame" and "packet"???
			
//			sndDesc.mBytesPerPacket = cmpInfo.bytesPerPacket * cd.numChannels;
			sndDesc.mBytesPerPacket = cmpInfo.bytesPerFrame;
			
//			sndDesc.mFramesPerPacket = cmpInfo.bytesPerPacket / cmpInfo.bytesPerFrame;
			sndDesc.mFramesPerPacket = cmpInfo.samplesPerPacket;
			
//			sndDesc.mBytesPerFrame = cmpInfo.bytesPerFrame;	// should be 0?
			sndDesc.mBytesPerFrame = 0;
			
			sndDesc.mChannelsPerFrame = cd.numChannels;
			sndDesc.mBitsPerChannel = 0;
			sndDesc.mReserved = 0;
# if 0 && defined( DEBUG_VERSION )
	// DEBUG
			fprintf( stderr, "B/P: %lu, F/P: %lu, #Ch: %lu\n",
				sndDesc.mBytesPerPacket,
				sndDesc.mFramesPerPacket,
				sndDesc.mChannelsPerFrame );
# endif  // 0
#endif  // 1
			}
		}
	
	// make a private copy (sigh) of just the sample data
	if ( noErr == err )
		err = SaveSampleData( inSndBytes + dataOffset, dataLen );
	
	return err;
}


/*
**	DTSSoundPriv::SaveSampleData()
**
**	once we've fully decoded a 'snd ' header, make a local copy of its sample data,
**	which we'll eventually pass on to a SoundChannel's audio-queue buffer. 
**	(Our own copy will eventually be discarded by our destructor). 
*/
OSStatus
DTSSoundPriv::SaveSampleData( const uchar * inData, size_t inLen )
{
	// cache our own private copy of just the sample data -- no headers at all
	if ( uchar * sampleBytes = NEW_TAG("SoundSampleBuffer") uchar[ inLen ] )
		{
		memcpy( sampleBytes, inData, inLen );
		
		// and set instance vars to point to it
		sndBytes = sampleBytes;
		sndLen = inLen;
		
		return noErr;
		}
	
	// failed to allocate memory
	return memFullErr;
}


/*
**	DTSSoundPriv::DidFinishPlayback()
**
**	update timing statistic, once the sound is done
*/
void
DTSSoundPriv::DidFinishPlayback( CFAbsoluteTime startTime )
{
	sndDuration = CFAbsoluteTimeGetCurrent() - startTime;
}


#pragma mark -
#pragma mark SoundChannel

/*
**	SoundChannel::SoundChannel()
**
**	constructor
*/
SoundChannel::SoundChannel( DTSSoundPriv * sp ) :
	mQueue( nullptr ),
	mBuffer( nullptr ),
	mSource( sp ),
	mStartedTime( 0 ),
	mStillPlaying( false )		// we haven't finished playing yet
{
	// insert ourselves into the linked list
	InstallFirst( gRootChannel );
}


/*
**	SoundChannel::~SoundChannel()
**
**	destructor
*/
SoundChannel::~SoundChannel()
{
	// if we still have a source sound, tell it we're gone
	if ( mSource )
		mSource->sndChannel = nullptr;
	
	// unlink ourselves from root
	Remove( gRootChannel );
	
	// get rid of the AudioQueue
	if ( mQueue )
		{
		Stop();		// just in case
		__Verify_noErr( AudioQueueDispose( mQueue, false ) );
		}
}


#if MAC_OS_X_VERSION_MIN_REQUIRED <= MAC_OS_X_VERSION_10_5
/*
**	SoundChannel::BufferProc()		[static; not needed for 10.6+]
**
**	When this function is called, we merely have to note that the sound has finished.
**	There's no buffer refilling to be done.
*/
void
SoundChannel::BufferProc( void * /* ud */, AudioQueueRef inAQ, AudioQueueBufferRef /* inBuf */ )
{
//	SoundChannel * channel = static_cast<SoundChannel *>( ud );
//	fprintf( stderr, "BufPrc: ch %p, q %p\n", channel, inAQ );
	
	// tell the stream to stop
	__Verify_noErr( AudioQueueStop( inAQ, false ) );
}
#endif  // <= 10.5


/*
**	SoundChannel::PropertyListenerProc()		[static]
**
**	a callback invoked whenever a certain queue property changes.
**	Currently we only register to watch the _IsRunning property
*/
void
SoundChannel::PropertyListenerProc( void * ud, AudioQueueRef inAQ, AudioQueuePropertyID inID )
{
	SoundChannel * channel = static_cast<SoundChannel *>( ud );
	
//	__Check( kAudioQueueProperty_IsRunning == inID );
	if ( kAudioQueueProperty_IsRunning == inID )
		{
		UInt32 isRunning = false;
		UInt32 size = sizeof isRunning;
		__Verify_noErr( AudioQueueGetProperty( inAQ, kAudioQueueProperty_IsRunning,
							&isRunning, &size ) );
		
//		fprintf( stderr, "PropListnr: i %p, q %p: %s\n", channel, inAQ,
//					channel->mStillPlaying ? "running" : "done" );
		
//		__Check( inAQ == channel->mQueue );
		
		channel->mStillPlaying = (isRunning != 0);
		
		// once playback stops, we can calculate the sound's duration
		if ( not isRunning && channel->mSource )
			channel->mSource->DidFinishPlayback( channel->mStartedTime );
		}
}


/*
**	SoundChannel::CleanupProc()
**
**	EventLoopTimer callback.  This function is called 4x/sec, so long as any
**	SoundQueues are still actively playing.
**	We scan the list of SoundQueues, deleting any which are now finished.
**	If no busy sounds are encountered, then we disable the timer callback.
*/
void
SoundChannel::CleanupProc( EventLoopTimerRef /* timer */, void * /* ud */ )
{
	// assume they're all done playing
	bool bAllSoundsDone = true;
	
	// examine each one
	const SoundChannel * next = nullptr;
	for ( const SoundChannel * chan = gRootChannel; chan; chan = next )
		{
		next = chan->linkNext;
		
		if ( chan->IsDone() )
			{
			// finished sounds need to have their resources reclaimed.
			// don't allow exceptions to propagate out
			try
				{
				delete chan;
				}
			catch ( ... ) {}
			}
		else
			{
			// at least one sound is not yet done
			bAllSoundsDone = false;
			}
		}
	
	// if no sounds are active, then we can put the timer to sleep
	if ( bAllSoundsDone && sCleanupTimer )
		__Verify_noErr( SetEventLoopTimerNextFireTime( sCleanupTimer,
							kEventDurationForever ) );
}


/*
**	SoundChannel::FindChannelForSound()		[static]
**
**	try to look up which SoundChannel is currently assigned to the given DTSSound
*/
SoundChannel *
SoundChannel::FindChannelForSound( const DTSSoundPriv * sp )
{
	for ( SoundChannel * chan = gRootChannel; chan; chan = chan->linkNext )
		{
		if ( sp == chan->mSource )
			return chan;
		}
	
	return nullptr;
}


// deal with __BLOCKS__

#if MAC_OS_X_VERSION_MIN_REQUIRED > MAC_OS_X_VERSION_10_5 && defined( __BLOCKS__ )
/*
**	SoundChannel::CreateAudioQueue()
**
**	encapsulates creation of the AudioQueue
**	This version, for runtimes known to be 10.6 or higher, avoids the weak-link test
**	and pre-BLOCKS fallback code path.
*/
OSStatus
SoundChannel::CreateAudioQueue( const AudioStreamBasicDescription * asbd )
{
	return AudioQueueNewOutputWithDispatchQueue( &mQueue,
				asbd,
				kNilOptions,
				dispatch_get_global_queue( DISPATCH_QUEUE_PRIORITY_DEFAULT, 0 ),
				^( AudioQueueRef q, AudioQueueBufferRef /* b */ )
					{
					AudioQueueStop( q, false );
					} );
}

#else  // < 10.6

/*
**	SoundChannel::CreateAudioQueue()
**
**	encapsulates creation of the AudioQueue
*/
OSStatus
SoundChannel::CreateAudioQueue( const AudioStreamBasicDescription * asbd )
{
	OSStatus result;
	
# if defined( __BLOCKS__ )
	// SDK & compiler know about this newer API, but the runtime might not.
	// Test for a successful weak link.
	if ( &AudioQueueNewOutputWithDispatchQueue != nullptr )
		{
		// same as above
		result = AudioQueueNewOutputWithDispatchQueue( &mQueue,
					asbd,
					kNilOptions,
					dispatch_get_global_queue( DISPATCH_QUEUE_PRIORITY_DEFAULT, 0 ),
					^( AudioQueueRef q, AudioQueueBufferRef /* b */ )
						{
						AudioQueueStop( q, false );
						} );
		}
	else
# endif  // __BLOCKS__
		{
		// we get here if building on a pre-10.6 SDK, or on a non-Blocks-capable compiler,
		// or (hopefully, and most importantly) if _running_ under 10.5.
		result = AudioQueueNewOutput( asbd,				// description
									 BufferProc,		// buffer-filler callback
									 this,				// "refcon"
									 nullptr,			// use default runloop
									 nullptr,			// i.e. kCFRunLoopCommonModes
									 kNilOptions,		// no options
									 &mQueue );			// resultant AudioQueueRef
		}
	
	return result;
}
#endif  // 10.6+


/*
**	SoundChannel::Play()
**
**	given a sound sample buffer (etc.) in 'p', start playing it via an AudioQueue.
*/
OSStatus
SoundChannel::Play( const DTSSoundPriv * p )
{
	// rearm the clean-up timer, at 4Hz
	if ( sCleanupTimer )
		__Verify_noErr( SetEventLoopTimerNextFireTime( sCleanupTimer, 0.25 ) );
	
	// create our AudioQueue
	OSStatus err = CreateAudioQueue( &p->sndDesc );
	__Check_noErr( err );
	
	// allocate the queue's sole buffer
	if ( noErr == err )
		{
		err = AudioQueueAllocateBuffer( mQueue, p->sndLen, &mBuffer );
		__Check_noErr( err );
		}
	
	// fill the buffer with the sample data
	if ( noErr == err )
		{
		// paranoia
		__Check( mBuffer->mAudioDataBytesCapacity >= p->sndLen );
		
		memcpy( mBuffer->mAudioData, p->sndBytes, p->sndLen );
		mBuffer->mAudioDataByteSize = p->sndLen;
		
#if MAC_OS_X_VERSION_MAX_ALLOWED > MAC_OS_X_VERSION_10_5
		// this field not present in 10.5 SDK -- weird
		mBuffer->mPacketDescriptionCount = 0;
#endif  // 10.6+
		
		// queue it up
		err = AudioQueueEnqueueBuffer( mQueue, mBuffer, 0, nullptr );
		__Check_noErr( err );
		}
	
	// install our is-done property listener, so we know when the sound finishes
	if ( noErr == err )
		{
		err = AudioQueueAddPropertyListener( mQueue,
					kAudioQueueProperty_IsRunning, PropertyListenerProc, this );
		__Check_noErr( err );
		}
	
	// set the volume
	if ( noErr == err )
		{
		err = AudioQueueSetParameter( mQueue, kAudioQueueParam_Volume, sDefaultVolume );
		__Check_noErr( err );
		}
	
	// start playback!
	mStartedTime = CFAbsoluteTimeGetCurrent();
	if ( noErr == err )
		{
		err = AudioQueueStart( mQueue, nullptr );
//		__Check_noErr( err );
		}
	
	// if all the above worked, great: we're off and running. Otherwise, let's at least
	// mark this SoundChannel as being deletable by the CleanupProc routine.
	mStillPlaying = (noErr == err);
	
	return err;
}


/*
**	SoundChannel::Stop()
**
**	stop a currently-playing sound queue.
*/
OSStatus
SoundChannel::Stop()
{
	OSStatus result = noErr;
	if ( mQueue && mStillPlaying )
		{
		// this is an async stop, since that makes the coding easier (no need
		// to call AudioQueueFlush before AudioQueueStop).
		// Should we support synchronous stops too?
		result = AudioQueueStop( mQueue, false );
		__Check_noErr( result );
//		mStillPlaying = false;	// the listener proc will do this, at the right time
		}
	
	return result;
}


/*
**	SoundChannel::IsDone()
**
**	test whether or not this channel is currently emitting a sound
*/
bool
SoundChannel::IsDone() const
{
	if ( mQueue )
		return not mStillPlaying;
	
	return true;	// ? never been queued; does that count as "done"?
}


/*
**	SoundChannel::SetVolume()
**
**	change the volume of a currently-playing sound
*/
OSStatus
SoundChannel::SetVolume( Float32 newVolume )
{
	OSStatus result = noErr;
	if ( mQueue && mStillPlaying )
		{
		result = AudioQueueSetParameter( mQueue, kAudioQueueParam_Volume, newVolume );
		__Check_noErr( result );
		}
	return result;
}


// static "entry" points, accessed via non-class functions in Sound_dts.h

/*
**	SoundChannel::Init()		[static]
**
**	install the cleanup timer.
*/
OSStatus
SoundChannel::Init()
{
	// it's initially disabled, but will recur at 4Hz when/if re-enabled
	OSStatus err = InstallEventLoopTimer( GetMainEventLoop(),
						kEventDurationForever,
						0.25,
						CleanupProc,
						nullptr,
						&sCleanupTimer );
	__Check_noErr( err );
	return err;
}


/*
**	SoundChannel::Exit()		[static]
**
**	disable & remove the cleanup timer, and mop up any remaining sounds
*/
void
SoundChannel::Exit()
{
	if ( sCleanupTimer )
		{
		__Verify_noErr( SetEventLoopTimerNextFireTime( sCleanupTimer,
							kEventDurationForever ) );
		__Verify_noErr( RemoveEventLoopTimer( sCleanupTimer ) );
		sCleanupTimer = nullptr;
		}
	
	DeleteLinkedList( gRootChannel );
}


/*
**	SoundChannel::SetAllVolumes()		[static]
**
**	change the volume of all currently-playing sounds
*/
void
SoundChannel::SetAllVolumes( Float32 volume )
{
	for ( SoundChannel * channel = gRootChannel; channel; channel = channel->linkNext )
		channel->SetVolume( volume );
}

#pragma mark -
#pragma mark DTSSound


/*
**	DTSSound::Init()
**
**	Load a sound from memory, which is expected to conform to the structure of
**	a 'snd ' resource, in standard (big-endian) orientation.
*/
DTSError
DTSSound::Init( const void * ptr, size_t len )
{
	if ( DTSSoundPriv * p = priv.p )
		return p->Init( static_cast<const uchar *>( ptr ), len );
	
	return -1;
}


/*
**	DTSSound::Play()
**
**	start playing the sound, on a newly-allocated sound channel.
**	Don't allocate more channels than the maximum, as set by DTSInitSound().
**	If no more channels are permitted, nothing gets played. (This is not an error.)
**	
**	[However... Do we really need to honor that limit? Why not play every sound we're asked
**	to emit?  Are there cases where the CL client tries to play many, many sounds simultaneously?]
**
**	I didn't bother to re-implement the old DTSSound's ability to play looping sounds,
**	via the 'bContinuous' argument, since the CL client has no need for that feature.
*/
void
DTSSound::Play()
{
	DTSSoundPriv * p = priv.p;
	if ( not p )
		return;
	
	// it turn out that if we reject duplicate sounds (by enabling this next section),
	// then if you visit "noisy" environments in the client (lots of repeated/recurring
	// sounds), there'll be a lot of unwanted sound drop-outs.
	// Playing _all_ sounds, as requested -- even dupes -- seems to cure the dropout problem.
#if 0
	// are we already being played?
	if ( SoundChannel::FindChannelForSound( p ) )
		{
# ifdef DEBUG_VERSION
		fprintf( stderr, "Not playing sound: duplicate\n" );
# endif
		return;
		}
#endif
	
	// don't play too many sounds at the same time
	int numInUse = SoundChannel::NumInUse();
	if ( numInUse >= sMaxSounds )
		{
#if 0 && defined( DEBUG_VERSION )
		// suggest maybe raising the channel limit
		fprintf( stderr, "too many sounds: only %d allowed\n", sMaxSounds );
#endif
		return;	// comment this line out if we decide to not actually honor the limit
		}
	
	// allocate a new SoundChannel to play on. The channel will be reclaimed, once it's done
	// playing, by the channel's timer function
	if ( SoundChannel * chan = NEW_TAG("SoundChannel") SoundChannel( p ) )
		{
		// remember it
		p->sndChannel = chan;
		
		// make it go
		(void) chan->Play( p );
		}
}


/*
**	DTSSound::Stop()
**
**	Stop this sound.
*/
void
DTSSound::Stop()
{
	if ( DTSSoundPriv * p = priv.p )
		if ( SoundChannel * channel = p->sndChannel )
			channel->Stop();
}


/*
**	DTSSound::IsDone()
**
**	Test if a sound has finished playing.
*/
bool
DTSSound::IsDone() const
{
	if ( const DTSSoundPriv * p = priv.p )
		if ( const SoundChannel * channel = p->sndChannel )
			return channel->IsDone();
	
	// this sound never started being played
	// or else it has already completed and its queue's been disposed of.
	// Either way, it's assuredly not still running.
	return true;
}


/*
**	GetSoundDuration()
**
**	return the sound's playback duration, if we know it.
**	yields -1 if the sound has never been played, 0 on various other errors.
*/
CFTimeInterval
GetSoundDuration( const DTSSound * snd )
{
	if ( snd )
		if ( const DTSSoundPriv * p = snd->priv.p )
			return p->GetDuration();
	
	return 0;
}

