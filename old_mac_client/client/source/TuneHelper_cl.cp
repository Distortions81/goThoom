/*
**	TuneHelper_cl.cp		ClanLord
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

#include "TuneHelper_cl.h"


// for my debug stuff
#ifndef V
# define V(w)
#endif

#ifdef CL_TUNE_HELPER_APP
# ifdef NEW_TAG
#  undef NEW_TAG
# endif
# define NEW_TAG(c)	new
#else
# include "ClanLord.h"
#endif	// CL_TUNE_HELPER_APP


// if true, incoming tunes are logged to a text file
#define	TUNE_LOG_ENABLE			0
	
// if set, each new tune appends to the log.
// otherwise, each new tune completely overwrites the log.
#define TUNE_LOG_CUMULATIVE		1

// name of tune-logging file
#define TUNE_LOG_PATH			"BardMusic.txt"


const char * const
CTuneBuilder::sErrorMessages[] =
{
	TXTCL_TUNE_ERROR_NO, // "No Error",															//	error_None
	TXTCL_TUNE_ERROR_INV_NOTE, // "The music contains an invalid character",					//	error_InvalidNote
	TXTCL_TUNE_ERROR_INV_MODIFIER, // "The music contains an invalid note modifier",			//	error_InvalidModifier
	TXTCL_TUNE_ERROR_TONEOVERFLOW, // "A note is out of the allowed octaves",					//	error_ToneOverflow
	TXTCL_TUNE_ERROR_POLYPHONYOVERFLOW, // "Too many chord notes are playing for this instrument",	//	error_PolyphonyOverflow
	TXTCL_TUNE_ERROR_INV_OCTAVE, // "A note is out of the allowed octaves",						//	error_InvalidOctave
	TXTCL_TUNE_ERROR_INV_CHORD, // "Too many chord notes are playing for this instrument",		//	error_InvalidChord
	TXTCL_TUNE_ERROR_UNSUPPORTED_INSTRUMENT, // "Long chords are not supported on this instrument",	//	error_UnsupportedInstrument
	TXTCL_TUNE_ERROR_OUTOFMEMORY, // "Ran out of memory while building tune",					//	error_OutOfMemory
	TXTCL_TUNE_ERROR_TOMANYMARKS, // "Loops are nested too deeply",								//	error_TooManyMarks
	TXTCL_TUNE_ERROR_UNMATCHEDMARK, // "There is an end loop mark without a start",				//	error_UnmatchedMark
	TXTCL_TUNE_ERROR_UNMATCHEDCOMMENT, // "Invalid comment termination",						//	error_UnmatchedComment
	TXTCL_TUNE_ERROR_INV_TEMPO, // "A tempo is outside the allowed values",						//	error_InvalidTempo
	TXTCL_TUNE_ERROR_INV_TEMPCHANGE, // "The tempo cannot change while chords are playing",		//	error_InvalidTempoChange
	TXTCL_TUNE_ERROR_MODIFIERNEEDVALUE, // "A tempo modifier is missing its value",				//	error_ModifierNeedValue
	TXTCL_TUNE_ERROR_DUPLICATEENDING, // "An ending number is defined more than once",			//	error_DuplicateEnding
	TXTCL_TUNE_ERROR_DUPLICATEDEFAULTENDING, // "A default ending is defined more than once",	//	error_DuplicateDefEnding
	TXTCL_TUNE_ERROR_ENDINGINCHORD, // "Ending marks cannot be specified in chords",			//	error_EndingInChord
	TXTCL_TUNE_ERROR_ENDINGOUTSIDELOOP, // "Ending marks cannot be specified outside loops",	//	error_EndingOutsideLoop
	TXTCL_TUNE_ERROR_DEFAULTENDINGERROR, // "Default endings cannot have an index",				//	error_DefaultEndingError
	TXTCL_TUNE_ERROR_INV_ENDINGINDEX, // "An ending index is not valid",						//	error_InvalidEndingIndex
	TXTCL_TUNE_ERROR_UNTERMINATEDLOOP, // "A loop is missing its end mark",						//	error_unterminatedLoop
	TXTCL_TUNE_ERROR_UNTERMINATEDCHORD, // "A chord is missing its end mark",					//	error_unterminatedChord
	TXTCL_TUNE_ERROR_ALREADYPLAYING // "You are already playing"								//	error_AlreadyPlaying
									// technically part of CTuneQueue, but convenient here
};


#pragma mark -- CDataHandle --

//
//	constructor
//	allocate the zero-size container
//
CDataHandle::CDataHandle()
	: mData( nullptr )
{
#if 1
	mData = CFDataCreateMutable( kCFAllocatorDefault, 0 );
#elif
	mData = NewHandle( 0 );
#endif
	
	// if the above fails, we're royally hosed
	__Check( mData );
}


//
//	destructor
//	toss the handle
//
CDataHandle::~CDataHandle()
{
	if ( mData )
		{
#if 1
		CFRelease( mData );
#else
		::DisposeHandle( mData );
		__Check_noErr( ::MemError() );
#endif
		}
}


/*
**	CDataHandle::Add()
**
**	grow the container, and append (optional) data
**	return pointer to the start of the newly-added space
*/
char *
CDataHandle::Add( const void * inData, long inSize )
{
#if 1
	CFIndex oldsize = CFDataGetLength( mData );
	if ( inData )
		CFDataAppendBytes( mData, static_cast<const uchar *>( inData ), inSize );
	else
		CFDataSetLength( mData, oldsize + inSize );
	
	return (char *) CFDataGetMutableBytePtr( mData ) + oldsize;
#else
	long oldsize = ::GetHandleSize( mData );
	__Check_noErr( ::MemError() );
	
	::HUnlock( mData );
	__Check_noErr( ::MemError() );
	::SetHandleSize( mData, oldsize + inSize );
	__Check_noErr( ::MemError() );
	if ( ::MemError() )
		return nullptr;
	::HLock( mData );
	__Check_noErr( ::MemError() );
	
	if ( inData )
		::BlockMoveData( inData, *mData + oldsize, inSize );
	
	return *mData + oldsize;
#endif  // 1
}


/*
**	CDataHandle::Reset()
**
**	empty out the container
*/
void
CDataHandle::Reset()
{
#if 1
	CFDataDeleteBytes( mData, CFRangeMake( 0, CFDataGetLength( mData ) ) );
#else
	SetHandleSize( mData, 0 );
	__Check_noErr( ::MemError() );
#endif
}


#pragma mark -- CQTMusic --

#ifdef CL_TUNE_HELPER_APP
//
//	constructor
//	init the sound generator
//
CQTMusic::CQTMusic()
		:	mNumNc( 0 )
{
	mNa = ::OpenDefaultComponent( kNoteAllocatorComponentType, 0 );
	for ( int i = 0; i < max_Channels; ++i )
		mNc[i] = nullptr;
}


//
//	destructor
//
CQTMusic::~CQTMusic()
{
	for ( int i = 0; i < max_Channels; ++i )
		if ( NoteChannel nc = mNc[i] )
			::NADisposeNoteChannel( mNa, nc );
	if ( mNa )
		::CloseComponent( mNa );
	mNa = nullptr;
}


//
//	CQTMusic::DisposeNoteChannel()
//	close all open channels
//
void
CQTMusic::DisposeNoteChannel( short inChannel )
{
	if ( not mNa )
		return;
	
	if ( mNc[ inChannel ] )
		::NADisposeNoteChannel( mNa, mNc[ inChannel ] );
	
	mNc[ inChannel ] = nullptr;
}


//
//	CQTMusic::NewNoteChannel
//	(re)open one of the note channels
//
ComponentResult
CQTMusic::NewNoteChannel( short inChannel, short inPolyphonie )
{
	if ( not mNa )
		return noErr;
	
	DisposeNoteChannel( inChannel );
	
	NoteRequestInfo& nri = mNoteRequest[ inChannel ].info;
	nri.flags 					= 0;
	nri.midiChannelAssignment	= 0;
	nri.polyphony				= EndianU32_NtoB( inPolyphonie );
	nri.typicalPolyphony		= EndianU32_NtoB( 0x00010000 );
	
	mNoteRequest[ inChannel ].tone = mInstrument;
	
	ComponentResult err = ::NANewNoteChannel( mNa, &mNoteRequest[inChannel], &mNc[inChannel] );
	__Check_noErr( err );
	return err;
}


//
//	CQTMusic::PickInstrument()
//	let user select an instrument via system picker UI
//
OSStatus
CQTMusic::PickInstrument( ToneDescription *	outDescription )
{
	if ( not mNa )
		return paramErr;
	
	const UInt32 kPickFlags = kPickEditAllowEdit | kPickEditAllowPick | kPickEditControllers;
	ComponentResult err = ::NAPickInstrument( mNa, nullptr,
								"\pPick an instrument",
								&mInstrument,
								kPickFlags,
								reinterpret_cast<long>( this ), 0, 0 );
	if ( not err && outDescription )
		*outDescription	= mInstrument;
	if ( not err )
		{
		for ( int i = 0; i < max_Channels; ++i )
			if ( mNc[i] )
				NewNoteChannel( i, mNoteRequest[i].info.polyphony );
		}
	return err;
}


//
//	CQTMusic::SetInstrument()
//	assign an instrument to the given channel
//
OSStatus
CQTMusic::SetInstrument( short inInstrument, ToneDescription * outDescription )
{
	if ( not mNa )
		return paramErr;
	
	ComponentResult err = ::NAStuffToneDescription( mNa, inInstrument, &mInstrument );
	if ( not err && outDescription )
		*outDescription = mInstrument;
	
	if ( not err )
		{
		for ( int i = 0; i < max_Channels; ++i )
			if ( mNc[i] )
				NewNoteChannel( i, mNoteRequest[i].info.polyphony );
		}
	return err;
}


//
//	CQTMusic::PlayNote()
//	play one note, instantaneously
//
void
CQTMusic::PlayNote( short inChannel, short inNote, short inVelocity )
{
	if ( not mNa )	return;
	if ( inChannel < 0 || inChannel >= max_Channels || not mNc[ inChannel ] )	return;
	
	__Verify_noErr( ::NAPlayNote( mNa, mNc[ inChannel ], inNote, inVelocity ) );
}


//
// CQTMusic::StopNote()
//	stop playing one note
//
void
CQTMusic::StopNote( short inChannel, short inNote )
{
	if ( not mNa )	return;
	if ( inChannel < 0 || inChannel >= max_Channels || not mNc[ inChannel ] )	return;
	
	__Verify_noErr( ::NAPlayNote( mNa, mNc[ inChannel ], inNote, 0 ) );
}
#endif  // CL_TUNE_HELPER_APP


#pragma mark -- CCLInstrument --

/*
**	CCLInstrument::CCLInstrument()
**	constructor
*/
CCLInstrument::CCLInstrument() :
	mOctaveOffset( +1 ),
	mFlags( flags_Custom ),
	mChordVelocity( 100 ),
	mMelodyVelocity( 100 )
{
	memset( mRestrictions, 0, sizeof mRestrictions );
	mName[0] = '\0';
}


// translate MIDI note numbers to/from offsets, 0-based relative to instrument's gamut
inline short
CCLInstrument::NoteToOffset( short inNote ) const
{
	return inNote - (kMiddleC - kOctave + (mOctaveOffset * kOctave));
}


inline short
CCLInstrument::OffsetToNote( short inOffset ) const
{
	return (kMiddleC - kOctave + (mOctaveOffset * kOctave)) + inOffset;
}


// test instrument's melody flag
inline bool
CCLInstrument::HasMelody() const
{
	return not (mFlags & flags_NoMelody);
}


//	test instrument's chord flag
inline bool
CCLInstrument::HasChords() const
{
	return not (mFlags & flags_NoChords);
}


//	how many simultaneous chord-notes can this instrument produce?
inline short
CCLInstrument::GetPolyphony() const
{
	if ( HasChords() )
		return mPolyphony;
	return 0;
}


//	test if a note is playable (in gamut & not forbidden)
inline bool
CCLInstrument::IsNoteRestricted( short inNote ) const
{ 
	short off = NoteToOffset( inNote );
	return IsOffsetRestricted( mRestrictions, off );
}


//	test a note against the restriction bitmap
inline bool
CCLInstrument::IsOffsetRestricted( const SRestrictions&	inRestrictions, short inOffset )
{
	const int kBPL = bits_per_long;
	return (inRestrictions[inOffset / kBPL] >> (kBPL - 1 - (inOffset % kBPL))) & 1;
}


#ifdef CL_TUNE_HELPER_APP
//	disallow playing a certain note
void
CCLInstrument::SetNoteRestriction( short inNote, bool inRestrict )
{
	short off = NoteToOffset( inNote );
	SetOffsetRestriction( mRestrictions, off, inRestrict );
}


//	disallow (or allow) playing a certain pitch (in the restriction bitmap)
void
CCLInstrument::SetOffsetRestriction( SRestrictions& ioRestricts, short inOffset, bool inRestrict )
{
	if ( inOffset < 0 || inOffset > (kNumOctaves * kOctave) )
		return;
	ulong mask = (bits_per_long - 1 - (inOffset % bits_per_long));
	mask = 1UL << mask;
	if ( inRestrict )
		ioRestricts[inOffset / bits_per_long] |= mask;
	else
		ioRestricts[inOffset / bits_per_long] &= ~mask;
}
#endif	// CL_TUNE_HELPER_APP


#pragma mark -- CCLInstrumentList --

//
//	Constructor
//
CCLInstrumentList::CCLInstrumentList()
		:	mList( nullptr )
{
	Load( false );
}


//
//	Destructor
//
CCLInstrumentList::~CCLInstrumentList()
{
	if ( mList )
		CFRelease( mList );
}


/*
**	CCLInstrumentList::Load()
**
**	load in the instrument-list resource.
**	or maybe create an empty one, if none exists
*/
OSStatus
CCLInstrumentList::Load( bool inCreate )
{
	// dispose of the previous one, if any
	if ( mList )
		{
		CFRelease( mList );
		mList = nullptr;
		}
	
	OSStatus err = noErr;
	Handle list = ::GetResource( resType, resID );
	if ( list )
		{
		__Check( (GetHandleSize( list ) % sizeof(CCLInstrument)) == 0 );
		mList = CFDataCreate( kCFAllocatorDefault, (UInt8 *) *list, ::GetHandleSize( list ) );
		}
	else
		err = resNotFound;
	
#ifndef CL_TUNE_HELPER_APP
	// CL client never needs to create a new resource
# pragma unused( inCreate )
#else
	if ( not list && err == resNotFound && inCreate )
		{
		err = noErr;
		list = ::NewHandle( 0 );
		if ( not list )
			err = ::MemError();
		
		if ( list )
			{
			::AddResource( list, resType, resID, "\p" );
			err = ::ResError();
			mList = CFDataCreate( kCFAllocatorDefault, nullptr, 0 );
			}
		}
#endif  // CL_TUNE_HELPER_APP
	
	if ( list )
		::ReleaseResource( list );
	
	return err;
}


/*
**	CCLInstrumentList::Get()
**
**	retrieve a single instrument from the list
*/
short
CCLInstrumentList::Get( short inIndex, CCLInstrument& outInstrument ) const
{
	if ( not mList )
		return -1;

	if ( inIndex < 0 || inIndex >= Count() )
		return -1;
	
	CFRange wanted = CFRangeMake( inIndex * sizeof(CCLInstrument), sizeof(CCLInstrument) );
	CFDataGetBytes( mList, wanted, (UInt8 *) &outInstrument );
	
#if DTS_LITTLE_ENDIAN
	// the ToneDescription parts are left big-endian, because that's how QTMA
	// wants them stuffed into a NoteRequest event.
	outInstrument.mOctaveOffset    = ntohs( outInstrument.mOctaveOffset );
	outInstrument.mChordVelocity   = ntohs( outInstrument.mChordVelocity );
	outInstrument.mMelodyVelocity  = ntohs( outInstrument.mMelodyVelocity );
	outInstrument.mRestrictions[0] = ntohl( outInstrument.mRestrictions[0] );
	outInstrument.mRestrictions[1] = ntohl( outInstrument.mRestrictions[1] );
#endif
	
	return inIndex;
}


#ifdef CL_TUNE_HELPER_APP
/*
**	CCLInstrumentList::BuildMenu()
**
**	create a menu of all instruments
*/
short
CCLInstrumentList::BuildMenu( MenuRef inMenu, short inKeepItems )
{
	MenuRef menu = inMenu;
	if ( not menu || not mList )
		return 0;
	
	short remains = inKeepItems;
	
	while ( ::CountMenuItems( menu ) > remains )
		::DeleteMenuItem( menu, 1 );
	
	short count = Count();
	short index	= 0;
	const CCLInstrument * insts =
			reinterpret_cast<const CCLInstrument *>( CFDataGetBytePtr( mList ) );
	
	for ( int i = 0; i < count; ++i )
	{
//		::InsertMenuItem( menu, "\pUnavailable", index );
		InsertMenuItemTextWithCFString( menu, CFSTR("Unavailable"), index, 0, 0 );
		++index;
		if ( not remains )
			{
			if ( insts[i].mName[1] != '/' )
				::SetMenuItemText( menu, index, insts[i].mName );
			else
				::DisableMenuItem( menu, index );
			}
		else
			::SetMenuItemText( menu, index, insts[i].mName );
		}
	
	return index;
}


/*
**	CCLInstrumentList::Save()
**
**	update one instrument definition in the list,
**	and save the thus-modified instrument-list resource
*/
short
CCLInstrumentList::Save( short inIndex, CCLInstrument& inInstrument )
{
	if ( not inInstrument.mName[0] || not mList )
		return -1;
	
	// FIXME: mList is no longer a Handle
	OSErr err = noErr;
	short count = Count();
	if ( inIndex < 0 || inIndex >= count )
		{
		inIndex = count;
		++count;
		::SetHandleSize( mList, count * sizeof(CCLInstrument) );
		err = ::MemError();
		}
	if ( err )
		return err;
	
	::HLock( mList );	
	CCLInstrument * insts = reinterpret_cast<CCLInstrument *>( *mList );
	
	insts[inIndex] = inInstrument;
	insts[inIndex].mFlags &= ~CCLInstrument::flags_Custom;
	
	::HUnlock( mList );
	::ChangedResource( mList );
	::WriteResource( mList );
	
	return inIndex;
}


/*
**	CCLInstrumentList::Delete()
**
**	eliminate one instrument's definition, in RAM and in the resource
*/
short
CCLInstrumentList::Delete( short inIndex )
{
	if ( not mList )
		return -1;
	short count = Count();
	if ( inIndex < 0 || inIndex >= count )
		return -1;
	
	// FIXME: mList is no longer a Handle
	CCLInstrument * insts = reinterpret_cast<CCLInstrument *>( *mList );
	
	if ( count - inIndex )
		::BlockMoveData( &insts[ inIndex + 1 ], &insts[inIndex],
			(count - inIndex) * sizeof(CCLInstrument) );
	
	--count;
	::SetHandleSize( mList, count * sizeof(CCLInstrument) );
	
	::ChangedResource( mList );
	::WriteResource( mList );
	
	return noErr;
}
#endif	// CL_TUNE_HELPER_APP


#pragma mark -- CTuneBuilder --

//
//	Constructor (default)
//
CTuneBuilder::CTuneBuilder() :
		mMelodyVoice( kDefaultVoice ),
		mChordVoice( kDefaultVoice + 1 )
{
#ifdef CL_TUNE_HELPER_APP
	mNote2Text = nullptr;
#endif
	
	CCLInstrument inst;
	SetParameters( inst, kDefaultTempo, kDefaultVelocity );
}


//
//	Constructor
//
CTuneBuilder::CTuneBuilder(
	CCLInstrument &			inInstrument,
	short					inTempo,
	short					inVelocity,
	short					inStartVoice ) :
		mMelodyVoice( inStartVoice ),
		mChordVoice( mMelodyVoice + 1 )
{
	SetParameters( inInstrument, inTempo, inVelocity );
}


/*
**	CTuneBuilder::SetParameters()
**	assign instrument, tempo, loudness
*/
void
CTuneBuilder::SetParameters( CCLInstrument& inInstrument, short inTempo, short inVelocity )
{
	SetInstrument( inInstrument );
	SetTempo( inTempo );
	SetVelocity( inVelocity );
}


//
// Set the current tempo
//
void
CTuneBuilder::SetTempo( short inTempo )
{
	mTempo			= inTempo;
	mPauseLength	= (600 / (inTempo / 60.0F)) / 4;	// quarter-notes
	mNoteLength		= (long(mPauseLength) * ratio_NoteRest) / 100;
}


/*
**	set the instrument
*/
void
CTuneBuilder::SetInstrument( CCLInstrument& inInstrument )
{
	mInstrument		= inInstrument;
	mBaseNote		= kMiddleC + (mInstrument.mOctaveOffset * kOctave);
	mMinNote		= mBaseNote - kOctave;
	mMaxNote		= mBaseNote + ((kNumOctaves - 1) * kOctave);
}


//
//	set the velocity (loudness)
//
void
CTuneBuilder::SetVelocity( short inVelocity )
{
	mVelocity 		= inVelocity;
	mChordVelocity	= (mVelocity * mInstrument.mChordVelocity) / 100L;
	mMelodyVelocity	= (mVelocity * mInstrument.mMelodyVelocity) / 100L;
}


//
//	Handle an Octave related character
//
OSStatus
CTuneBuilder::GetOctave( char c )
{
	switch ( c )
		{
		case '-':					// +1 octave
			if ( mOctave > -1 )
				--mOctave;
			break;
		
		case '+':					// -1 octave
			if ( mOctave < 1 )
				++mOctave;
			break;
		
		case '=':					// central octave
			mOctave	= 0;
			break;
		
		case '/':					// highest oct.
			mOctave = 1;
			break;
		
		case '\\':					// lowest oct.
			mOctave = -1;
			break;
		
		default:
			return error_InvalidModifier;
		}
	
	return noErr;
}


//
//	Handle a note character
//
bool
CTuneBuilder::GetNote( char c )
{
	const short	midiNotes[] = { 69, 71, 60, 62, 64, 65, 67 };	// MIDI values for ABCDEFG
	mNote = -1;
	if ( c >= 'a' && c <= 'g' )
		{
		mNote		= midiNotes[c - 'a'] + (mOctave * kOctave);
		mDuration	= duration_Black;
		}
	else
	if ( c >= 'A' && c <= 'G' )
		{
		mNote 		= midiNotes[c - 'A'] + (mOctave * kOctave);
		mDuration	= duration_White;
		}
	if ( mNote != -1 )
		mNote -= (kMiddleC - mBaseNote);
	
	return mNote != -1;
}


//
//	Handle a note modifier
//
OSStatus
CTuneBuilder::GetNoteModifier( char c )
{
	switch ( c )
		{
		case '#':		// sharp
			if ( mNote + 1 > mMaxNote || (mNote + 1) >= (kMiddleC + (3 * kOctave)) )
				return error_ToneOverflow;
			++mNote;
			break;
		
		case '.':		// flat
			if ( mNote - 1 < mMinNote )
				return error_ToneOverflow;
			--mNote;
			break;
		
		case '_':		// tied
			mLinked	= true;
			break;
		
		default:		// must be a duration [1-9]
			if ( IsDuration( c ) )
				mDuration = GetDuration( c );
			else
				return error_InvalidModifier;
			break;
		}
	
	return noErr;
}


//
// Main entry point, the state machine
// Somewhat modified for Clan Lord (as opposed to CL Tune Helper)
//
OSStatus
CTuneBuilder::BuildTune(
	CDataHandle *			inSong,
	const char *			inMusic,
	long					inMusicLen,
	bool					inErrors )
{
#ifdef CL_TUNE_HELPER_APP
# pragma unused( inErrors )
	
# define set_flag(p, f)	(ioFlags ? \
		(ioFlags[ (p) - inMusic ] = (f) | (mComment || (f) == type_Comment ? \
			type_Comment : mBuildingChord ? type_InChord : 0)) : 0 )
	
	uchar * ioFlags = nullptr;
	CDataHandle * ioOffsets = nullptr;

#else
	// client doesn't do text annotations
# define set_flag(p, f)
#endif
	
	const char * current = inMusic;
	const char * last = inMusic + inMusicLen;
	short lastStuff	= 0;
	
	const ushort inFlags = flag_PauseStart | flag_PauseEnd;
	
	if ( inSong )
		inSong->Reset();
	
	// reset the flags if present
#ifdef CL_TUNE_HELPER_APP
	if ( ioFlags )
		{
		for ( int i = 0; i < inMusicLen; ++i )
			ioFlags[i] = 0;
		}
#endif
	
	mChordVolume		= 10;
	mMelodyVolume		= 10;
#ifdef CL_TUNE_HELPER_APP
	mNote2Text			= ioOffsets;
#endif
	mBeat				= 0;
	mOctave				= 0;
	mErrorPosition 		= 0;
	mError				= noErr;
	mTotalNotes			= 0;
	mTotalChords		= 0;
	mTotalDuration		= 0;
	mMaxChordPolyphony	= 0;
	mCurrentChordNum	= 0;
	mChordNum			= 0;
	for ( int i = 0; i < max_NotesInChord; ++i )
		mCurrentChord[i].duration = 0;
	
	mMarkNum			= 0;
	mComment			= 0;
	mNewTempo			= 0;
	mBuildingChord		= false;
	mSilent				= false;
	mStatus				= status_PickNote;		// First state
	
#ifdef CL_TUNE_HELPER_APP
	mNoteTextOffset		= -1;
	mNoteTextLen		= 0;
#endif
	
	//
	// Bug fix. first notes are eaten in some case. having a "fake"
	// event first (a rest) helps alleviate the problem.
	// the negative value means *hard coded* pause. not tempo related.
	// with duets/trios all songs *must* start at the same time.
	//
	if ( inFlags & flag_PauseStart )
		mError = StuffPause( inSong, mMelodyVoice, -300 );
	
	//
	//	While we have song to analyse, and the status hasn't returned to PickNote,
	//	and there is no error, we loop here.
	//	Note: 	We can loop SEVERAL time on a character; only the 'next'
	//			flag tells the machine to skip to the next one.
	
	//	FIXME: this parser sometimes looks at the character(s) beyond the song text limit.
	//	(for example, just after the transition from status_HasNote to status_PickModifier).
	//	If the spurious text happens to be, say, a comment starter, or some type of marker,
	//	then the parser can go completely berserk.
	//	I can't think of a good, simple, elegant fix for this problem. So, instead, we arrange
	//	to append a trailing null byte to the input text [see CTunePlayer::QueueTune()].
	//	That fixes the problem in the PickModifier case, as the null drives the state machine
	//	into status_StuffNote and then back to status_PickNote.
	//	But I'm not sure if it fixes the -general- problem, in other states.
	
	while ( ((*current && current < last) || mStatus != status_PickNote) && not mError )
		{
#if 0	// possible fix? (part 1 of 2)
		char c = 0;
		if ( current >= inMusic && current < last )
			c = * current;
#else
		char c 		= * current;
#endif
		bool next	= false;
		
		// Handle nested comments
		if ( IsComment( c, false ) )		// got end comment?
			{
			if ( mComment <= 0 )
				mError = error_UnmatchedComment;	// ... w/o corresponding start!
			else
				{
				--mComment;					// de-nest 1 comment level
				if ( mComment <= 0 )
					{
					// finished outermost comment
					next = true;
					set_flag( current, type_Comment );
					}
				}
			}
		
		if ( IsComment( c, true ) )
			++mComment;		//  got comment start, increase nesting level
		
		//
		// Skip comments, else proceed
		//
		if ( mComment > 0 )
			{
			next = true;
			set_flag( current, type_Comment );
			}
		else
		if ( IsSkip( c ) )		// ignorable whitespace
			next = true;
		else
		if ( not mError && not next )
			switch ( mStatus )
				{
				case status_PickNote:
					if ( IsNote( c ) )
						mStatus = status_HasNote;		// a note!
					else
					if ( not mBuildingChord && IsChord( c, true ) )	// Opening of a chord
						{
						next			= true;
						mBuildingChord	= true;
#ifdef CL_TUNE_HELPER_APP
						set_flag( current, type_InChord );
						mNoteTextOffset	= current - inMusic;
						mNoteTextLen	= 1;
#endif
						mChordNum		= 0;
						}
					else
					if ( mBuildingChord && IsChord( c, false ) )	// Closing of a chord
						{
						next			= true;
#ifdef CL_TUNE_HELPER_APP
						set_flag( current, type_InChord );
						mNoteTextLen	= current - inMusic - mNoteTextOffset + 1;
#endif
						mStatus			= status_HasChord;
						}
					else
					if ( not mBuildingChord && IsPause( c ) )		// A Pause
						{
						next			= true;
#ifdef CL_TUNE_HELPER_APP
						set_flag( current, type_Pause );
						if ( not mBuildingChord )
							{
							mNoteTextOffset	= current - inMusic;
							mNoteTextLen	= 1;
							}
#endif
						mStatus			= status_HasPause;
						}
					else
					if ( IsOctave( c ) )							// Octave command
						{
						next			= true;
						set_flag( current, type_OctaveMark );
						if ( not mSilent )					// Don't change octave if not 'playing'
							mError		= GetOctave( c );
						}
					else
					if ( not mBuildingChord && IsMark( c, true ) )	// Opening mark
						{
						if ( mMarkNum + 1 > max_Marks )
							mError		= error_TooManyMarks;
						else
							{
							mStatus		= status_HasMark;
							next		= true;
							set_flag( current, type_LoopMark );
							}
						}
					else
					if ( not mBuildingChord && IsMark( c, false ) )	// Closing mark
						{
						if ( mMarkNum <= 0 )
							mError		= error_UnmatchedMark;
						else
							{
							mStatus		= status_HasEndMark;
							next		= true;
							set_flag( current, type_LoopMark );
							}
						}
					else
					if ( IsTempoChange( c ) )						// tempo change
						{
						// don't set an error if we are silent. for multiple endings, tempo
						// changes will be parsed several times, including times where there
						// might be chords playing from another ending
						if ( (mBuildingChord || mCurrentChordNum) && not mSilent )
							mError 		= error_InvalidTempoChange;
						else
							{
							mNewTempo	= 0;
							mCommandChar= 0;
							mStatus		= status_PickTempo;
							next		= true;
							set_flag( current, type_Comment );
							}
						}
					else
					if ( IsVolume( c ) )
						{
						set_flag( current, type_OctaveMark );
						mCommandChar	= c;
						next			= true;
						mStatus			= status_PickVolume;
						}
					else
					if ( IsEnding( c ) )
						{
						if ( mBuildingChord )
							mError 		= error_EndingInChord;
						else
						if ( mMarkNum <= 0 )
							mError		= error_EndingOutsideLoop;
						else
							{
							mCommandChar	= c;
							if ( '!' == c )
								mStatus			= status_HasDefEnding;
							else
								{
								mStatus			= status_HasEnding;
								next			= true;
								}
							set_flag( current, type_LoopMark );
							}
						}
					else
					if ( IsSkip( c ) )								// ignore spaces etc
						next			= true;
					else
						mError			= error_InvalidNote;
					break;
				
				case status_HasDefEnding:
					{
					next			= true;
					mSilent			= true;
					// The first pass doesn't play. It is used to know where we are going.
					if ( 0 == mMark[mMarkNum - 1].count )
						{
						// sets the mark
						if ( mMark[mMarkNum - 1].endingDefault )
							mError		= error_DuplicateDefEnding;
						else
							mMark[mMarkNum - 1].endingDefault = current + 1;
						mStatus 		= status_PickNote;
						}
					else
						{
						// goto the end
						current			= mMark[mMarkNum - 1].end;
						next			= false;
						mStatus 		= status_HasEndMark;
						}
					break;
					}
				
				case status_HasEnding:
					{
					mSilent			= true;
					short index = 0;
					if ( IsDuration( c ) )
						{
						set_flag( current, type_Modifier );
						index	 		= GetDuration( c );
						next			= true;
						}
					else
						{
						mError			= error_InvalidEndingIndex;
						continue;
						}
					// The first pass doesn't play. It is used to know where we are going.
					if ( 0 == mMark[mMarkNum - 1].count )
						{
						// sets the mark
						if ( mMark[mMarkNum - 1].ending[index] )
							mError		= error_DuplicateEnding;
						else
							mMark[mMarkNum - 1].ending[index] = current + 1;// skip index value
						mStatus 		= status_PickNote;
						}
					else
						{
						// goto the end
						current			= mMark[mMarkNum - 1].end;
						next			= false;
						mStatus 		= status_HasEndMark;
						}
					break;
					}
				
				case status_PickVolume:
					{
					short duration = 0;
					if ( IsDuration( c ) )		// is it 1-9 ?
						{
						set_flag( current, type_Modifier );
						duration 		= GetDuration( c );		// 1-digit atoi()
						next			= true;
						}
					if ( mSilent )
						{
						mStatus			= status_PickNote;		// silent: let's just *ignore* it
						break;
						}
					short& vol = mBuildingChord ? mChordVolume : mMelodyVolume;
					switch ( mCommandChar )
						{
						case '%':
							vol = duration ? duration : 10;
							break;
						case '{':
							vol = duration ? vol - duration : vol - 1;
							break;
						case '}':
							vol = duration ? vol + duration : vol + 1;
							break;
						}
					
					if ( vol < 1 )
						vol = 1;
					if ( vol > 10 )
						vol = 10;
					mStatus 			= status_PickNote;
					break;
					}
				
				case status_PickTempo:
					if ( not mNewTempo && (c == '+' || c == '-' || c == '=') )
						{
						mCommandChar	= c;
						next			= true;
						set_flag( current, type_Modifier );
						}
					else
					if ( isdigit( c ) )
						{
						mNewTempo		= (mNewTempo * 10) + (c - '0');
						next			= true;
						set_flag( current, type_Modifier );
						if ( mNewTempo > kMaximumTempo )
							mError		= error_InvalidTempo;
						}
					else
						{
						mStatus			= status_HasTempo;
						if ( mSilent )
							{
							// we clear the errors, they will be picked when this
							// piece is played for real anyway. 
							mError		= 0;
							mStatus		= status_PickNote;				
							}
						}
					break;
				
				case status_HasTempo:					
					if ( not mNewTempo )
						{
						if ( mCommandChar )
							mError = error_ModifierNeedValue;
						else
							mNewTempo = kDefaultTempo;
						}
					switch ( mCommandChar )
						{
						case '+':
							mNewTempo = mTempo + mNewTempo;
							if ( mNewTempo > kMaximumTempo )
								mNewTempo = kMaximumTempo;
							break;
						
						case '-':
							mNewTempo = mTempo - mNewTempo;
							if ( mNewTempo < kMinimumTempo )
								mNewTempo = kMinimumTempo;
							break;
						
						default:
							break;
						}
					
					if ( mNewTempo < kMinimumTempo || mNewTempo > kMaximumTempo )
						mError 			= error_InvalidTempo;
					else
						SetTempo( mNewTempo );
					mStatus 			= status_PickNote;
					break;
				
				case status_HasMark:
					{
					SMark& mk = mMark[ mMarkNum ];
					mk.pos				= current;
					mk.count			= 0;
					mk.index			= 0;
					mk.endingCurrent	= 0;
					mk.endingDefault	= nullptr;
					mk.silentState		= mSilent;
					for ( int i = 0; i < max_Endings; ++i )
						mk.ending[i]	= nullptr;
					++mMarkNum;
					mStatus 			= status_PickNote;
					}
					break;
				
				case status_HasEndMark:	// (cde|1(abc)2fga|2(ge)2f!ab)3
											// if we were skipping endings
					mSilent				= mMark[mMarkNum - 1].silentState;	
					mStatus 			= status_PickNote;
					if ( 0 == mMark[mMarkNum - 1].count )
						{
						// on first pass, we are not stuffing. we are setting endings marks
						// and checking the number of loops. Once we are here, we reset to
						// loop beginning and start doing stuff properly
						mMark[mMarkNum - 1].count = 1;
						mMark[mMarkNum - 1].index = 1;
						mMark[mMarkNum - 1].end	= current;			// save current position
						if ( IsDuration( c ) )
							mMark[mMarkNum - 1].count = GetDuration( c );
						}
					if ( 0 == mMark[mMarkNum - 1].endingCurrent )
						{
						// jump to our fav ending ?
						const char * end = mMark[mMarkNum - 1].ending[ mMark[mMarkNum - 1].index ];
						if ( not end )
							end	 = mMark[mMarkNum - 1].endingDefault;
						mMark[mMarkNum - 1].endingCurrent = mMark[mMarkNum - 1].index;
						if ( end )
							{
							current		= end;
							next		= false;
							continue;
							}
						}
					mMark[mMarkNum - 1].endingCurrent = 0;
					if ( 1 == mMark[mMarkNum - 1].count )
						{
						// This is the last loop, we pop the mark and continue happily
						if ( IsDuration( c ) )
							{
							next		= true;						// to skip it
							set_flag( current, type_Modifier );
							}
						--mMarkNum;
						}
					else
						{
						mMark[mMarkNum - 1].count -= 1;
						mMark[mMarkNum - 1].index += 1;
						current 		= mMark[mMarkNum - 1].pos;
						next			= false;
						}
					break;
				
				case status_HasNote:
					if ( GetNote( c ) )
						{
						mStatus 		= status_PickModifiers;
						mLinked			= false;
						next 			= true;
						set_flag( current, type_Note );
#ifdef CL_TUNE_HELPER_APP
						if ( not mBuildingChord )
							{
							mNoteTextOffset	= current - inMusic;
							mNoteTextLen	= 1;
							}
#endif
						}
					else
						mError			= error_InvalidNote;
					break;
				
				case status_PickModifiers:
					if ( IsNoteModifier( c ) )
						mStatus 		= status_HasModifiers;
					else
						mStatus 		= status_StuffNote;
					break;
				
				case status_HasModifiers:
					mError				= GetNoteModifier( c );
					if ( not mError )
						{
						mStatus 		= status_PickModifiers;
						next 			= true;
#ifdef CL_TUNE_HELPER_APP
						set_flag( current, type_Modifier );
						++mNoteTextLen;
#endif
						}
					break;
				
				case status_HasChord:
					mChordDuration		= default_ChordDuration;
					if ( IsDuration( c ) )
						{
						mChordDuration	= GetDuration( c );
						next			= true;
#ifdef CL_TUNE_HELPER_APP
						set_flag( current, type_Modifier );
						++mNoteTextLen;
#endif
						}
					else
					if ( IsLongChord( c ) )
						{
						if ( mInstrument.mFlags & CCLInstrument::flags_LongChord )
							{
							mChordDuration	= 0;
							next			= true;
#ifdef CL_TUNE_HELPER_APP
							set_flag( current, type_Modifier );
							++mNoteTextLen;
#endif
							}
						else
							mError = error_UnsupportedInstrument;
						}
					mStatus 			= status_StuffChord;
					break;
				
				case status_HasPause:
					mPauseDuration		= duration_Black;
					if ( IsDuration( c ) )
						{
						mPauseDuration	= GetDuration( c );
						next			= true;
#ifdef CL_TUNE_HELPER_APP
						set_flag( current, type_Modifier );
						++mNoteTextLen;
#endif
						}
					mStatus 			= status_StuffPause;
					break;
				
				case status_StuffNote:
					lastStuff			= status_StuffNote;
					mStatus 			= status_PickNote;
					if ( mBuildingChord )
						{
						if ( mChordNum + 1 > mInstrument.GetPolyphony() )
							mError		= error_InvalidChord;
						mChord[ mChordNum++ ] = mNote;
						}
					else
						{
						if ( mSilent )
							break;
						mError = StuffNote( inSong, mMelodyVoice, mNote, mLinked, mDuration );
						}
					++mTotalNotes;
					break;
				
				case status_StuffChord:
					lastStuff			= status_StuffChord;
					mBuildingChord		= false;
					mStatus 			= status_PickNote;
					if ( mSilent )
						break;
					mError = StuffChord( inSong, mChordVoice, mChord, mChordNum,
								false, mChordDuration );
					++mTotalChords;
					break;
				
				case status_StuffPause:
					lastStuff			= status_StuffPause;
					mStatus 			= status_PickNote;
					if ( mSilent )
						break;
					mError				= StuffPause( inSong, mMelodyVoice, mPauseDuration );
					break;
				}
		
		if ( next && not mError )
			{
			++current;
#if 0		// possible fix (part 2 of 2)
			if ( current > last )
				current = last;		// don't drive completely off the cliff
#endif
			}
		}
	
	if ( not mError && mMarkNum )
		mError = error_unterminatedLoop;
	
	if ( not mError && mBuildingChord )
		mError = error_unterminatedChord;
	
	if ( not mError )
		LongChordFinish( inSong );
	
	// add a pause at the end, to have the last note ring.
	if ( not mError
	&&   (inFlags & flag_PauseEnd)
	&&   mTotalNotes > 0
	&&   lastStuff != status_StuffPause )
		{
		mError = StuffPause( inSong, mMelodyVoice, 8 );
		}
	
	if ( mError )
		{
		mErrorPosition = current - inMusic;
#if 0
		// this error positioning is not consistent;
		// will need to cleanify for this code to work
		if ( mErrorPosition && next )
			{
			--mErrorPosition;
			--current;
			}
#endif	// 0
		set_flag( current, type_Error );
		}
	else
		StuffEndSong( inSong );
	
#ifndef CL_TUNE_HELPER_APP
	if ( mError && inErrors )
		{
		SafeString msg;
# ifdef DEBUG_VERSION
		msg.Format( "* %s (%d).", sErrorMessages[ mError ], (int) mError );
# else
		msg.Format( "* %s.", sErrorMessages[ mError ] );
# endif
		ShowInfoText( msg.Get() );
		}
#endif // ! CL_TUNE_HELPER_APP
	
	return mError;
}


//
// Returns a QTMA low-level duration from a beat duration
//
short
CTuneBuilder::StuffGetNoteDuration( bool inLinked, short inDuration ) const
{
//	short duration = inLinked ? inDuration * mPauseLength : inDuration * mNoteLength;
	short duration;
	
	if ( inLinked  )
		duration = inDuration * mPauseLength; 	// linked note has no small rest
	else
	if ( inDuration > 1 )			// else it has one
		duration = ((inDuration - 1) * mPauseLength) + mNoteLength;
	else
		duration = mNoteLength;
	
	return duration;
}


//
//	utility for appending to the compiled song storage
//
inline
MusicOpWord *
CTuneBuilder::GrowSong( CDataHandle * inSong, int inOpsNeeded )
{
	void * w = inSong->Add( nullptr, inOpsNeeded * sizeof(MusicOpWord) );
	return static_cast<MusicOpWord *>( w );
}


//
// Stuff a QTMA note into our song.
// a Note is nothing, only the Rest duration fixes its length. (Well, sort of.)
//
OSStatus
CTuneBuilder::StuffNote(
	CDataHandle *			inSong,
	short					inVoice,
	short					inNote,
	bool					inLinked,
	short					inDuration )
{
	if ( not inSong )
		return noErr;
	
	bool silent = ( 0 == mMelodyVelocity )
			   || not mInstrument.HasMelody()
			   || mInstrument.IsNoteRestricted( inNote );
	
	// note event if not silent, plus rest event always
	MusicOpWord * w = GrowSong( inSong, silent ? 1 : 2 );
	if ( not w )
		return error_OutOfMemory;
	
	if ( not silent )
		{
		short duration = StuffGetNoteDuration( inLinked, inDuration );
		qtma_StuffNoteEvent( *w, inVoice, inNote,
								(mMelodyVelocity * (mMelodyVolume * 10)) / 100, duration );
		++w;
		
#ifdef CL_TUNE_HELPER_APP
		StuffNoteOffet( inNote, inVoice );
#endif
		}
	
	qtma_StuffRestEvent( *w, inDuration * mPauseLength );
	IncrementBeat( inDuration );
	
#ifdef CL_TUNE_HELPER_APP
	StuffNoteOffet( 0, inVoice );
#endif
	
	return noErr;
}


//
// Stuff a chord
//
OSStatus
CTuneBuilder::StuffChord(
	CDataHandle *			inSong,
	short					inVoice,
	const short *			inChord,
	short					inChordLen,
	bool					inLinked,
	short					inDuration )
{
	if ( not inSong )
		return noErr;
	if ( 0 == mChordVelocity || not mInstrument.HasChords() )
		return noErr;
	
	short duration = StuffGetNoteDuration( inLinked, inDuration );
	for ( int i = 0; i < inChordLen; ++i )
		{
		if ( not mInstrument.IsNoteRestricted( inChord[i] ) )
			{
			bool stuff 		= true;
			short roomIndex = -1;
			for ( int n = 0; n < mInstrument.GetPolyphony(); ++n )
				{
				if ( mCurrentChord[n].note == inChord[i] )
					{
					// this one was playing, stop it now ?
					roomIndex = n;
					if ( -1 == mCurrentChord[n].duration )
						{
						LongChordFinishNote( inSong, n );
						stuff = (inDuration != 0);	// re-play this one, with a duration maybe
						}
					else
					if ( mCurrentChord[n].duration > 0 )
						{
						mCurrentChord[n].duration = 0;
						// since we replace a note, we reset it properly
						if ( mCurrentChordNum )
							--mCurrentChordNum;
						}
					break;
					}
				}
			
			if ( not stuff )		// we just stopped a long note
				continue;
			if ( -1 == roomIndex && mCurrentChordNum >= mInstrument.GetPolyphony() )
				return error_PolyphonyOverflow;	// error, no room!!
			if ( -1 == roomIndex )
				{
				// this one is a new one, find a slot
				for ( int n = 0; n < mInstrument.GetPolyphony(); ++n )
					{
					if ( not mCurrentChord[n].duration )
						{
						roomIndex = n;
						break;
						}
					}
				}
			if ( -1 == roomIndex )				// error, no room!!
				return error_PolyphonyOverflow;
			
			MusicOpWord * w = GrowSong( inSong, inDuration ? 1 : 2 );
			if ( not w )
				return error_OutOfMemory;
			
			SChord& cc		= mCurrentChord[ roomIndex ];
			cc.note 		= inChord[i];
			cc.startBeat 	= mBeat;
			cc.velocity		= (mChordVelocity * (mChordVolume * 10)) / 100;
			cc.event 		= reinterpret_cast<Ptr>( w ) - inSong->GetPtr();	// keep it here
			
			if ( inDuration )
				{
				cc.duration 	= inDuration;
				
				qtma_StuffNoteEvent( *w, inVoice, inChord[i], cc.velocity, duration );
				}
			else
				{
				// long notes are stored as 'extended notes', meaning they take *two* long words.
				// we just put in a placeholder here; will be overwritten in LongChordFinishNote()
				cc.duration 	= -1;							// long note
				
				qtma_StuffXNoteEvent( w[0], w[1], inVoice, inChord[i], cc.velocity, duration );
#ifdef CL_TUNE_HELPER_APP
				StuffNoteOffet( i ? 0 : inChord[i], inVoice );	// need that first one
#endif
				}
			
#ifdef CL_TUNE_HELPER_APP
			StuffNoteOffet( i ? 0 : inChord[i], inVoice );
#endif
			
			// keep track of the maximum note played on that channel
			++mCurrentChordNum;
			if ( mCurrentChordNum > mMaxChordPolyphony )
				mMaxChordPolyphony = mCurrentChordNum;
			}
		}
	
	return noErr;
}


//
//	Stuff a pause
//
OSStatus
CTuneBuilder::StuffPause( CDataHandle * inSong, short inVoice, short inDuration )
{
#ifndef CL_TUNE_HELPER_APP
# pragma unused( inVoice )
#endif
	
	if ( not inSong )
		return noErr;
	
	MusicOpWord * w = GrowSong( inSong, 1 );
	if ( not w )
		return error_OutOfMemory;
	
	// negative durations mean "absolute"
	int duration;
	if ( inDuration < 0 )
		duration = -inDuration;
	else
		duration = inDuration * mPauseLength;
	
	qtma_StuffRestEvent( *w, duration );
	IncrementBeat( inDuration );
	
#ifdef CL_TUNE_HELPER_APP
	StuffNoteOffet( 255, inVoice );
#endif
	
	return noErr;
}


//
// Increment the beat number with inDuration.
// This is called to know what notes are still playing at any time
//
void
CTuneBuilder::IncrementBeat( short inDuration )
{
	if ( inDuration < 0 )
		{
		mTotalDuration -= inDuration;	// absolute
		return;
		}
	
	mBeat += inDuration;
	mTotalDuration += inDuration * mPauseLength;
	
	// age the chords' lengths
	for ( int i = 0; i < max_NotesInChord; ++i )
		{
		// skip chords with duration -1, which are 'long' notes (aka pedal points)
		if ( mCurrentChord[i].duration > 0 )
			{
			if ( mCurrentChord[i].duration - inDuration <= 0 )
				{
				// this one is done, terminate it
				mCurrentChord[i].duration = 0;
				if ( mCurrentChordNum )
					--mCurrentChordNum;
				}
			else
				mCurrentChord[i].duration -= inDuration;	// just age it
			}
		}
}


/*
**	CTuneBuilder::LongChordFinishNote()
**	finish a single note of a long chord
*/
void
CTuneBuilder::LongChordFinishNote( CDataHandle * inSong, short inIndex )
{
	// paranoia: be sure it really WAS a long chord
	if ( mCurrentChord[inIndex].duration != -1 )
		return;
	
	// this was a long note, and we now know its true duration.
	long duration = (mBeat - mCurrentChord[inIndex].startBeat) * mPauseLength;
	
	// update (overwrite) the placeholder
	MusicOpWord * w = (MusicOpWord *)( inSong->GetPtr() + mCurrentChord[inIndex].event );
	qtma_StuffXNoteEvent( w[0], w[1],
						mChordVoice,
						mCurrentChord[inIndex].note,
						mCurrentChord[inIndex].velocity,
						duration );
	mCurrentChord[inIndex].duration = 0;
	
	if ( mCurrentChordNum )
		--mCurrentChordNum;
}


/*
**	CTuneBuilder::LongChordFinish()
**	finish all the notes of a long chord
*/
void
CTuneBuilder::LongChordFinish( CDataHandle * inSong )
{
	for ( int i = 0; i < max_NotesInChord; ++i )
		if ( -1 == mCurrentChord[i].duration )
			LongChordFinishNote( inSong, i );
}


//
//	Stuff an EndSong mark. used by QTMA
//
OSStatus
CTuneBuilder::StuffEndSong( CDataHandle * inSong )
{
	if ( not inSong )
		return noErr;
	MusicOpWord * w = GrowSong( inSong, 1 );
	if ( not w )
		return error_OutOfMemory;
	
	*w = kEndMarkerValue;	// aka qtma_StuffMarkerEvent( w, kMarkerEventEnd, 0 );
	
	return noErr;
}


#ifdef CL_TUNE_HELPER_APP
/*
**	CTuneBuilder::StuffNoteOffet()
**	update the note-to-text mapping
*/
void
CTuneBuilder::StuffNoteOffet( short inNote, short inVoice )
{
	if ( not mNote2Text )
		return;
	
	SNote2Text * w = reinterpret_cast<SNote2Text *>( mNote2Text->Add( nullptr, sizeof(SNote2Text) ) );
	if ( not w )
		return;		// not very important if this fails
	
	w->mTextOffset	= inNote ? mNoteTextOffset : 0;
	w->mNote		= inNote;
	w->mLength		= inNote ? mNoteTextLen : 0;
	w->mVoice		= inVoice;
}
#endif  // CL_TUNE_HELPER_APP


//
//	For tune header, stuff a note request
//
OSStatus
CTuneBuilder::StuffNoteRequest( CDataHandle * inSong, short inVoice, const NoteRequest * inNote )
{
	if ( not inSong )
		return noErr;
	
	struct SStuffRequestData
	{
		MusicOpWord		w1;
		NoteRequest		re;
		MusicOpWord		w2;
	};
	SStuffRequestData * data = reinterpret_cast<SStuffRequestData *>(
				inSong->Add( nullptr, sizeof(SStuffRequestData) ) );
	if ( not data )
		return error_OutOfMemory;
	
	qtma_StuffGeneralEvent( data->w1, data->w2, inVoice,
				kGeneralEventNoteRequest, sizeof(SStuffRequestData) / sizeof(long) );
	data->re = *inNote;
	return noErr;
}


/*
**	create the tune header (note requests for both voices)
*/
OSStatus
CTuneBuilder::MakeTuneHeader( CDataHandle * inHeader )
{
	if ( not inHeader )
		return error_OutOfMemory;
	
	ComponentResult	error = 0;
	NoteRequest note;
	
	note.info.flags 		= 0;
	note.info.midiChannelAssignment	= 0;
#if TARGET_RT_LITTLE_ENDIAN
	note.info.typicalPolyphony.bigEndianValue = EndianS32_NtoB( 0x00010000 );
#else
	note.info.typicalPolyphony = EndianS32_NtoB( 0x00010000 );
#endif
	note.tone				= mInstrument.mInstrument;
	
	// since we haven't built the song yet, we don't know whether it actually uses any chords
	// so we'll prime the chord voice as long as the instrument could play them
//	if ( not error && mMaxChordPolyphony )
	if ( not error && mInstrument.HasChords() )
		{
#if TARGET_RT_LITTLE_ENDIAN
			// max_NotesInChord
		note.info.polyphony.bigEndianValue = EndianS16_NtoB( mMaxChordPolyphony );
#else
		note.info.polyphony	= EndianS16_NtoB( mMaxChordPolyphony );	// max_NotesInChord
#endif
		error = StuffNoteRequest( inHeader, mChordVoice, &note );
		}
	
	// and the melody voice
	if ( not error )
		{
#if TARGET_RT_LITTLE_ENDIAN
		note.info.polyphony.bigEndianValue	= EndianS16_NtoB( 1 );
#else
		note.info.polyphony	= EndianS16_NtoB( 1 );
#endif
		error = StuffNoteRequest( inHeader, mMelodyVoice, &note );
		}
	
	// the header is now complete
	if ( not error )
		error = StuffEndSong( inHeader );
	
	return error;
}


#ifdef CL_TUNE_HELPER_APP
//
//	Small and dirty SimplePlayTune, for CL Tune Helper
//
OSStatus
CTuneBuilder::SimplePlayTune(
	CDataHandle *			inTune,
	TunePlayCallBackUPP		inCallback /* = nullptr */,
	long					inRefCon /* = nullptr */ )
{
	CDataHandle tuneHeader;
	ComponentResult	error = MakeTuneHeader( &tuneHeader );
	
	TunePlayer player = nullptr;
	
	if ( not error )
		{
		player = ::OpenDefaultComponent( kTunePlayerComponentType, 0 );
		if ( not player )
			error = 1;
		}
	
	if ( inCallback )
		{
		if ( not error )
			error = ::TuneSetNoteChannels( player, 0, nullptr, inCallback, inRefCon );
		}
	else
		{
		if ( not error )
			error = ::TuneSetHeader( player, reinterpret_cast<ulong *>( tuneHeader.GetPtr() ) );
		}
	
	if ( not error )
		error = ::TuneQueue( player, reinterpret_cast<ulong *>( inTune->GetPtr() ),
								0x00010000, 0, 0x7FFFFFFF, 0, nullptr, 0 );
	
	if ( not error )
		{
		TuneStatus playerStatus;
		do {
			error = ::TuneGetStatus( player, &playerStatus );
			
			EventRecord	evt;
			::EventAvail( everyEvent, &evt );
			if ( inCallback )
				{
				InvokeTunePlayCallBackUPP( nullptr, 0, inRefCon, inCallback );
//				CallTunePlayCallBackProc( inCallback, nullptr, 0, inRefCon );
				}
			} while ( not error && playerStatus.queueTime && not ::Button() );
		}
	
	if ( player )
		::CloseComponent( player );
	
	return error;
}
#endif  // CL_TUNE_HELPER_APP


#pragma mark -- CTunePlayer --

//
//	Constructor
//
CTunePlayer::CTunePlayer( ulong inID, CDataHandle * inData,
						  CDataHandle * inSong, const CDataHandle * inHeader )
		:	mID( inID ),
			mTouched( 0 ),
			mInstrument( 0 ),
			mTempo( kDefaultTempo ),
			mVelocity( kDefaultVelocity ),
			mPlayer( nullptr ),
			mHeader( inHeader ),
			mTunesNum( 0 ),
			mTunesQueued( 0 ),
			mTuneBuffer( inData ),
			mTuneBufferLen( 0 ),
			mTune( inSong ),
			mPlayed( false )
{
}


//
//	Destructor
//
CTunePlayer::~CTunePlayer()
{
	if ( mPlayer )
		{
		__Verify_noErr( ::TuneUnroll( mPlayer ) );
		__Verify_noErr( ::CloseComponent( mPlayer ) );
		}
	delete mHeader;
	delete mTuneBuffer;
	delete mTune;
}


//
//	see if the player is currently making noise
//
bool
CTunePlayer::IsPlaying() const
{
	if ( not mPlayer || not mHeader || not mTunesQueued )
		return false;
	
	TuneStatus playerStatus;
	ComponentResult error = ::TuneGetStatus( mPlayer, &playerStatus );
	if ( error != noErr )
		return false;
	
	return playerStatus.queueTime != 0;
}


/*
**	CTunePlayer::Prepare()
**
**	get ready to play ASAP: load the component, set its header & volume, and preroll it
*/
OSStatus
CTunePlayer::Prepare()
{
	ComponentResult error = noErr;
	
#ifndef CL_TUNE_HELPER_APP
	// don't bother if they've got their sounds turned off
	if ( not gPrefsData.pdSound )
		return noErr;
#endif
	
	if ( not mTunesQueued )	// don't double-prepare
		{
		if ( noErr == error )
			{
			mPlayer = ::OpenDefaultComponent( kTunePlayerComponentType, 0 );
			if ( not mPlayer )
				error = 1;	// ?? fixme
			}
		
		if ( noErr == error )
			{
			error = ::TuneSetHeader( mPlayer, reinterpret_cast<ulong *>( mHeader->GetPtr() ) );
			__Check_noErr( error );
			}
			
#ifndef CL_TUNE_HELPER_APP
		// impose bard-song volume limit, from prefs
		if ( noErr == error )
			{
			// scale 0-100 to Fixed (16.16) format
			Fixed volume = (gPrefsData.pdBardVolume * 0x10000) / 100;
			error = ::TuneSetVolume( mPlayer, volume );
			__Check_noErr( error );
			}
#endif
		
		if ( noErr == error )
			{
			error = ::TunePreroll( mPlayer );
			__Check_noErr( error );
			}
		}
	return error;
}


/*
**	CTunePlayer::ChangeVolume()
**
**	adjust the volume of a currently-playing tune
*/
void
CTunePlayer::ChangeVolume( int pct )
{
	if ( IsPlaying() )
		{
		// scale 0-100 to Fixed (16.16) format
		Fixed volume = (pct * 0x10000) / 100;
		__Verify_noErr( TuneSetVolume( mPlayer, volume ) );
		}
}


//
//	start playing!
//
OSStatus
CTunePlayer::Play()
{
	ComponentResult error = 0;
	
#ifndef CL_TUNE_HELPER_APP
	if ( not gPrefsData.pdSound )
		{
		// if sound is off, just _pretend_ we've played it
		++mTunesQueued;
		mPlayed = true;
		return noErr;
		}
#endif
	
	if ( not mPlayer )
		error = Prepare();
	
	error = ::TuneQueue( mPlayer,
						reinterpret_cast<ulong *>( mTune->GetPtr() ),
						0x00010000,		// "normal" speed
						0,				// start pos
						0x7FFFFFFF,		// stop pos
						kNilOptions,	// queue options
						nullptr,		// callback UPP
						0 );			// refcon
	__Check_noErr( error );
	++mTunesQueued;
	
	if ( not error && mTunesQueued )
		mPlayed = true;
	
	return error;
}


//
//	store a part (might be one of several)
//
OSStatus
CTunePlayer::QueueTune( const char * inTune, long inLen, bool bLastPart )
{
	if ( max_Parts == mTunesNum )
		return -1;
	
	// put it into the tune buffer
	char * p = mTuneBuffer->Add( inTune, inLen );
	if ( not p )
		return -1;
	
	mTuneBufferLen += inLen;
	
	if ( bLastPart )
		{
		// add a final trailing null, to compensate for parser fragility.
		// do NOT increment the song length; this is "out-of-band" data.
		mTuneBuffer->Add( "", 1 );
		}
	
	// increment the part count; hit the timestamp
	++mTunesNum;
	Touch();
	
	return noErr;
}


/*
**	CTunePlayer::GetParameters()
**
**	retrieve current settings
*/
inline void
CTunePlayer::GetParameters( short& outInstrument, short& outTempo, short& outVelocity ) const
{
	outInstrument = mInstrument;
	outTempo = mTempo;
	outVelocity = mVelocity;
}


/*
**	CTunePlayer::SetParameters()
**
**	change current settings
*/
inline void
CTunePlayer::SetParameters( short inInstrument, short inTempo, short inVelocity )
{
	mInstrument = inInstrument;
	mTempo = inTempo;
	mVelocity = inVelocity;
}


//
//	check if any part is too old and stale
//
inline bool
CTunePlayer::Timeout() const
{
	return GetCurrentEventTime() - mTouched > part_Timeout;
}


//
//	check if a player is no longer relevant
//
inline bool
CTunePlayer::Purgeable() const
{
	// we can't toss it if it's currently playing.
	// we CAN toss it if it's been played, or has timed out.
	// otherwise, it must be waiting around for other parts to arrive, so we must keep it.
	return not IsPlaying() && ( Played() || Timeout() );
}


#pragma mark -- CTuneQueue --

//
//	the various commands, sent to all listeners by the instruments' Socks script
//
const CTuneQueue::STextCmd
CTuneQueue::sCommands[] =
{
	// some commands				have terse synonyms
	
	{ "/music",	cmd_Music },
	{ "/play",	cmd_Play },			{ "/P",		cmd_Play },
	{ "/stop",	cmd_Stop },			{ "/S",		cmd_Stop },
	{ "/who",	cmd_Who },			{ "/W",		cmd_Who },
	{ "/tempo",	cmd_Tempo },		{ "/T",		cmd_Tempo },
	{ "/inst",	cmd_Instrument },	{ "/I",		cmd_Instrument },
	{ "/vol",	cmd_Volume },		{ "/V",		cmd_Volume },
	{ "/notes",	cmd_Notes },		{ "/N",		cmd_Notes },
	{ "/part",	cmd_Part },			{ "/M",		cmd_Part },
	{ "/with",	cmd_With },			{ "/H",		cmd_With },
	{ "/me",	cmd_Me },			{ "/E",		cmd_Me },
	{ nullptr,	0 }
	
	// keep these sync'ed with scripts/verb_music_instrument.func (or vice-versa)
};


//
//	Constructor
//
CTuneQueue::CTuneQueue()
{
	for ( int i = 0; i < max_Players; ++i )
		mPlayers[i] = nullptr;
}


//
//	Destructor
//
CTuneQueue::~CTuneQueue()
{
	for ( int i = 0; i < max_Players; ++i )
		delete mPlayers[i];
}


//
// Try to allocate a slot for that player ID
// return successfulnessitudehoodshipicity
//
bool
CTuneQueue::GetPlayer( ulong inPlayerID, short& outPlayerIndex ) const
{
	outPlayerIndex = -1;
	
	if ( 0 == inPlayerID )
		outPlayerIndex = system_Queue;
	else
		{
		for ( int i = 1; i < max_Players && outPlayerIndex == -1; ++i )		// existing player?
			if ( mPlayers[i] && mPlayers[i]->GetID() == inPlayerID )
				outPlayerIndex = i;
		for ( int i = 1; i < max_Players && outPlayerIndex == -1; ++i )		// free slot, then?
			if ( not mPlayers[i] )
				outPlayerIndex = i;
		}
	
	return outPlayerIndex != -1;
}


//
// Lookup a command (starting at *p) in our static table
// and return the corresponding command number.
// Increment the pointer as well, while we're at it
//
short
CTuneQueue::LookupCommand( const char *& p ) const
{
	short cmd = 0;
	for ( int i = 0; sCommands[i].tcmd; ++i )
		{
		size_t l = strlen( sCommands[i].tcmd );
		if ( 0 == strncmp( p, sCommands[i].tcmd, l ) )
			{
			p += l;
			cmd = sCommands[i].cmd;
			break;
			}
		}
	return cmd;
}


/*
**	SimpleAToI()
**
**	dead-simple string-to-integer converter: base 10, no error checking
*/
static inline int
SimpleAToI( const char *& p )
{
	int result = 0;
	while ( isdigit( *p ) )
		result = result * 10 + (*p++ - '0');
	return result;
}


#if defined( DEBUG_VERSION ) && TUNE_LOG_ENABLE
/*
**	LogTune()
**
**	for debugging (and nethack solution finding :)
*/
static void
LogTune( const char * inTune )
{
	// mode for fopen()
#if TUNE_LOG_CUMULATIVE
# define TUNE_LOG_MODE	"a"
#else
# define TUNE_LOG_MODE	"w"
#endif
	
	if ( FILE * stream = fopen( TUNE_LOG_PATH, "" ) )
		{
#if 1 && TUNE_LOG_CUMULATIVE	// maybe prepend a timestamp (pointless for non-cumulative)
		DTSDate date;
		date.Get();
		int hour = date.dateHour;
		char ampm = ( hour >= 12 ) ? 'p' : 'a';
		hour = ( hour + 11 ) % 12 + 1;
		fprintf( stream, "%d/%d/%.2d %d:%.2d:%.2d%c ",
			date.dateMonth,
			date.dateDay,
			date.dateYear % 100,
			hour,
			date.dateMinute,
			date.dateSecond,
			ampm );
#endif  // timestamping
	
		fprintf( stream, "%s\n", inTune );
		fclose( stream );
		}
}
#endif  // TUNE_LOG_ENABLE


/*
**	CTuneQueue::QueueTune()
**
**	handle a "/play" command, and its related adjuncts
*/
OSStatus
CTuneQueue::QueueTune( ulong inPlayerID, const char * inTune, long inLen, bool inErrors )
{
	if ( not inTune || inLen <= 0 )
		return paramErr;
	
#ifndef CL_TUNE_HELPER_APP
	// bail fast if sound is turned off (unless song syntax-checking is desired)
	if ( not gPrefsData.pdSound && not inErrors )
		return noErr;
#endif
	
	OSStatus err = noErr;
	
	short	tuneInstrument	= 0;				// default instrument? nah.
	short	tuneTempo		= kDefaultTempo;
	short	tuneVelocity	= kDefaultVelocity;	// in percent
	bool	tunePart		= false;
	short	tuneWithNum		= 0;
	ulong	tuneWith[ CTuneSync::max_Sync ];	// other performers' IDs
	const char * p			= inTune;
	
	// Extract variable parameters
	bool done = false;
	short cmd;
	do {
		cmd = LookupCommand( p );
		switch ( cmd )
			{
			case cmd_Tempo:
				tuneTempo = SimpleAToI( p );
				if ( tuneTempo < kMinimumTempo || tuneTempo > kMaximumTempo )
					tuneTempo = kDefaultTempo;
				break;
			
			case cmd_Instrument:
				tuneInstrument = SimpleAToI( p );
				if ( tuneInstrument < 0 || tuneInstrument >= mInstruments.Count() )
					tuneInstrument = 0;
				break;
			
			case cmd_Volume:
				tuneVelocity = SimpleAToI( p );
				if ( tuneVelocity < 0 || tuneVelocity > 100 )
					tuneVelocity = 100;
				break;
			
			case cmd_With:
				tuneWith[ tuneWithNum ] = SimpleAToI( p );
				if ( tuneWithNum < CTuneSync::max_Sync )
					++tuneWithNum;
				break;
			
			case cmd_Part:
				tunePart = true;
				break;
			
//			case cmd_Notes:		// same as default case
//				done = true;
//				break;
			
			default:
				done = true;
				break;
			}
		} while ( not done );
	
	if ( cmd != cmd_Notes )
		return paramErr;
	
	short playerIndex = -1;
	if ( not GetPlayer( inPlayerID, playerIndex ) )		// get a player slot
		return -1;
	
	if ( CTunePlayer * tp = mPlayers[ playerIndex ] )	// Compare with existing playing stuff
		{
		short tInstrument	= 0;						// default instrument?
		short tTempo		= kDefaultTempo;
		short tVelocity		= kDefaultVelocity;			// in percent
		tp->GetParameters( tInstrument, tTempo, tVelocity );
		
		if ( tInstrument != tuneInstrument /* || tTempo != tuneTempo */ )
			{
			// we COULD ignore that tune, but stopping the previous one
			// will alert the players that something is wrong
			delete tp;
			mPlayers[ playerIndex ] = nullptr;
			}
		if ( kDefaultTempo == tuneTempo )				// restore old tempo if new is default
			tuneTempo = tTempo;
		}
	
	CCLInstrument inst;
	if ( mInstruments.Get( tuneInstrument, inst ) < 0 )
		return paramErr;
	
	mBuilder.SetParameters( inst, tuneTempo, (kDefaultVelocity * tuneVelocity) / 100 );
	
	long tuneLen = inLen - (p - inTune);
	CDataHandle	* tuneHeader = nullptr;
	
#if TUNE_LOG_ENABLE && defined( DEBUG_VERSION )
	if ( not err )
		LogTune( inTune );
#endif
	
	if ( not err )
		{
		CTunePlayer * tp = mPlayers[ playerIndex ];
		if ( tp )
			{
			if ( tp->IsPlaying() )
				{
				// the song won't play. should we queue it? set an error?
				if ( inErrors )
					{
					err = CTuneBuilder::error_AlreadyPlaying;
					SafeString msg;
					msg.Format( "* %s.", CTuneBuilder::sErrorMessages[ err ] );
					ShowInfoText( msg.Get() );
					}
				}
			else
				err = tp->QueueTune( p, tuneLen, not tunePart );
			}
		else
			{
			CDataHandle	* tuneData = nullptr;
			CDataHandle * songData = nullptr;
			tuneHeader = NEW_TAG("CTuneQueue::tuneHeader") CDataHandle;
			if ( not tuneHeader )
				err = memFullErr;
			if ( not err )
				err = mBuilder.MakeTuneHeader( tuneHeader );
			if ( not err )
				{
				tuneData = NEW_TAG("CTuneQueue::tuneData") CDataHandle;
				if ( not tuneData )
					err = memFullErr;
				}
			if ( not err )
				{
				songData = NEW_TAG("CTuneQueue::songData") CDataHandle;
				if ( not songData )
					err = memFullErr;
				}
			if ( not err )
				{
				mPlayers[ playerIndex ] = tp = NEW_TAG("CTunePlayer") CTunePlayer( inPlayerID,
															tuneData, songData, tuneHeader );
				if ( not tp )
					err = memFullErr;
				}
			if ( not err )
				err = tp->QueueTune( p, tuneLen, not tunePart );
			if ( not err )
				{
				// Fill up the synchronisation table
				mSync.Want( inPlayerID );					// we want ourselves...
				for ( int i = 0; i < tuneWithNum; ++i )		// plus everyone we're playing with
					mSync.Want( tuneWith[i] );
				
				tp->SetParameters( tuneInstrument, tuneTempo, tuneVelocity );
				}
			}
		tuneHeader = nullptr;	// the CTunePlayer now owns this, so don't delete it
		}
	
	// maybe mark this part as done
	if ( not tunePart )
		mSync.Got( inPlayerID );
	
	// wow, play it NOW, maybe ?
	if ( not err && not tunePart && ( system_Queue == playerIndex || mSync.Ready() ) )
		{
		if ( system_Queue == playerIndex )
			{
			// compile the anonymous system player's song
			CTunePlayer * sysPlayer = mPlayers[ system_Queue ];
			err = mBuilder.BuildTune( sysPlayer->GetCompiledTune(), 
									  sysPlayer->GetBufferPtr(), 
									  sysPlayer->GetBufferLen(), 
									  inErrors );
										 
			if ( not err )
				err = sysPlayer->Play();
			}
		else
			{
			// compile the players' songs
			for ( int i = 1; i < max_Players && not err; ++i )
				{
				CTunePlayer * tp = mPlayers[i];
				if ( tp && mSync.Have( tp->GetID() ) )
					{
					// retrieve instrument specs (polyphony etc.)
					short buildInstID;
					short buildTempo;
					short buildVelocity;
					tp->GetParameters( buildInstID, buildTempo, buildVelocity );
					CCLInstrument buildInst;
					if ( mInstruments.Get( buildInstID, buildInst ) < 0 )
						return paramErr;
					mBuilder.SetParameters( buildInst, buildTempo,
								(kDefaultVelocity * buildVelocity) / 100 );
					
					// build tune
					err	= mBuilder.BuildTune( tp->GetCompiledTune(), 
											  tp->GetBufferPtr(), 
											  tp->GetBufferLen(), 
											  inErrors );
					} // have this player
				} // loop over players
			
			// play them
			if ( not err )
				{
				// Prepare() loads and prerolls the components;
				// it should help with the timing problems on duets/trios
				for ( int i = 1; i < max_Players && not err; ++i )
					{
					if ( mPlayers[i] && mSync.Have( mPlayers[i]->GetID() ) )
						err = mPlayers[i]->Prepare();
					}
				for ( int i = 1; i < max_Players && not err; ++i )
					{
					if ( mPlayers[i] && mSync.Have( mPlayers[i]->GetID() ) )
						err = mPlayers[i]->Play();
					}
				mSync.Reset();
				} // no compilation error
			} // not system queue
		} // ready to play
	
	// Cleanup if problem
	if ( err )
		{
		delete mPlayers[ playerIndex ];
		mPlayers[ playerIndex ] = nullptr;
		}
	delete tuneHeader;
	
	return err;
}


/*
**	CTuneQueue::HandleCommand()
**
**	interpret the various commands sent by bard instruments
*/
bool
CTuneQueue::HandleCommand( const char * inCommand, long inLen )
{
	bool handled = false;
	
	Idle();			// Maybe delete an old one?
	
	if ( inLen <= 0 )
		return handled;
	
	if ( '\r' == *inCommand )
		++inCommand;
	const char * p = inCommand;
	
	short cmd = LookupCommand( p );
	if ( cmd != cmd_Music )
		return false;
	
	bool done = false;
	ulong who = 0;
	bool me = false;
	do {
		cmd = LookupCommand( p );
		switch ( cmd )
			{
			case cmd_Who:
				who = SimpleAToI( p );
				break;
			
			case cmd_Me:
				me = true;
				break;
			
			case cmd_Play:
				handled = true;
				done	= true;
				QueueTune( who, p, inLen - (p - inCommand), me );
				break;
			
			case cmd_Stop:
				handled = true;
				//done	= true;
				StopPlaying( who );
				break;
			
			default:
				done = true;
				break;
			}
		} while ( not done && '/' == *p );
	
	return 	handled;
}


//
//	discard any stale tuneplayers
//
void
CTuneQueue::Idle()
{
	for ( int i = 0; i < max_Players; ++i )
		{
		if ( mPlayers[i] && mPlayers[i]->Purgeable() )
			{
			delete mPlayers[i];
			mPlayers[i] = nullptr;
			mSync.Reset();
			}
		}
}


/*
**	CTuneQueue::ChangeVolume()
**
**	adjust master volume (0-100)
*/
void
CTuneQueue::ChangeVolume( int volume )
{
	// change any currently-playing songs
	for ( int i = 0; i < max_Players; ++i )
		{
		if ( mPlayers[ i ] )
			mPlayers[ i ]->ChangeVolume( volume );
		}
	
	// and remember this for posterity
	gPrefsData.pdBardVolume = volume;
}


//
//	check if a given player ID is playing
//	(ID 0 means "any")
//
bool
CTuneQueue::IsPlaying( ulong inPlayerID ) const
{
	for ( int i = 0; i < max_Players; ++i )
		{
		if ( mPlayers[i] )
			{
			if ( 0 == inPlayerID || ( mPlayers[i]->GetID() == inPlayerID ) )
				if ( mPlayers[i]->IsPlaying() )
					return true;
			}
		}
	return false;
}


//
//	stop a specified player
//	(ID 0 means "stop all of them")
//
void
CTuneQueue::StopPlaying( ulong inPlayerID )
{
	for ( int i = 0; i < max_Players; ++i )
		{
		if ( mPlayers[i] ) 
			{
			if ( 0 == inPlayerID || (mPlayers[i]->GetID() == inPlayerID) )
				{
				delete mPlayers[i];
				mPlayers[i] = nullptr;
				}
			}
		}
	mSync.Reset();
}


#pragma mark -- CTuneSync --

/*
**	CTuneSync::Want()
**	register an unsatisfied part request
**	returns true for success
*/
bool
CTuneSync::Want( ulong inID )
{
	if ( 0 == inID )
		return false;							// system channel
	
	for ( int i = 0; i < max_Sync; ++i )
		if ( (mWant & (1U << i)) && mIDs[i] == inID )
			return true;						// already there
	
	if ( mWant == ((1U << max_Sync) - 1) )
		return false;							// table full
	
	for ( int i = 0; i < max_Sync; ++i )
		if ( not (mWant & (1U << i)) )			// slot free
			{
			mWant |= (1U << i);
			mIDs[i] = inID;
			return true;
			}
	
	return false;								// no more slots
}


/*
**	CTuneSync::Got()
**	a given part has been received
*/
void
CTuneSync::Got( ulong inID )
{
	if ( 0 == inID )
		return;									// system channel
	
	for ( int i = 0; i < max_Sync; ++i )
		if ( (mWant & (1U << i)) && mIDs[i] == inID )
			{
			mGot |= (1U << i);
			break;								// already there
			}
}


/*
**	CTuneSync::Have()
**	test if a part has been received
*/
bool
CTuneSync::Have( ulong inID ) const
{
	if ( 0 == inID )
		return false;							// system channel
	
	for ( int i = 0; i < max_Sync; ++i )
		if ( (mGot & (1U << i)) && mIDs[i] == inID )
			return true;						// there
	
	return false;
}

